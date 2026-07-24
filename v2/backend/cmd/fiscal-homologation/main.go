package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	observability "github.com/devpablocristo/platform/observability/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/artifacts"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/homologation"
	fiscalstorage "github.com/devpablocristo/pymes/v2/backend/internal/fiscal/storage"
	"github.com/google/uuid"
)

const serviceName = "pymes-v2-fiscal-homologation"

type options struct {
	OrganizationID uuid.UUID
	Actor          string
	Timeout        time.Duration
	DatabaseURL    string
	Storage        fiscalstorage.Config
}

func main() {
	logger := observability.NewJSONLogger(serviceName)
	ctx, stop := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer stop()
	if err := execute(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		logger.Error(
			"ARCA homologation technical run failed",
			"event", "fiscal_homologation_failed",
			"error", err,
		)
		os.Exit(1)
	}
}

func execute(
	ctx context.Context,
	args []string,
	getenv func(string) string,
	output io.Writer,
) error {
	cfg, err := loadOptions(args, getenv)
	if err != nil {
		return err
	}
	if output == nil {
		return errors.New("homologation output is required")
	}
	runCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	store, err := fiscalstorage.Open(runCtx, cfg.Storage)
	if err != nil {
		return fmt.Errorf("configure homologation fiscal storage: %w", err)
	}
	database, err := postgres.OpenWithConfig(
		runCtx,
		cfg.DatabaseURL,
		postgres.DefaultConfig(serviceName),
	)
	if err != nil {
		return fmt.Errorf("open homologation database: %w", err)
	}
	defer database.Close()
	repository, err := homologation.NewPostgresRepository(database.Pool())
	if err != nil {
		return err
	}
	runner, err := homologation.NewRunner(
		repository,
		store.KMS,
		store.Objects,
		artifacts.NewRenderer(),
		ar.NewHTTPTransport(),
		time.Now,
	)
	if err != nil {
		return err
	}
	result, runErr := runner.Run(runCtx, homologation.Command{
		OrganizationID: cfg.OrganizationID,
		RequestedBy:    cfg.Actor,
	})
	if result.RunID != uuid.Nil {
		if err := json.NewEncoder(output).Encode(result); err != nil {
			return fmt.Errorf("write homologation result: %w", err)
		}
	}
	if runErr != nil {
		return runErr
	}
	return nil
}

func loadOptions(args []string, getenv func(string) string) (options, error) {
	if getenv == nil {
		return options{}, errors.New("homologation environment reader is required")
	}
	// This command performs real network calls to ARCA homologation. Requiring
	// the exact opt-in value avoids accidental execution through a truthy typo.
	if !strings.EqualFold(
		strings.TrimSpace(getenv("PYMES_FISCAL_HOMOLOGATION_ENABLED")),
		"true",
	) {
		return options{}, errors.New(
			"fiscal homologation is disabled; set PYMES_FISCAL_HOMOLOGATION_ENABLED=true explicitly",
		)
	}
	flags := flag.NewFlagSet("fiscal-homologation", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	organizationRaw := flags.String(
		"organization-id", "", "organization UUID to validate in homologation",
	)
	actor := flags.String(
		"actor", "fiscal-homologation-cli", "audit actor recorded with the run",
	)
	timeout := flags.Duration(
		"timeout", 3*time.Minute, "maximum duration for the read-only run",
	)
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, errors.New("homologation command does not accept positional arguments")
	}
	organizationID, err := uuid.Parse(strings.TrimSpace(*organizationRaw))
	if err != nil || organizationID == uuid.Nil {
		return options{}, errors.New("--organization-id must be a non-zero UUID")
	}
	cfg := options{
		OrganizationID: organizationID,
		Actor:          strings.TrimSpace(*actor),
		Timeout:        *timeout,
		DatabaseURL:    strings.TrimSpace(getenv("PYMES_DATABASE_URL")),
	}
	if cfg.Actor == "" {
		return options{}, errors.New("--actor cannot be empty")
	}
	if cfg.Timeout <= 0 || cfg.Timeout > 15*time.Minute {
		return options{}, errors.New("--timeout must be positive and at most 15m")
	}
	if cfg.DatabaseURL == "" {
		return options{}, errors.New("PYMES_DATABASE_URL is required")
	}
	storageConfig, err := fiscalstorage.LoadConfig(
		getenv,
		getenv("PYMES_ENVIRONMENT"),
	)
	if err != nil {
		return options{}, fmt.Errorf("homologation fiscal storage: %w", err)
	}
	cfg.Storage = storageConfig
	return cfg, nil
}
