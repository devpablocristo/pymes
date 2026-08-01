package worker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	workerdomain "github.com/devpablocristo/pymes/v3/backend/internal/worker/usecases/domain"
)

type readinessStub struct {
	err error
}

func (s readinessStub) Ready(context.Context) error {
	return s.err
}

type metricsStub struct {
	value workerdomain.Metrics
	err   error
}

func (s metricsStub) Collect(
	context.Context,
) (workerdomain.Metrics, error) {
	return s.value, s.err
}

type fixedCircuit bool

func (c fixedCircuit) CircuitOpen() bool {
	return bool(c)
}

func TestOperationalHTTPKeepsHealthAndReadinessSemantics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		path       string
		readiness  Readiness
		wantStatus int
		wantBody   string
	}{
		{
			name: "health is process liveness", path: "/healthz",
			wantStatus: http.StatusOK, wantBody: `"status":"ok"`,
		},
		{
			name: "ready durable dependency", path: "/readyz",
			readiness: readinessStub{}, wantStatus: http.StatusOK,
			wantBody: `"status":"ready"`,
		},
		{
			name: "not ready dependency", path: "/readyz",
			readiness:  readinessStub{err: errors.New("database unavailable")},
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `"status":"not_ready"`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := HTTP{
				Readiness: test.readiness,
				Metrics:   metricsStub{},
			}.Handler()
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(
				recorder,
				httptest.NewRequest(http.MethodGet, test.path, nil),
			)
			if recorder.Code != test.wantStatus ||
				!strings.Contains(recorder.Body.String(), test.wantBody) {
				t.Fatalf(
					"status=%d body=%q",
					recorder.Code,
					recorder.Body.String(),
				)
			}
		})
	}
}

func TestOperationalHTTPRendersStableMetricsWithoutTenantData(t *testing.T) {
	t.Parallel()
	handler := HTTP{
		Readiness: readinessStub{},
		Metrics: metricsStub{value: workerdomain.Metrics{
			OutboxPending: 3, OutboxLeased: 1, OutboxRetrying: 2,
			OutboxDeadLetters: 4, OutboxOldestAgeSeconds: 12.5,
			FiscalUncertain: 5, ApplicationPending: 6, ReversalPending: 7,
			NotificationsStalled: 8, NotificationsFailed: 9,
		}},
		Circuits: map[string]CircuitState{
			"fiscal": fixedCircuit(true), "accounting": fixedCircuit(false),
		},
	}.Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/metrics", nil),
	)
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK ||
		recorder.Header().Get("Content-Type") != "text/plain; version=0.0.4" {
		t.Fatalf("status=%d headers=%v body=%q", recorder.Code, recorder.Header(), body)
	}
	for _, expected := range []string{
		"pymes_outbox_pending 3\n",
		"pymes_outbox_leased 1\n",
		"pymes_outbox_retrying 2\n",
		"pymes_outbox_dead_letters 4\n",
		"pymes_outbox_oldest_age_seconds 12.500\n",
		"pymes_fiscal_uncertain 5\n",
		"pymes_notifications_stalled 8\n",
		"pymes_notifications_failed 9\n",
		"pymes_accounting_applications_pending 6\n",
		"pymes_accounting_reversals_pending 7\n",
		`pymes_dependency_circuit_open{dependency="accounting"} 0`,
		`pymes_dependency_circuit_open{dependency="fiscal"} 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("metrics body lacks %q:\n%s", expected, body)
		}
	}
	for _, forbidden := range []string{
		"organization_id", `actor="`, `cuit="`, "tax_identifier",
		`token="`, `credential="`, `certificate="`, `payload="`, `xml="`,
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("metrics body contains forbidden field %q", forbidden)
		}
	}
}

func TestOperationalHTTPFailsClosedWhenMetricsUnavailable(t *testing.T) {
	t.Parallel()
	handler := HTTP{
		Readiness: readinessStub{},
		Metrics:   metricsStub{err: errors.New("database unavailable")},
	}.Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/metrics", nil),
	)
	if recorder.Code != http.StatusServiceUnavailable ||
		recorder.Body.String() != "metrics unavailable\n" {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
