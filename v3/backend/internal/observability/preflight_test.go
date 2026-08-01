package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	preflighthelpers "github.com/devpablocristo/pymes/v3/backend/internal/observability/preflight/helpers"
	preflightmodels "github.com/devpablocristo/pymes/v3/backend/internal/observability/preflight/models"
)

func TestPreflightGateProtectsOnlyTaggedRevision(t *testing.T) {
	t.Parallel()
	const tag = "candidate-1111111111111111111111111111111111111111"
	token := strings.Repeat("0123456789abcdef", 4)
	handler := PreflightGate(
		http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusNoContent)
		}),
		preflightmodels.Config{Tag: tag, Token: token},
	)
	for _, test := range []struct {
		name   string
		host   string
		token  string
		status int
	}{
		{
			name: "stable service remains public", host: "pymes-api.run.app",
			status: http.StatusNoContent,
		},
		{
			name: "tagged service denies missing capability",
			host: tag + "---pymes-api.run.app", status: http.StatusNotFound,
		},
		{
			name: "tagged service denies wrong capability",
			host: tag + "---pymes-api.run.app", token: token + "0",
			status: http.StatusNotFound,
		},
		{
			name: "tagged service accepts exact capability",
			host: tag + "---pymes-api.run.app", token: token,
			status: http.StatusNoContent,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "https://"+test.host+"/readyz", nil)
			request.Host = test.host
			if test.token != "" {
				request.Header.Set(preflighthelpers.Header, test.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}
