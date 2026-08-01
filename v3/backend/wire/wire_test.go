package wire

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestComposePublicHTTPKeepsGeneratedAPIAndClerkWebhookDisjoint(t *testing.T) {
	t.Parallel()

	apiCalls := 0
	webhookCalls := 0
	handler := composePublicHTTP(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			apiCalls++
			w.WriteHeader(http.StatusNoContent)
		}),
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			webhookCalls++
			w.WriteHeader(http.StatusAccepted)
		}),
	)

	webhookResponse := httptest.NewRecorder()
	handler.ServeHTTP(webhookResponse, httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/clerk", nil))
	if webhookResponse.Code != http.StatusAccepted || webhookCalls != 1 || apiCalls != 0 {
		t.Fatalf("webhook route: status=%d api_calls=%d webhook_calls=%d", webhookResponse.Code, apiCalls, webhookCalls)
	}

	apiResponse := httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if apiResponse.Code != http.StatusNoContent || webhookCalls != 1 || apiCalls != 1 {
		t.Fatalf("generated API route: status=%d api_calls=%d webhook_calls=%d", apiResponse.Code, apiCalls, webhookCalls)
	}
}
