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

func TestComposePublicHTTPRoutesSchedulingWithoutCommerceCoupling(t *testing.T) {
	t.Parallel()
	apiCalls := 0
	schedulingCalls := 0
	handler := composePublicHTTP(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			apiCalls++
			w.WriteHeader(http.StatusNoContent)
		}),
		http.NotFoundHandler(),
		publicContextRoute{
			Pattern: "/api/v1/organizations/{organizationId}/scheduling/",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				schedulingCalls++
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			"/api/v1/organizations/org-a/scheduling/bookings",
			nil,
		),
	)
	if response.Code != http.StatusAccepted ||
		schedulingCalls != 1 ||
		apiCalls != 0 {
		t.Fatalf(
			"status=%d scheduling_calls=%d api_calls=%d",
			response.Code,
			schedulingCalls,
			apiCalls,
		)
	}
}

func TestComposePublicHTTPRoutesCalendarCallbackWithoutCommerceCoupling(t *testing.T) {
	t.Parallel()
	apiCalls := 0
	calendarCalls := 0
	handler := composePublicHTTP(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			apiCalls++
			w.WriteHeader(http.StatusNoContent)
		}),
		http.NotFoundHandler(),
		publicContextRoute{
			Pattern: "GET /api/v1/calendars/google/oauth/callback",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calendarCalls++
				w.WriteHeader(http.StatusSeeOther)
			}),
		},
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			"/api/v1/calendars/google/oauth/callback?state=test&code=test",
			nil,
		),
	)
	if response.Code != http.StatusSeeOther ||
		calendarCalls != 1 ||
		apiCalls != 0 {
		t.Fatalf(
			"status=%d calendar_calls=%d api_calls=%d",
			response.Code,
			calendarCalls,
			apiCalls,
		)
	}
}
