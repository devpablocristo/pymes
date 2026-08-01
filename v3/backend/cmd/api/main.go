package main

import (
	"context"
	"errors"
	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
	"github.com/devpablocristo/pymes/v3/backend/internal/observability"
	"github.com/devpablocristo/pymes/v3/backend/wire"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		slog.Error("api startup failed", "code", "CONFIG_INVALID")
		return
	}
	shutdownTracing, err := observability.ConfigureTracing(
		ctx, "pymes-v3-api", cfg.Environment, os.Getenv,
	)
	if err != nil {
		slog.Error("api startup failed", "code", "TRACING_CONFIG_INVALID")
		return
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if traceErr := shutdownTracing(shutdown); traceErr != nil {
			slog.Error("trace shutdown failed", "code", "TRACE_SHUTDOWN_FAILED")
		}
	}()
	app, err := wire.Initialize(ctx, cfg)
	if err != nil {
		slog.Error("api startup failed", "code", "DEPENDENCY_UNAVAILABLE")
		return
	}
	defer app.Close()
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: app.Handler, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			slog.Error("api shutdown failed", "code", "SHUTDOWN_FAILED")
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("api stopped unexpectedly", "code", "SERVER_FAILED")
		}
	}
}
