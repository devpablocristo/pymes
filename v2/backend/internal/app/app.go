package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
	"github.com/devpablocristo/pymes/v2/backend/internal/httpserver"
)

type App struct {
	database        *postgres.DB
	server          *http.Server
	logger          *slog.Logger
	shutdownTimeout time.Duration
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	databaseConfig := postgres.DefaultConfig(config.ServiceName)
	database, err := postgres.OpenWithConfig(ctx, cfg.DatabaseURL, databaseConfig)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	return &App{
		database: database,
		server: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           httpserver.NewHandler(logger, database, cfg.ReadinessTimeout),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		logger:          logger,
		shutdownTimeout: cfg.ShutdownTimeout,
	}, nil
}

func (app *App) Run(ctx context.Context) error {
	if app == nil || app.server == nil {
		return fmt.Errorf("application is not initialized")
	}

	serverErrors := make(chan error, 1)
	go func() {
		app.logger.Info("api listening", "event", "api_listening", "addr", app.server.Addr)
		serverErrors <- app.server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.shutdownTimeout)
		defer cancel()
		if err := app.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http: %w", err)
		}
		app.logger.Info("api stopped gracefully", "event", "api_stopped_gracefully")
		return nil
	}
}

func (app *App) Close() {
	if app != nil && app.database != nil {
		app.database.Close()
	}
}
