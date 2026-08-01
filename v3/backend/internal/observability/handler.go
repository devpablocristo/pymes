// Package observability provides transport-level evidence without retaining
// credentials, request bodies, tax identifiers, or other PII.
// architecture:adapter handler
package observability

import (
	"log/slog"
	"net/http"
	"time"

	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity"
	handlerdto "github.com/devpablocristo/pymes/v3/backend/internal/observability/handler/dto"
	handlerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/observability/handler/helpers"
)

const (
	requestIDHeader     = "X-Request-ID"
	correlationIDHeader = "X-Correlation-ID"
)

var opaqueTraceID = handlerhelpers.OpaqueTraceID

// HTTP attaches stable request and correlation IDs, emits structured request
// traces, and records successful mutations as redacted audit events. It never
// logs query strings, headers, request bodies, response bodies, or raw errors.
func HTTP(next http.Handler, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r, span := startServerSpan(r)
		ids := handlerdto.RequestIDs{
			RequestID: handlerhelpers.HeaderOrNew(r.Header.Get(requestIDHeader)),
		}
		ids.CorrelationID = handlerhelpers.HeaderOrDefault(
			r.Header.Get(correlationIDHeader), ids.RequestID,
		)
		requestID, correlationID := ids.RequestID, ids.CorrelationID
		w.Header().Set(requestIDHeader, requestID)
		w.Header().Set(correlationIDHeader, correlationID)
		r = r.WithContext(identityusecases.WithRequestMetadata(r.Context(), identityusecases.RequestMetadata{
			RequestID:     requestID,
			CorrelationID: correlationID,
		}))

		startedAt := time.Now()
		response := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(response, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		finishServerSpan(span, r.Method, route, response.status)
		traceID, spanID := traceIDs(r.Context())
		attributes := []slog.Attr{
			slog.String("request_id", requestID),
			slog.String("correlation_id", correlationID),
			slog.String("method", r.Method),
			slog.String("route", route),
			slog.Int("status", response.status),
			slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		}
		if traceID != "" {
			attributes = append(attributes, slog.String("trace_id", traceID))
		}
		if spanID != "" {
			attributes = append(attributes, slog.String("span_id", spanID))
		}
		logger.LogAttrs(r.Context(), slog.LevelInfo, "http_request", attributes...)
		if r.Method != http.MethodGet && response.status >= 200 && response.status < 300 {
			logger.LogAttrs(r.Context(), slog.LevelInfo, "audit_event",
				slog.String("action", r.Method),
				slog.String("resource", route),
				slog.String("organization_id", r.PathValue("organizationID")),
				slog.String("request_id", requestID),
				slog.String("correlation_id", correlationID),
			)
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func headerOrNew(value string) string {
	return handlerhelpers.HeaderOrNew(value)
}

func headerOrDefault(value, fallback string) string {
	return handlerhelpers.HeaderOrDefault(value, fallback)
}
