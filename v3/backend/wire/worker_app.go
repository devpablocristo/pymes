package wire

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	postgresadapter "github.com/devpablocristo/pymes/v3/backend/internal/infrastructure/postgres"
	workerusecases "github.com/devpablocristo/pymes/v3/backend/internal/worker/usecases"
)

type closeResource interface {
	Close() error
}

type WorkerApp struct {
	Server          *http.Server
	runner          workerusecases.Runner
	database        *postgresadapter.Database
	identity        closeResource
	shutdownTracing func(context.Context) error
	shutdownTimeout time.Duration
	closeOnce       sync.Once
	closeErr        error
}

type WorkerStartupError struct {
	Code string
	Err  error
}

type WorkerRuntimeError struct {
	Code string
	Err  error
}

type WorkerShutdownError struct {
	Code string
	Err  error
}

type workerComponentResult struct {
	component string
	err       error
}

func (e *WorkerRuntimeError) Error() string {
	return e.Err.Error()
}

func (e *WorkerRuntimeError) Unwrap() error {
	return e.Err
}

func (e *WorkerStartupError) Error() string {
	return e.Err.Error()
}

func (e *WorkerStartupError) Unwrap() error {
	return e.Err
}

func (e *WorkerShutdownError) Error() string {
	return e.Err.Error()
}

func (e *WorkerShutdownError) Unwrap() error {
	return e.Err
}

func (a *WorkerApp) Run(ctx context.Context) error {
	if a == nil || a.Server == nil {
		return fmt.Errorf("worker app is not initialized")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan workerComponentResult, 2)
	go func() {
		err := a.Server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		results <- workerComponentResult{component: "server", err: err}
	}()
	go func() {
		results <- workerComponentResult{
			component: "runner",
			err:       a.runner.Run(runCtx),
		}
	}()

	var runErr error
	completed := 0
	select {
	case <-ctx.Done():
	case result := <-results:
		completed++
		runErr = workerComponentError(result)
	}
	cancel()
	timeout := a.shutdownTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		timeout,
	)
	defer shutdownCancel()
	if err := a.Server.Shutdown(shutdownCtx); err != nil && runErr == nil {
		runErr = workerRuntimeError(
			"SHUTDOWN_FAILED",
			fmt.Errorf("worker operations shutdown: %w", err),
		)
	}
	for completed < 2 {
		select {
		case result := <-results:
			completed++
			if componentErr := workerComponentError(result); runErr == nil {
				runErr = componentErr
			}
		case <-shutdownCtx.Done():
			if runErr == nil {
				runErr = workerRuntimeError(
					"SHUTDOWN_FAILED",
					fmt.Errorf(
						"worker components did not stop: %w",
						shutdownCtx.Err(),
					),
				)
			}
			return runErr
		}
	}
	return runErr
}

func (a *WorkerApp) Close() error {
	if a == nil {
		return nil
	}
	a.closeOnce.Do(func() {
		var shutdownErrors []error
		if a.identity != nil {
			if err := a.identity.Close(); err != nil {
				shutdownErrors = append(
					shutdownErrors,
					workerShutdownError("KMS_CLIENT_CLOSE_FAILED", err),
				)
			}
		}
		if a.shutdownTracing != nil {
			timeout := a.shutdownTimeout
			if timeout <= 0 {
				timeout = 5 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			if err := a.shutdownTracing(ctx); err != nil {
				shutdownErrors = append(
					shutdownErrors,
					workerShutdownError("TRACE_SHUTDOWN_FAILED", err),
				)
			}
			cancel()
		}
		if a.database != nil {
			a.database.Close()
		}
		a.closeErr = errors.Join(shutdownErrors...)
	})
	return a.closeErr
}

func WorkerErrorCode(err error) string {
	var startupErr *WorkerStartupError
	if errors.As(err, &startupErr) && startupErr.Code != "" {
		return startupErr.Code
	}
	return "WORKER_STARTUP_FAILED"
}

func WorkerRunErrorCode(err error) string {
	var runtimeErr *WorkerRuntimeError
	if errors.As(err, &runtimeErr) && runtimeErr.Code != "" {
		return runtimeErr.Code
	}
	return "WORKER_RUNTIME_FAILED"
}

func WorkerCloseErrorCode(err error) string {
	var shutdownErr *WorkerShutdownError
	if errors.As(err, &shutdownErr) && shutdownErr.Code != "" {
		return shutdownErr.Code
	}
	return "WORKER_SHUTDOWN_FAILED"
}

func workerStartupError(code string, err error) error {
	return &WorkerStartupError{Code: code, Err: err}
}

func workerRuntimeError(code string, err error) error {
	return &WorkerRuntimeError{Code: code, Err: err}
}

func workerShutdownError(code string, err error) error {
	return &WorkerShutdownError{Code: code, Err: err}
}

func workerComponentError(result workerComponentResult) error {
	if result.err == nil {
		return nil
	}
	if result.component == "server" {
		return workerRuntimeError(
			"SERVER_FAILED",
			fmt.Errorf("worker operations server: %w", result.err),
		)
	}
	return workerRuntimeError(
		"WORKER_RUNTIME_FAILED",
		fmt.Errorf("worker runtime: %w", result.err),
	)
}
