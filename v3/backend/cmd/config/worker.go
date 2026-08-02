package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	localWorkerReleaseSHA = "0000000000000000000000000000000000000000"
	localWorkerRevision   = "local"
)

var (
	workerReleaseSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)
	workerRevision   = regexp.MustCompile(`^[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
)

type WorkerConfig struct {
	HTTPAddr                    string
	DatabaseURL                 string
	FiscalURL                   string
	AccountingURL               string
	Environment                 string
	ReleaseSHA                  string
	Revision                    string
	SchedulingActionTokenSecret string
	AllowInsecureLocalServices  bool
	RunOnce                     bool
	DispatchInterval            time.Duration
	MetricsInterval             time.Duration
	LeaseDuration               time.Duration
	ShutdownTimeout             time.Duration
	PerGo                       PerGoWorker
	Calendars                   Calendars
}

type PerGoWorker struct {
	Enabled                  bool
	BaseURL                  string
	APIKey                   string
	Audience                 string
	WorkspaceID              string
	Channel                  string
	AllowGlobalRouteFallback bool
	Timeout                  time.Duration
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
	releaseSHA, revision, err := loadWorkerReleaseMetadata(getenv, environment)
	if err != nil {
		return WorkerConfig{}, err
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
	actionTokenSecret := strings.TrimSpace(
		getenv("PYMES_SCHEDULING_ACTION_TOKEN_SECRET"),
	)
	if len(actionTokenSecret) < 32 {
		return WorkerConfig{}, workerConfigError(
			"ACTION_TOKEN_SECRET_INVALID",
			"PYMES_SCHEDULING_ACTION_TOKEN_SECRET must contain at least 32 bytes",
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
	pergoEnabled, err := parseWorkerBoolean(
		"PYMES_PERGO_ENABLED",
		strings.TrimSpace(getenv("PYMES_PERGO_ENABLED")),
	)
	if err != nil {
		return WorkerConfig{}, &WorkerConfigError{
			Code: "PERGO_CONFIG_INVALID",
			Err:  err,
		}
	}
	allowGlobalRouteFallback, err := parseWorkerBoolean(
		"PERGO_ALLOW_GLOBAL_ROUTE_FALLBACK",
		strings.TrimSpace(getenv("PERGO_ALLOW_GLOBAL_ROUTE_FALLBACK")),
	)
	if err != nil {
		return WorkerConfig{}, &WorkerConfigError{
			Code: "PERGO_CONFIG_INVALID",
			Err:  err,
		}
	}
	pergoTimeout := 5 * time.Second
	if value := strings.TrimSpace(getenv("PERGO_TIMEOUT")); value != "" {
		pergoTimeout, err = time.ParseDuration(value)
		if err != nil || pergoTimeout < 100*time.Millisecond ||
			pergoTimeout > 30*time.Second {
			return WorkerConfig{}, workerConfigError(
				"PERGO_CONFIG_INVALID",
				"PERGO_TIMEOUT must be between 100ms and 30s",
			)
		}
	}
	pergo := PerGoWorker{
		Enabled:                  pergoEnabled,
		BaseURL:                  strings.TrimRight(strings.TrimSpace(getenv("PERGO_URL")), "/"),
		APIKey:                   strings.TrimSpace(getenv("PERGO_API_KEY")),
		Audience:                 strings.TrimSpace(getenv("PYMES_PERGO_AUDIENCE")),
		WorkspaceID:              strings.TrimSpace(getenv("PERGO_WORKSPACE_ID")),
		Channel:                  defaultValue(getenv("PERGO_CHANNEL"), "whatsapp"),
		AllowGlobalRouteFallback: allowGlobalRouteFallback,
		Timeout:                  pergoTimeout,
	}
	if pergo.Enabled &&
		(pergo.BaseURL == "" || pergo.APIKey == "" ||
			pergo.WorkspaceID == "" ||
			(pergo.Channel != "whatsapp" &&
				pergo.Channel != "whatsapp_cloud" &&
				pergo.Channel != "whatsapp_mock")) {
		return WorkerConfig{}, workerConfigError(
			"PERGO_CONFIG_INVALID",
			"complete PerGo worker configuration is required",
		)
	}
	if pergo.Enabled && environment == "production" &&
		pergo.Audience == "" {
		return WorkerConfig{}, workerConfigError(
			"PERGO_CONFIG_INVALID",
			"PYMES_PERGO_AUDIENCE is required in production",
		)
	}
	if pergo.Audience != "" && !validPerGoAudience(pergo.Audience) {
		return WorkerConfig{}, workerConfigError(
			"PERGO_CONFIG_INVALID",
			"PYMES_PERGO_AUDIENCE must be an exact HTTPS origin without path",
		)
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
		DatabaseURL:                 databaseURL,
		FiscalURL:                   fiscalURL,
		AccountingURL:               accountingURL,
		Environment:                 environment,
		ReleaseSHA:                  releaseSHA,
		Revision:                    revision,
		SchedulingActionTokenSecret: actionTokenSecret,
		AllowInsecureLocalServices:  allowInsecure,
		RunOnce:                     runOnce,
		DispatchInterval:            dispatchInterval,
		MetricsInterval:             metricsInterval,
		LeaseDuration:               30 * time.Second,
		ShutdownTimeout:             5 * time.Second,
		PerGo:                       pergo,
		Calendars:                   calendars,
	}, nil
}

func validPerGoAudience(value string) bool {
	audience, err := url.Parse(value)
	return err == nil &&
		audience.Scheme == "https" &&
		audience.Host != "" &&
		audience.Opaque == "" &&
		audience.User == nil &&
		audience.Path == "" &&
		audience.RawPath == "" &&
		audience.RawQuery == "" &&
		!audience.ForceQuery &&
		audience.Fragment == "" &&
		!strings.ContainsAny(audience.Host, " \t\r\n,|")
}

func loadWorkerReleaseMetadata(
	getenv func(string) string,
	environment string,
) (string, string, error) {
	releaseSHA := strings.TrimSpace(getenv("PYMES_RELEASE_SHA"))
	revision := strings.TrimSpace(getenv("K_REVISION"))
	if environment != "production" {
		if releaseSHA == "" {
			releaseSHA = localWorkerReleaseSHA
		}
		if revision == "" {
			revision = localWorkerRevision
		}
	}
	if !workerReleaseSHA.MatchString(releaseSHA) {
		return "", "", workerConfigError(
			"WORKER_RELEASE_METADATA_INVALID",
			"PYMES_RELEASE_SHA must contain exactly 40 lowercase hexadecimal characters",
		)
	}
	if !workerRevision.MatchString(revision) {
		return "", "", workerConfigError(
			"WORKER_RELEASE_METADATA_INVALID",
			"K_REVISION must contain a valid Cloud Run revision name",
		)
	}
	return releaseSHA, revision, nil
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
