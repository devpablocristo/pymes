package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	observability "github.com/devpablocristo/platform/observability/go"
)

type Readiness interface {
	Ping(context.Context) error
}

type healthResponse struct {
	Status string `json:"status"`
}

func NewHandler(logger *slog.Logger, readiness Readiness, readinessTimeout time.Duration) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if readiness == nil {
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()
		if err := readiness.Ping(ctx); err != nil {
			observability.LoggerFromContext(r.Context()).Warn(
				"readiness check failed",
				"event", "readiness_check_failed",
				"error", err,
			)
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ready"})
	})

	return observability.Middleware(logger, mux)
}

func writeJSON(w http.ResponseWriter, status int, body healthResponse) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
