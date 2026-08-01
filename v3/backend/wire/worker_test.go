package wire

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
	identityaccess "github.com/devpablocristo/pymes/v3/backend/internal/identity/access"
	workerdomain "github.com/devpablocristo/pymes/v3/backend/internal/worker/domain"
	workerusecases "github.com/devpablocristo/pymes/v3/backend/internal/worker/usecases"
)

type workerDispatcherFunc func(context.Context) error

func (f workerDispatcherFunc) DispatchOnce(ctx context.Context) error {
	return f(ctx)
}

type workerMetricsStub struct {
	calls atomic.Int64
}

func (m *workerMetricsStub) Collect(
	context.Context,
) (workerdomain.Metrics, error) {
	m.calls.Add(1)
	return workerdomain.Metrics{}, nil
}

type closeResourceStub struct {
	calls atomic.Int64
	err   error
}

func (s *closeResourceStub) Close() error {
	s.calls.Add(1)
	return s.err
}

func TestWorkerAppOwnsServerAndRunnerLifecycle(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	metrics := &workerMetricsStub{}
	var dispatches atomic.Int64
	app := &WorkerApp{
		Server: &http.Server{
			Addr:              "127.0.0.1:0",
			Handler:           http.NewServeMux(),
			ReadHeaderTimeout: time.Second,
		},
		runner: workerusecases.Runner{
			Dispatcher: workerDispatcherFunc(func(context.Context) error {
				dispatches.Add(1)
				cancel()
				return nil
			}),
			Metrics:       metrics,
			Logger:        slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
			DispatchEvery: time.Millisecond,
			MetricsEvery:  time.Hour,
		},
		shutdownTimeout: time.Second,
	}
	if err := app.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if dispatches.Load() != 1 || metrics.calls.Load() != 1 {
		t.Fatalf(
			"dispatches=%d metrics=%d",
			dispatches.Load(),
			metrics.calls.Load(),
		)
	}
}

