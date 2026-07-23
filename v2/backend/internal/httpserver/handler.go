package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	observability "github.com/devpablocristo/platform/observability/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
)

type Readiness interface {
	Ping(context.Context) error
}

type healthResponse struct {
	Status string `json:"status"`
}

func NewHandler(logger *slog.Logger, readiness Readiness, readinessTimeout time.Duration) http.Handler {
	return NewHandlerWithIAM(logger, readiness, readinessTimeout, NewIAMAPI(config.ClerkConfig{}))
}

func NewHandlerWithIAM(
	logger *slog.Logger,
	readiness Readiness,
	readinessTimeout time.Duration,
	iamAPI api.ServerInterface,
) http.Handler {
	return NewHandlerWithIAMAndIdempotency(
		logger,
		readiness,
		readinessTimeout,
		iamAPI,
		nil,
	)
}

func NewHandlerWithIAMAndIdempotency(
	logger *slog.Logger,
	readiness Readiness,
	readinessTimeout time.Duration,
	iamAPI api.ServerInterface,
	idempotency *IAMIdempotency,
) http.Handler {
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

	api.HandlerWithOptions(iamAPI, api.StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
		},
	})

	var handler http.Handler = mux
	if idempotency != nil {
		handler = idempotency.Wrap(handler)
	}
	return observability.Middleware(logger, handler)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
