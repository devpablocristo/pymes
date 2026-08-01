package observability

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity"
)

func TestHTTPPropagatesIDsAndRedactsRequestData(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	var metadata identityusecases.RequestMetadata
	handler := HTTP(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var ok bool
		metadata, ok = identityusecases.RequestMetadataFromContext(request.Context())
		if !ok {
			t.Fatal("request metadata missing from context")
		}
		w.WriteHeader(http.StatusCreated)
	}), logger)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-secret/sales?token=leak", strings.NewReader(`{"cuit":"20-12345678-9","token":"do-not-log"}`))
	request.Header.Set("Authorization", "Bearer do-not-log")
	request.Header.Set(correlationIDHeader, "corr-1")
	request.SetPathValue("organizationID", "org-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Header().Get(correlationIDHeader) != "corr-1" || response.Header().Get(requestIDHeader) == "" {
		t.Fatalf("response headers = %#v", response.Header())
	}
	if metadata.CorrelationID != "corr-1" || metadata.RequestID != response.Header().Get(requestIDHeader) {
		t.Fatalf("context metadata = %#v", metadata)
	}
	logged := output.String()
	for _, forbidden := range []string{"do-not-log", "20-12345678-9", "token=leak", "Bearer"} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("log leaked %q: %s", forbidden, logged)
		}
	}
	for _, required := range []string{"http_request", "audit_event", "corr-1", "org-secret"} {
		if !strings.Contains(logged, required) {
			t.Fatalf("log is missing %q: %s", required, logged)
		}
	}
}

func TestHTTPDoesNotAuditFailedMutation(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := HTTP(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}), logger)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if strings.Contains(output.String(), "audit_event") {
		t.Fatalf("failed mutation must not be audited as successful: %s", output.String())
	}
}

func TestHTTPRejectsNonOpaqueOrOversizedTraceHeaders(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name          string
		requestID     string
		correlationID string
	}{
		{name: "possible PII", requestID: "person@example.com", correlationID: "customer name"},
		{name: "oversized", requestID: strings.Repeat("a", 256), correlationID: strings.Repeat("b", 256)},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var metadata identityusecases.RequestMetadata
			next := http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
				metadata, _ = identityusecases.RequestMetadataFromContext(request.Context())
			})
			request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			request.Header.Set(requestIDHeader, test.requestID)
			request.Header.Set(correlationIDHeader, test.correlationID)
			response := httptest.NewRecorder()

			HTTP(next, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))).ServeHTTP(response, request)

			if metadata.RequestID == test.requestID || metadata.CorrelationID == test.correlationID {
				t.Fatalf("unsafe trace metadata was accepted: %+v", metadata)
			}
			if !opaqueTraceID.MatchString(metadata.RequestID) ||
				metadata.CorrelationID != metadata.RequestID {
				t.Fatalf("fallback metadata is not canonical: %+v", metadata)
			}
		})
	}
}
