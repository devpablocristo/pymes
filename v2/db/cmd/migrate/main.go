package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	migrateapp "github.com/devpablocristo/pymes/v2/db/internal/migrate"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "migrate pymes v2: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := strings.TrimSpace(os.Getenv("PYMES_DATABASE_URL"))
	if databaseURL == "" {
		return fmt.Errorf("PYMES_DATABASE_URL is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	database, err := postgres.OpenWithConfig(ctx, databaseURL, postgres.DefaultConfig("pymes-v2-migrate"))
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	defer database.Close()

	return migrateapp.Up(ctx, database)
}
