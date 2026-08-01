package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type WorkerConfig struct {
	HTTPAddr                   string
	DatabaseURL                string
	FiscalURL                  string
	AccountingURL              string
	Environment                string
	AllowInsecureLocalServices bool
	RunOnce                    bool
	DispatchInterval           time.Duration
	MetricsInterval            time.Duration
	LeaseDuration              time.Duration
	ShutdownTimeout            time.Duration
	Calendars                  Calendars
}

type WorkerConfigError struct {
	Code string
	Err  error
}

func (e *WorkerConfigError) Error() string {
	return e.Err.Error()
}

func (e *WorkerConfigError) Unwrap() error {
	return e.Err
}

func LoadWorker() (WorkerConfig, error) {
	return LoadWorkerFrom(os.Getenv)
}

func LoadWorkerFrom(getenv func(string) string) (WorkerConfig, error) {
	if getenv == nil {
		return WorkerConfig{}, workerConfigError(
			"WORKER_CONFIG_INVALID",
			"environment reader is required",
		)
	}
	environment := strings.ToLower(
		defaultValue(getenv("PYMES_ENVIRONMENT"), "development"),
	)
	if environment != "development" &&
		environment != "test" &&
		environment != "production" {
		return WorkerConfig{}, workerConfigError(
			"WORKLOAD_IDENTITY_INVALID",
			"PYMES_ENVIRONMENT must be development, test, or production",
		)
	}
	allowInsecure := strings.EqualFold(
		strings.TrimSpace(getenv("PYMES_ALLOW_INSECURE_LOCAL_SERVICES")),
		"true",
	)
	if allowInsecure && environment == "production" {
		return WorkerConfig{}, workerConfigError(
			"WORKLOAD_IDENTITY_INVALID",
			"insecure local services are forbidden in production",
		)
	}
	databaseURL := strings.TrimSpace(getenv("PYMES_DATABASE_URL"))
	if databaseURL == "" {
		return WorkerConfig{}, workerConfigError(
			"DATABASE_URL_MISSING",
			"PYMES_DATABASE_URL is required",
		)
	}
	fiscalURL := strings.TrimSpace(getenv("FISCAL_ADAPTER_URL"))
	accountingURL := strings.TrimSpace(getenv("ACCOUNTING_URL"))
	if fiscalURL == "" || accountingURL == "" {
		return WorkerConfig{}, workerConfigError(
			"DEPENDENCY_URL_MISSING",
			"FISCAL_ADAPTER_URL and ACCOUNTING_URL are required",
		)
	}
	calendars, err := loadCalendars(getenv, environment)
	if err != nil {
		return WorkerConfig{}, &WorkerConfigError{
			Code: "CALENDAR_CONFIG_INVALID",
			Err:  err,
		}
	}
	metricsInterval, err := parseWorkerMetricsInterval(
		strings.TrimSpace(getenv("PYMES_WORKER_METRICS_INTERVAL")),
	)
	if err != nil {
		return WorkerConfig{}, &WorkerConfigError{
			Code: "METRICS_INTERVAL_INVALID",
			Err:  err,
		}
	}
	runOnce, err := parseWorkerBoolean(
		"PYMES_WORKER_RUN_ONCE",
		strings.TrimSpace(getenv("PYMES_WORKER_RUN_ONCE")),
	)
	if err != nil {
		return WorkerConfig{}, &WorkerConfigError{
			Code: "WORKER_RUN_ONCE_INVALID",
			Err:  err,
		}
	}
	dispatchInterval := time.Second
	if strings.TrimSpace(getenv("PYMES_WORKER_INTERVAL_MS")) != "" {
		dispatchInterval = 250 * time.Millisecond
	}
	return WorkerConfig{
		HTTPAddr: defaultValue(
			getenv("PYMES_WORKER_HTTP_ADDR"),
			":8080",
		),
		DatabaseURL:                databaseURL,
		FiscalURL:                  fiscalURL,
		AccountingURL:              accountingURL,
		Environment:                environment,
		AllowInsecureLocalServices: allowInsecure,
		RunOnce:                    runOnce,
		DispatchInterval:           dispatchInterval,
		MetricsInterval:            metricsInterval,
		LeaseDuration:              30 * time.Second,
		ShutdownTimeout:            5 * time.Second,
		Calendars:                  calendars,
	}, nil
}

func WorkerErrorCode(err error) string {
	var configErr *WorkerConfigError
	if errors.As(err, &configErr) && configErr.Code != "" {
		return configErr.Code
	}
	return "WORKER_CONFIG_INVALID"
}

func parseWorkerBoolean(name, value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return parsed, nil
}

func parseWorkerMetricsInterval(value string) (time.Duration, error) {
	if value == "" {
		return time.Minute, nil
	}
	interval, err := time.ParseDuration(value)
	if err != nil || interval < 5*time.Second {
		return 0, fmt.Errorf(
			"metrics interval must be a duration of at least 5s",
		)
	}
	return interval, nil
}

func workerConfigError(code, message string) error {
	return &WorkerConfigError{Code: code, Err: errors.New(message)}
}
