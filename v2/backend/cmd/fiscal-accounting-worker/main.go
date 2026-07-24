package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	observability "github.com/devpablocristo/platform/observability/go"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscalaccounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscalaccounting/workerhost"
)

const (
	serviceName                  = "pymes-v2-fiscal-accounting-worker"
	defaultOrganizationBatchSize = 100
	maxOrganizationBatchSize     = 1000
	maxAttempts                  = 1000
)

type environment struct {
	DatabaseURL       string
	WorkerConfig      fiscalaccounting.Config
	OrganizationBatch int
}

func main() {
	logger := observability.NewJSONLogger(serviceName)
	if err := run(logger, os.Getenv); err != nil {
		logger.Error(
			"fiscal accounting worker stopped",
			"event",
			"fiscal_accounting_worker_stopped",
			"error",
			err,
		)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, getenv func(string) string) error {
	if logger == nil {
		return errors.New("fiscal accounting worker logger is required")
	}
	cfg, err := loadEnvironment(getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	database, err := postgres.OpenWithConfig(
		ctx,
		cfg.DatabaseURL,
		postgres.DefaultConfig(serviceName),
	)
	if err != nil {
		return fmt.Errorf("open fiscal accounting worker database: %w", err)
	}
	defer database.Close()

	worker, err := fiscalaccounting.NewWorker(
		database.Pool(),
		cfg.WorkerConfig,
	)
	if err != nil {
		return err
	}
	host, err := workerhost.New(
		database.Pool(),
		worker,
		cfg.OrganizationBatch,
	)
	if err != nil {
		return err
	}

	logger.Info(
		"fiscal accounting worker started",
		"event",
		"fiscal_accounting_worker_started",
		"worker_id",
		cfg.WorkerConfig.WorkerID,
	)
	return host.Run(
		ctx,
		cfg.WorkerConfig.PollInterval,
		func(result workerhost.Result, processErr error) {
			if processErr != nil {
				logger.Warn(
					"fiscal accounting cycle completed with failures",
					"event",
					"fiscal_accounting_processing_degraded",
					"organizations",
					result.OrganizationsFound,
					"processed",
					result.Processed,
					"failed",
					result.Failed,
					"error",
					processErr,
				)
				return
			}
			if result.Processed > 0 {
				logger.Info(
					"fiscal accounting intents posted",
					"event",
					"fiscal_accounting_intents_posted",
					"organizations",
					result.OrganizationsFound,
					"processed",
					result.Processed,
				)
			}
		},
	)
}

func loadEnvironment(getenv func(string) string) (environment, error) {
	if getenv == nil {
		return environment{}, errors.New(
			"fiscal accounting worker environment reader is required",
		)
	}
	workerConfig := fiscalaccounting.DefaultConfig()
	hostname, _ := os.Hostname()
	workerConfig.WorkerID = strings.TrimSpace(
		getenv("PYMES_FISCAL_ACCOUNTING_WORKER_ID"),
	)
	if workerConfig.WorkerID == "" {
		workerConfig.WorkerID = hostname + ":" + strconv.Itoa(os.Getpid())
	}
	if actor := strings.TrimSpace(
		getenv("PYMES_FISCAL_ACCOUNTING_ACTOR"),
	); actor != "" {
		workerConfig.ActorID = actor
	}
	cfg := environment{
		DatabaseURL:       strings.TrimSpace(getenv("PYMES_DATABASE_URL")),
		WorkerConfig:      workerConfig,
		OrganizationBatch: defaultOrganizationBatchSize,
	}
	if cfg.DatabaseURL == "" {
		return environment{}, errors.New("PYMES_DATABASE_URL is required")
	}

	var err error
	if value := strings.TrimSpace(
		getenv("PYMES_FISCAL_ACCOUNTING_POLL_INTERVAL"),
	); value != "" {
		cfg.WorkerConfig.PollInterval, err = time.ParseDuration(value)
		if err != nil || cfg.WorkerConfig.PollInterval <= 0 {
			return environment{}, errors.New(
				"PYMES_FISCAL_ACCOUNTING_POLL_INTERVAL must be positive",
			)
		}
	}
	if value := strings.TrimSpace(
		getenv("PYMES_FISCAL_ACCOUNTING_RETRY_DELAY"),
	); value != "" {
		cfg.WorkerConfig.RetryDelay, err = time.ParseDuration(value)
		if err != nil || cfg.WorkerConfig.RetryDelay < 0 {
			return environment{}, errors.New(
				"PYMES_FISCAL_ACCOUNTING_RETRY_DELAY cannot be negative",
			)
		}
	}
	if value := strings.TrimSpace(
		getenv("PYMES_FISCAL_ACCOUNTING_MAX_ATTEMPTS"),
	); value != "" {
		cfg.WorkerConfig.MaxAttempts, err = boundedPositiveInt(
			value,
			maxAttempts,
		)
		if err != nil {
			return environment{}, errors.New(
				"PYMES_FISCAL_ACCOUNTING_MAX_ATTEMPTS must be between 1 and 1000",
			)
		}
	}
	if value := strings.TrimSpace(
		getenv("PYMES_FISCAL_ACCOUNTING_ORG_BATCH_SIZE"),
	); value != "" {
		cfg.OrganizationBatch, err = boundedPositiveInt(
			value,
			maxOrganizationBatchSize,
		)
		if err != nil {
			return environment{}, errors.New(
				"PYMES_FISCAL_ACCOUNTING_ORG_BATCH_SIZE must be between 1 and 1000",
			)
		}
	}
	if len(workerConfig.WorkerID) > 200 {
		return environment{}, errors.New(
			"PYMES_FISCAL_ACCOUNTING_WORKER_ID is too long",
		)
	}
	if len(workerConfig.ActorID) > 200 {
		return environment{}, errors.New(
			"PYMES_FISCAL_ACCOUNTING_ACTOR is too long",
		)
	}
	return cfg, nil
}

func boundedPositiveInt(value string, maximum int) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > maximum {
		return 0, errors.New("value is outside the allowed range")
	}
	return parsed, nil
}
