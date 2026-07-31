package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type commerceStub struct {
	Commerce
	readyErr error
}

func (s commerceStub) Ready(context.Context) error { return s.readyErr }

func TestReadinessReflectsCommerceDependency(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "ready", status: http.StatusOK},
		{name: "database unavailable", err: errors.New("database unavailable"), status: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			NewHTTPServer(commerceStub{readyErr: test.err}, nil).Handler().ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("expected status %d, got %d", test.status, recorder.Code)
			}
		})
	}
}
