package main

import (
	"context"
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
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/iamsync"
)

const serviceName = "pymes-v2-iam-sync-worker"

func main() {
	logger := observability.NewJSONLogger(serviceName)
	if err := run(logger, os.Getenv); err != nil {
		logger.Error("IAM sync worker stopped", "event", "iam_sync_worker_stopped", "error", err)
		os.Exit(1)
	}
}

type environment struct {
	DatabaseURL  string
	ClerkSecret  string
	ClerkBaseURL string
	Worker       iamsync.WorkerConfig
}

func run(logger *slog.Logger, getenv func(string) string) error {
	if logger == nil {
		return fmt.Errorf("IAM sync worker logger is required")
	}
	cfg, err := loadEnvironment(getenv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	database, err := postgres.OpenWithConfig(
		ctx,
		cfg.DatabaseURL,
		postgres.DefaultConfig(serviceName),
	)
	if err != nil {
		return err
	}
	defer database.Close()

	provider := clerk.New(clerk.Config{
		SecretKey: cfg.ClerkSecret,
		BaseURL:   cfg.ClerkBaseURL,
	})
	worker, err := iamsync.NewPostgresWorker(database.Pool(), provider, cfg.Worker)
	if err != nil {
		return err
	}

	logger.Info("IAM sync worker started", "event", "iam_sync_worker_started")
	return worker.Run(ctx, func(result platformoutbox.DispatchResult, dispatchErr error) {
		if dispatchErr != nil {
			logger.Error(
				"IAM sync dispatch degraded",
				"event", "iam_sync_dispatch_failed",
				"leased", result.Leased,
				"published", result.Published,
				"retried", result.Retried,
				"failed", result.Failed,
				"error", dispatchErr,
			)
			return
		}
		if result.Leased > 0 {
			logger.Info(
				"IAM sync command dispatched",
				"event", "iam_sync_dispatched",
				"leased", result.Leased,
				"published", result.Published,
			)
		}
	})
}

func loadEnvironment(getenv func(string) string) (environment, error) {
	if getenv == nil {
		return environment{}, fmt.Errorf("IAM sync environment reader is required")
	}
	cfg := environment{
		DatabaseURL:  strings.TrimSpace(getenv("PYMES_DATABASE_URL")),
		ClerkSecret:  strings.TrimSpace(getenv("PYMES_CLERK_SECRET_KEY")),
		ClerkBaseURL: strings.TrimRight(strings.TrimSpace(getenv("PYMES_CLERK_API_URL")), "/"),
		Worker:       iamsync.DefaultWorkerConfig(),
	}
	if cfg.DatabaseURL == "" {
		return environment{}, fmt.Errorf("PYMES_DATABASE_URL is required")
	}
	if cfg.ClerkSecret == "" {
		return environment{}, fmt.Errorf("PYMES_CLERK_SECRET_KEY is required")
	}

	var err error
	cfg.Worker.BatchSize, err = positiveInt(
		getenv("PYMES_IAM_SYNC_WORKER_BATCH_SIZE"),
		cfg.Worker.BatchSize,
	)
	if err != nil {
		return environment{}, fmt.Errorf("PYMES_IAM_SYNC_WORKER_BATCH_SIZE: %w", err)
	}
	cfg.Worker.PollInterval, err = positiveDuration(
		getenv("PYMES_IAM_SYNC_WORKER_POLL_INTERVAL"),
		cfg.Worker.PollInterval,
	)
	if err != nil {
		return environment{}, fmt.Errorf("PYMES_IAM_SYNC_WORKER_POLL_INTERVAL: %w", err)
	}
	cfg.Worker.LeaseDuration, err = positiveDuration(
		getenv("PYMES_IAM_SYNC_WORKER_LEASE_DURATION"),
		cfg.Worker.LeaseDuration,
	)
	if err != nil {
		return environment{}, fmt.Errorf("PYMES_IAM_SYNC_WORKER_LEASE_DURATION: %w", err)
	}
	return cfg, nil
}

func positiveDuration(value string, fallback time.Duration) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return duration, nil
}

func positiveInt(value string, fallback int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return parsed, nil
}
