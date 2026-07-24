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
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/artifacts"
	fiscalstorage "github.com/devpablocristo/pymes/v2/backend/internal/fiscal/storage"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/workerhost"
)

const serviceName = "pymes-v2-fiscal-worker"

type environment struct {
	DatabaseURL   string
	Storage       fiscalstorage.Config
	WorkerID      string
	PollInterval  time.Duration
	LeaseDuration time.Duration
}

func main() {
	logger := observability.NewJSONLogger(serviceName)
	if err := run(logger, os.Getenv); err != nil {
		logger.Error("fiscal worker stopped", "event", "fiscal_worker_stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, getenv func(string) string) error {
	if logger == nil {
		return errors.New("fiscal worker logger is required")
	}
	cfg, err := loadEnvironment(getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := fiscalstorage.Open(ctx, cfg.Storage)
	if err != nil {
		return fmt.Errorf("configure fiscal worker storage: %w", err)
	}
	database, err := postgres.OpenWithConfig(
		ctx, cfg.DatabaseURL, postgres.DefaultConfig(serviceName),
	)
	if err != nil {
		return fmt.Errorf("open fiscal worker database: %w", err)
	}
	defer database.Close()
	host, err := workerhost.New(
		database.Pool(), store.KMS, store.Objects, artifacts.NewRenderer(),
		ar.NewHTTPTransport(), cfg.WorkerID, cfg.LeaseDuration,
	)
	if err != nil {
		return err
	}
	logger.Info("fiscal worker started", "event", "fiscal_worker_started")
	return host.Run(ctx, cfg.PollInterval, func(result workerhost.Result, processErr error) {
		if processErr != nil {
			logger.Warn(
				"fiscal voucher requires retry or reconciliation",
				"event", "fiscal_voucher_processing_degraded",
				"processed", result.Processed,
				"error", processErr,
			)
			return
		}
		if result.Processed {
			logger.Info("fiscal voucher processed", "event", "fiscal_voucher_processed")
		}
	})
}

func loadEnvironment(getenv func(string) string) (environment, error) {
	if getenv == nil {
		return environment{}, fmt.Errorf("fiscal worker environment reader is required")
	}
	hostname, _ := os.Hostname()
	cfg := environment{
		DatabaseURL:   strings.TrimSpace(getenv("PYMES_DATABASE_URL")),
		WorkerID:      strings.TrimSpace(getenv("PYMES_FISCAL_WORKER_ID")),
		PollInterval:  2 * time.Second,
		LeaseDuration: 2 * time.Minute,
	}
	if cfg.WorkerID == "" {
		cfg.WorkerID = hostname + ":" + strconv.Itoa(os.Getpid())
	}
	if cfg.DatabaseURL == "" {
		return environment{}, errors.New("PYMES_DATABASE_URL is required")
	}
	storageConfig, err := fiscalstorage.LoadConfig(
		getenv,
		getenv("PYMES_ENVIRONMENT"),
	)
	if err != nil {
		return environment{}, fmt.Errorf("fiscal worker storage: %w", err)
	}
	cfg.Storage = storageConfig
	if value := strings.TrimSpace(getenv("PYMES_FISCAL_WORKER_POLL_INTERVAL")); value != "" {
		cfg.PollInterval, err = time.ParseDuration(value)
		if err != nil || cfg.PollInterval <= 0 {
			return environment{}, fmt.Errorf("PYMES_FISCAL_WORKER_POLL_INTERVAL must be positive")
		}
	}
	if value := strings.TrimSpace(getenv("PYMES_FISCAL_WORKER_LEASE_DURATION")); value != "" {
		cfg.LeaseDuration, err = time.ParseDuration(value)
		if err != nil || cfg.LeaseDuration <= 0 || cfg.LeaseDuration > 15*time.Minute {
			return environment{}, fmt.Errorf(
				"PYMES_FISCAL_WORKER_LEASE_DURATION must be positive and at most 15m",
			)
		}
	}
	return cfg, nil
}
