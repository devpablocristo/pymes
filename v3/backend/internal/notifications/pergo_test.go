package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	pergohelpers "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestPerGoSendsStableIdentityWithoutLoggingOrLeakingProviderTypes(t *testing.T) {
	intent := domain.Intent{
		ID: "notification-1", OrganizationID: "org-1",
		Kind: domain.KindConfirmation, RecipientE164: "+5491112345678",
		TemplateName: "booking.confirmation", TemplateVersion: 3,
		Locale: "es_AR", Body: "Confirmado", IdempotencyKey: "booking-1:v3",
		CorrelationID: "correlation-1",
	}
	expectedTraceID, err := pergohelpers.TraceID(
		intent.OrganizationID,
		intent.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	client := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer secret" ||
			request.Header.Get("X-Trace-ID") != expectedTraceID ||
			request.Header.Get("Idempotency-Key") != intent.IdempotencyKey {
			t.Fatalf("unexpected headers: %#v", request.Header)
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["to"] != "5491112345678" || payload["body"] != "Confirmado" {
			t.Fatalf("unexpected PerGo payload: %#v", payload)
		}
		metadata := payload["metadata"].(map[string]any)
		if metadata["pymes_org_id"] != "org-1" ||
			metadata["pymes_template_version"] != "3" {
			t.Fatalf("unexpected metadata: %#v", metadata)
		}
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body: io.NopCloser(strings.NewReader(
				`{"message_id":"external-1","status":"queued","queued_at":"2026-08-01T00:00:00Z"}`,
			)),
		}, nil
	})
	result, err := (PerGo{
		BaseURL: "http://pergo", APIKey: "secret",
		Channel: "whatsapp", Client: client,
	}).Send(context.Background(), intent)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExternalMessageID != "external-1" {
		t.Fatalf("external ID = %q", result.ExternalMessageID)
	}
}

func TestPerGoClassifiesLostResponseAsUncertainAndFourHundredAsTerminal(t *testing.T) {
	intent := domain.Intent{
		ID: "notification-1", OrganizationID: "org-1",
		Kind: domain.KindReminder, RecipientE164: "+5491112345678",
		TemplateName: "booking.reminder", TemplateVersion: 1,
		Locale: "es_AR", Body: "Recordatorio", IdempotencyKey: "reminder-1",
		CorrelationID: "correlation-1",
	}
	_, err := (PerGo{
		BaseURL: "http://pergo", APIKey: "secret",
		Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	}).Send(context.Background(), intent)
	providerError, ok := AsProviderError(err)
	if !ok || !providerError.Retry || !providerError.Unknown {
		t.Fatalf("uncertain error = %#v, %v", providerError, err)
	}
	_, err = (PerGo{
		BaseURL: "http://pergo", APIKey: "secret",
		Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnprocessableEntity,
				Body:       io.NopCloser(strings.NewReader(`{"secret":"must-not-surface"}`)),
			}, nil
		}),
	}).Send(context.Background(), intent)
	providerError, ok = AsProviderError(err)
	if !ok || providerError.Retry ||
		!errors.Is(providerError, providerError.Cause) {
		t.Fatalf("terminal error = %#v, %v", providerError, err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatal("provider body leaked through error")
	}
}
