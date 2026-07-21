package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	observability "github.com/devpablocristo/platform/observability/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/app"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
)

func main() {
	logger := observability.NewJSONLogger(config.ServiceName)
	if err := run(logger); err != nil {
		logger.Error("api stopped", "event", "api_stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer application.Close()

	return application.Run(ctx)
}
