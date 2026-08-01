package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
	"github.com/devpablocristo/pymes/v3/backend/wire"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	cfg, err := config.LoadWorker()
	if err != nil {
		logger.Error(
			"worker startup failed",
			"code",
			config.WorkerErrorCode(err),
		)
		return
	}
	app, err := wire.InitializeWorker(ctx, cfg, logger)
	if err != nil {
		logger.Error(
			"worker startup failed",
			"code",
			wire.WorkerErrorCode(err),
		)
		return
	}
	defer func() {
		if err := app.Close(); err != nil {
			logger.Error(
				"worker shutdown failed",
				"code",
				wire.WorkerCloseErrorCode(err),
			)
		}
	}()
	if err := app.Run(ctx); err != nil {
		logger.Error(
			"worker stopped",
			"code",
			wire.WorkerRunErrorCode(err),
		)
	}
}
