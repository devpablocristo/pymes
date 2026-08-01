package main

import (
	"context"
	"errors"
	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
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
	app, err := wire.Initialize(ctx, cfg)
	if err != nil {
		slog.Error("api startup failed", "code", wire.APIStartupErrorCode(err))
		return
	}
	defer func() {
		if err := app.Close(); err != nil {
			slog.Error("trace shutdown failed", "code", "TRACE_SHUTDOWN_FAILED")
		}
	}()
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
