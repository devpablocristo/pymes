package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	if err := runWorker(ctx, app); err != nil {
		logger.Error(
			"worker stopped",
			"code",
			workerRunErrorCode(err),
		)
	}
}

type workerRuntimeError struct {
	Code string
	Err  error
}

type workerComponentResult struct {
	component string
	err       error
}

func (err *workerRuntimeError) Error() string { return err.Err.Error() }
func (err *workerRuntimeError) Unwrap() error { return err.Err }

func runWorker(ctx context.Context, app *wire.WorkerApp) error {
	if app == nil || app.Server == nil {
		return fmt.Errorf("worker app is not initialized")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan workerComponentResult, 2)
	go func() {
		err := app.Server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		results <- workerComponentResult{component: "server", err: err}
	}()
	go func() {
		results <- workerComponentResult{
			component: "runner",
			err:       app.Runner.Run(runCtx),
		}
	}()

	var runErr error
	completed := 0
	select {
	case <-ctx.Done():
	case result := <-results:
		completed++
		runErr = classifyWorkerComponent(result)
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := app.Server.Shutdown(shutdownCtx); err != nil && runErr == nil {
		runErr = newWorkerRuntimeError(
			"SHUTDOWN_FAILED",
			fmt.Errorf("worker operations shutdown: %w", err),
		)
	}
	for completed < 2 {
		select {
		case result := <-results:
			completed++
			if componentErr := classifyWorkerComponent(result); runErr == nil {
				runErr = componentErr
			}
		case <-shutdownCtx.Done():
			if runErr == nil {
				runErr = newWorkerRuntimeError(
					"SHUTDOWN_FAILED",
					fmt.Errorf("worker components did not stop: %w", shutdownCtx.Err()),
				)
			}
			return runErr
		}
	}
	return runErr
}

func newWorkerRuntimeError(code string, err error) error {
	return &workerRuntimeError{Code: code, Err: err}
}

func workerRunErrorCode(err error) string {
	var runtimeErr *workerRuntimeError
	if errors.As(err, &runtimeErr) && runtimeErr.Code != "" {
		return runtimeErr.Code
	}
	return "WORKER_RUNTIME_FAILED"
}

func classifyWorkerComponent(result workerComponentResult) error {
	if result.err == nil {
		return nil
	}
	if result.component == "server" {
		return newWorkerRuntimeError(
			"SERVER_FAILED",
			fmt.Errorf("worker operations server: %w", result.err),
		)
	}
	return newWorkerRuntimeError(
		"WORKER_RUNTIME_FAILED",
		fmt.Errorf("worker runtime: %w", result.err),
	)
}