func TestWorkerAppClassifiesRuntimeBoundaryFailures(t *testing.T) {
	t.Parallel()
	app := &WorkerApp{
		Server: &http.Server{
			Addr:    "127.0.0.1:0",
			Handler: http.NewServeMux(),
		},
		runner:          workerusecases.Runner{},
		shutdownTimeout: time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := app.Run(ctx)
	if err == nil || WorkerRunErrorCode(err) != "WORKER_RUNTIME_FAILED" {
		t.Fatalf("err=%v code=%q", err, WorkerRunErrorCode(err))
	}
}

func TestWorkerAppCloseIsIdempotent(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("close failed")
	resource := &closeResourceStub{err: sentinel}
	app := &WorkerApp{identity: resource}
	if err := app.Close(); !errors.Is(err, sentinel) {
		t.Fatalf("first close err=%v", err)
	}
	if err := app.Close(); !errors.Is(err, sentinel) {
		t.Fatalf("second close err=%v", err)
	}
	if resource.calls.Load() != 1 {
		t.Fatalf("close calls=%d", resource.calls.Load())
	}
	if WorkerCloseErrorCode(app.Close()) != "KMS_CLIENT_CLOSE_FAILED" {
		t.Fatalf("close code=%q", WorkerCloseErrorCode(app.Close()))
	}
}

func TestWorkerAppBoundsTracingShutdownAndClosesItOnce(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	var hasDeadline atomic.Bool
	app := &WorkerApp{
		shutdownTracing: func(ctx context.Context) error {
			calls.Add(1)
			if _, ok := ctx.Deadline(); ok {
				hasDeadline.Store(true)
			}
			<-ctx.Done()
			return ctx.Err()
		},
		shutdownTimeout: 5 * time.Millisecond,
	}
	err := app.Close()
	if WorkerCloseErrorCode(err) != "TRACE_SHUTDOWN_FAILED" {
		t.Fatalf("err=%v code=%q", err, WorkerCloseErrorCode(err))
	}
	if err := app.Close(); WorkerCloseErrorCode(err) != "TRACE_SHUTDOWN_FAILED" {
		t.Fatalf("second close err=%v", err)
	}
	if calls.Load() != 1 || !hasDeadline.Load() {
		t.Fatalf(
			"trace shutdown calls=%d deadline=%t",
			calls.Load(),
			hasDeadline.Load(),
		)
	}
}

func TestWorkerCompositionConfiguresOneTraceProvider(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("worker.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(source), "ConfigureTracing(") != 1 ||
		!strings.Contains(string(source), `"pymes-v3-worker"`) {
		t.Fatal("worker must configure exactly one pymes-v3-worker tracer provider")
	}
}

func TestWorkerIdentityAlwaysCreatesSignedInternalTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		allowInsecure     bool
		wantPlatformCalls int
	}{
		{name: "secure workload", wantPlatformCalls: 1},
		{name: "local platform bypass", allowInsecure: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tokenCalls := 0
			platformCalls := 0
			resource := &provisionTokenStub{}
			tokens, platform, closer, err := workerIdentity(
				context.Background(),
				config.WorkerConfig{
					Environment:                "test",
					AllowInsecureLocalServices: test.allowInsecure,
				},
				func(
					_ context.Context,
					subject string,
				) (workerTokenResource, error) {
					tokenCalls++
					if subject != "worker:outbox" {
						t.Fatalf("subject=%q", subject)
					}
					return resource, nil
				},
				func() identityaccess.PlatformTokenSource {
					platformCalls++
					return platformTokenStub{}
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if tokenCalls != 1 || tokens != resource || closer != resource {
				t.Fatalf(
					"token_calls=%d tokens=%T closer=%T",
					tokenCalls,
					tokens,
					closer,
				)
			}
			if platformCalls != test.wantPlatformCalls {
				t.Fatalf(
					"platform_calls=%d want=%d",
					platformCalls,
					test.wantPlatformCalls,
				)
			}
			if test.allowInsecure && platform != nil {
				t.Fatalf(
					"local bypass must omit only the platform token: %T",
					platform,
				)
			}
			if !test.allowInsecure && platform == nil {
				t.Fatal("secure workload requires a platform token")
			}
		})
	}
}

func TestWorkerIdentityRejectsProductionBypass(t *testing.T) {
	t.Parallel()
	tokenCalls := 0
	_, _, _, err := workerIdentity(
		context.Background(),
		config.WorkerConfig{
			Environment:                "production",
			AllowInsecureLocalServices: true,
		},
		func(
			context.Context,
			string,
		) (workerTokenResource, error) {
			tokenCalls++
			return &provisionTokenStub{}, nil
		},
		func() identityaccess.PlatformTokenSource {
			return platformTokenStub{}
		},
	)
	if err == nil || WorkerErrorCode(err) != "WORKLOAD_IDENTITY_INVALID" {
		t.Fatalf("err=%v code=%q", err, WorkerErrorCode(err))
	}
	if tokenCalls != 0 {
		t.Fatalf("invalid production config crossed identity boundary")
	}
}

func TestWorkerIdentityClosesTokenOnCompositionFailure(t *testing.T) {
	t.Parallel()
	resource := &provisionTokenStub{}
	_, _, _, err := workerIdentity(
		context.Background(),
		config.WorkerConfig{Environment: "test"},
		func(
			context.Context,
			string,
		) (workerTokenResource, error) {
			return resource, nil
		},
		func() identityaccess.PlatformTokenSource { return nil },
	)
	if err == nil {
		t.Fatal("expected missing platform identity to fail")
	}
	if resource.closeCalls != 1 {
		t.Fatalf("close_calls=%d", resource.closeCalls)
	}
}

func TestWorkerErrorCodesRemainStable(t *testing.T) {
	t.Parallel()
	startup := workerStartupError(
		"DATABASE_UNAVAILABLE",
		errors.New("database unavailable"),
	)
	if WorkerErrorCode(startup) != "DATABASE_UNAVAILABLE" {
		t.Fatalf("startup code=%q", WorkerErrorCode(startup))
	}
	runtime := workerRuntimeError(
		"SERVER_FAILED",
		errors.New("listen failed"),
	)
	if WorkerRunErrorCode(runtime) != "SERVER_FAILED" {
		t.Fatalf("runtime code=%q", WorkerRunErrorCode(runtime))
	}
	shutdown := workerShutdownError(
		"TRACE_SHUTDOWN_FAILED",
		errors.New("trace flush failed"),
	)
	if WorkerCloseErrorCode(shutdown) != "TRACE_SHUTDOWN_FAILED" {
		t.Fatalf("shutdown code=%q", WorkerCloseErrorCode(shutdown))
	}
}
