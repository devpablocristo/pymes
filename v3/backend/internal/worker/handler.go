// Package handler contains inbound HTTP adapters for worker operations.
// architecture:adapter handler
package worker

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	handlerdto "github.com/devpablocristo/pymes/v3/backend/internal/worker/handler/dto"
	handlerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/worker/handler/helpers"
)

type Readiness interface {
	Ready(context.Context) error
}

type HTTP struct {
	Readiness Readiness
	Metrics   MetricsReader
	Circuits  map[string]CircuitState
}

func (h HTTP) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /readyz", h.ready)
	mux.HandleFunc("GET /metrics", h.metrics)
	return mux
}

func (h HTTP) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, handlerdto.Health{Status: "ok"})
}

func (h HTTP) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if h.Readiness == nil || h.Readiness.Ready(ctx) != nil {
		writeJSON(
			w,
			http.StatusServiceUnavailable,
			handlerdto.Health{Status: "not_ready"},
		)
		return
	}
	writeJSON(w, http.StatusOK, handlerdto.Health{Status: "ready"})
}

func (h HTTP) metrics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if h.Metrics == nil {
		http.Error(w, "metrics unavailable", http.StatusServiceUnavailable)
		return
	}
	metrics, err := h.Metrics.Collect(ctx)
	if err != nil {
		http.Error(w, "metrics unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(
		w,
		"pymes_outbox_pending %d\n"+
			"pymes_outbox_leased %d\n"+
			"pymes_outbox_retrying %d\n"+
			"pymes_outbox_dead_letters %d\n"+
			"pymes_outbox_oldest_age_seconds %.3f\n"+
			"pymes_fiscal_uncertain %d\n"+
			"pymes_accounting_applications_pending %d\n"+
			"pymes_accounting_reversals_pending %d\n",
		metrics.OutboxPending,
		metrics.OutboxLeased,
		metrics.OutboxRetrying,
		metrics.OutboxDeadLetters,
		metrics.OutboxOldestAgeSeconds,
		metrics.FiscalUncertain,
		metrics.ApplicationPending,
		metrics.ReversalPending,
	)
	names := make([]string, 0, len(h.Circuits))
	for name := range h.Circuits {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := 0
		if circuit := h.Circuits[name]; circuit != nil && circuit.CircuitOpen() {
			value = 1
		}
		_, _ = fmt.Fprintf(
			w,
			"pymes_dependency_circuit_open{dependency=%q} %d\n",
			name,
			value,
		)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	handlerhelpers.WriteJSON(w, status, value)
}
