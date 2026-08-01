package wire

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v3/backend/cmd/config"
)

type closeResourceStub struct {
	calls atomic.Int64
	err   error
}

func (s *closeResourceStub) Close() error {
	s.calls.Add(1)
	return s.err
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

func TestWorkerAppClosesCalendarResourcesExactlyOnce(t *testing.T) {
	t.Parallel()
	resource := &closeResourceStub{}
	app := &WorkerApp{
		resources: compactCloseResources(nil, resource),
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	if resource.calls.Load() != 1 {
		t.Fatalf("calendar resource close calls=%d", resource.calls.Load())
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
	source, err := os.ReadFile("wire.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(source), "ConfigureTracing(") != 2 ||
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
				func() workerPlatformTokenSource {
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
		func() workerPlatformTokenSource {
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
		func() workerPlatformTokenSource { return nil },
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
	shutdown := workerShutdownError(
		"TRACE_SHUTDOWN_FAILED",
		errors.New("trace flush failed"),
	)
	if WorkerCloseErrorCode(shutdown) != "TRACE_SHUTDOWN_FAILED" {
		t.Fatalf("shutdown code=%q", WorkerCloseErrorCode(shutdown))
	}
}
