package usecases

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	workerdomain "github.com/devpablocristo/pymes/v3/backend/internal/worker/domain"
	workerports "github.com/devpablocristo/pymes/v3/backend/internal/worker/ports"
)

type dispatcherFunc func(context.Context) error

func (f dispatcherFunc) DispatchOnce(ctx context.Context) error {
	return f(ctx)
}

type countingMetrics struct {
	calls atomic.Int64
	value workerdomain.Metrics
	err   error
}

func (m *countingMetrics) Collect(
	context.Context,
) (workerdomain.Metrics, error) {
	m.calls.Add(1)
	return m.value, m.err
}

type fixedCircuit bool

func (c fixedCircuit) CircuitOpen() bool {
	return bool(c)
}

func TestRunnerOwnsDispatchAndMetricsLoop(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var dispatches atomic.Int64
	metrics := &countingMetrics{}
	runner := Runner{
		Dispatcher: dispatcherFunc(func(context.Context) error {
			dispatches.Add(1)
			cancel()
			return nil
		}),
		Metrics: metrics, Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		DispatchEvery: time.Millisecond, MetricsEvery: time.Hour,
	}
	if err := runner.Run(ctx); err != nil {
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

func TestRunnerRejectsMissingPorts(t *testing.T) {
	t.Parallel()
	if err := (Runner{}).Run(context.Background()); err == nil {
		t.Fatal("missing worker ports were accepted")
	}
}

func TestRunnerRunOnceDispatchesImmediatelyAndTerminates(t *testing.T) {
	t.Parallel()
	metrics := &countingMetrics{}
	var dispatches atomic.Int64
	runner := Runner{
		Dispatcher: dispatcherFunc(func(context.Context) error {
			dispatches.Add(1)
			return nil
		}),
		Metrics: metrics,
		Logger: slog.New(
			slog.NewTextHandler(&bytes.Buffer{}, nil),
		),
		DispatchEvery: time.Hour,
		MetricsEvery:  time.Hour,
		RunOnce:       true,
	}
	if err := runner.Run(context.Background()); err != nil {
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

func TestRunnerRunOnceReturnsDispatchFailure(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("dispatch failed")
	runner := Runner{
		Dispatcher: dispatcherFunc(func(context.Context) error {
			return sentinel
		}),
		Metrics:       &countingMetrics{},
		DispatchEvery: time.Hour,
		MetricsEvery:  time.Hour,
		RunOnce:       true,
	}
	if err := runner.Run(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("err=%v", err)
	}
}

func TestLogMetricsEmitsStablePIIFreeJSON(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	logMetrics(
		context.Background(),
		logger,
		workerdomain.Metrics{
			OutboxPending: 3, OutboxLeased: 1, OutboxRetrying: 2,
			OutboxDeadLetters: 4, OutboxOldestAgeSeconds: 12.5,
			FiscalUncertain: 5, ApplicationPending: 6, ReversalPending: 7,
		},
		map[string]workerports.CircuitState{
			"fiscal": fixedCircuit(true), "accounting": fixedCircuit(false),
		},
	)
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode structured log: %v", err)
	}
	expected := map[string]any{
		"event": "worker_metrics", "ready": true,
		"outbox_pending": float64(3), "outbox_leased": float64(1),
		"outbox_retrying": float64(2), "outbox_dead_letters": float64(4),
		"outbox_oldest_age_seconds": 12.5, "fiscal_uncertain": float64(5),
		"accounting_applications_pending": float64(6),
		"accounting_reversals_pending":    float64(7),
		"dependency_circuits_open":        float64(1),
		"fiscal_circuit_open":             true,
		"accounting_circuit_open":         false,
	}
	for key, want := range expected {
		if got := entry[key]; got != want {
			t.Errorf("%s=%#v want=%#v", key, got, want)
		}
	}
	for _, forbidden := range []string{
		"organization_id", "actor", "cuit", "tax_identifier",
		"token", "credential", "certificate", "payload", "xml",
	} {
		if _, exists := entry[forbidden]; exists {
			t.Errorf("structured metric log contains forbidden field %q", forbidden)
		}
	}
}

func TestEmitMetricsLogsUnavailableWithoutLeakingError(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	metrics := &countingMetrics{err: errors.New("secret database detail")}
	emitMetrics(
		context.Background(),
		logger,
		metrics,
		nil,
		time.Second,
	)
	if strings := output.String(); !bytes.Contains(
		[]byte(strings),
		[]byte(`"code":"METRICS_UNAVAILABLE"`),
	) || bytes.Contains([]byte(strings), []byte("secret database detail")) {
		t.Fatalf("log=%s", strings)
	}
}
