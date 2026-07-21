package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type readinessFunc func(context.Context) error

func (fn readinessFunc) Ping(ctx context.Context) error { return fn(ctx) }

func TestHealthz(t *testing.T) {
	handler := NewHandler(discardLogger(), nil, time.Second)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Request-Id") == "" {
		t.Fatal("missing X-Request-Id response header")
	}
}

func TestReadyzWhenPostgresIsReady(t *testing.T) {
	handler := NewHandler(discardLogger(), readinessFunc(func(context.Context) error { return nil }), time.Second)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	request.Header.Set("X-Request-Id", "caller-request-id")
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ready\"}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Request-Id") != "caller-request-id" {
		t.Fatalf("request id = %q", response.Header().Get("X-Request-Id"))
	}
}

func TestReadyzWhenPostgresIsUnavailable(t *testing.T) {
	handler := NewHandler(discardLogger(), readinessFunc(func(context.Context) error {
		return errors.New("database unavailable")
	}), time.Second)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "{\"status\":\"unavailable\"}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestHealthEndpointsRejectOtherMethods(t *testing.T) {
	handler := NewHandler(discardLogger(), nil, time.Second)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/healthz", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", response.Code)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
