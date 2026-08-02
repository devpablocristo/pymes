package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	pergohelpers "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

type platformTokenFunc func(context.Context, string) (string, error)

func (function platformTokenFunc) PlatformToken(
	ctx context.Context,
	audience string,
) (string, error) {
	return function(ctx, audience)
}

func TestPerGoSendsStableIdentityWithoutLoggingOrLeakingProviderTypes(t *testing.T) {
	intent := domain.Intent{
		ID: "notification-1", OrganizationID: "org-1",
		Kind: domain.KindConfirmation, RecipientE164: "+5491112345678",
		TemplateName: "booking.confirmation", TemplateVersion: 3,
		Locale: "es_AR", Body: "Confirmado", IdempotencyKey: "booking-1:v3",
		CorrelationID:   "correlation-1",
		DeliveryChannel: "whatsapp_cloud",
		SenderIdentity:  "5491100000000",
	}
	expectedTraceID, err := pergohelpers.TraceID(
		intent.OrganizationID,
		intent.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	expectedIdempotencyKey, err := pergohelpers.IngressIdempotencyKey(
		intent.OrganizationID,
		intent.IdempotencyKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	client := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer secret" ||
			request.Header.Get("X-Serverless-Authorization") !=
				"Bearer workload-token" ||
			request.Header.Get("X-Trace-ID") != expectedTraceID ||
			request.Header.Get("Idempotency-Key") != expectedIdempotencyKey {
			t.Fatalf("unexpected headers: %#v", request.Header)
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["to"] != "5491112345678" ||
			payload["from"] != "5491100000000" ||
			payload["channel"] != "whatsapp_cloud" ||
			payload["body"] != "Confirmado" {
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
		Audience: "https://pergo.example",
		PlatformTokens: platformTokenFunc(
			func(_ context.Context, audience string) (string, error) {
				if audience != "https://pergo.example" {
					t.Fatalf("audience=%q", audience)
				}
				return "workload-token", nil
			},
		),
		Channel: "whatsapp", Client: client,
	}).Send(context.Background(), intent)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExternalMessageID != "external-1" {
		t.Fatalf("external ID = %q", result.ExternalMessageID)
	}
}

func TestPerGoResolvesPlatformIdentityForEverySendAndFailsClosedOnTokenError(
	t *testing.T,
) {
	intent := domain.Intent{
		ID: "notification-1", OrganizationID: "org-1",
		Kind: domain.KindConfirmation, RecipientE164: "+5491112345678",
		TemplateName: "booking.confirmation", TemplateVersion: 1,
		Locale: "es_AR", Body: "Confirmado", IdempotencyKey: "booking-1",
		DeliveryChannel: "whatsapp", SenderIdentity: "5491100000000",
	}
	tokenCalls := 0
	requests := 0
	adapter := PerGo{
		BaseURL:  "https://pergo.example",
		APIKey:   "application-api-key",
		Audience: "https://pergo.example",
		PlatformTokens: platformTokenFunc(
			func(_ context.Context, audience string) (string, error) {
				tokenCalls++
				if audience != "https://pergo.example" {
					t.Fatalf("audience=%q", audience)
				}
				if tokenCalls == 3 {
					return "", errors.New("metadata unavailable")
				}
				return "workload-token-" + strconv.Itoa(tokenCalls), nil
			},
		),
		Client: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			if request.Header.Get("Authorization") !=
				"Bearer application-api-key" {
				t.Fatalf(
					"application authorization=%q",
					request.Header.Get("Authorization"),
				)
			}
			want := "Bearer workload-token-" + strconv.Itoa(requests)
			if got := request.Header.Get("X-Serverless-Authorization"); got != want {
				t.Fatalf("serverless authorization=%q want=%q", got, want)
			}
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Body: io.NopCloser(strings.NewReader(
					`{"message_id":"external-1","status":"queued","queued_at":"2026-08-01T00:00:00Z"}`,
				)),
			}, nil
		}),
	}
	for range 2 {
		if _, err := adapter.Send(context.Background(), intent); err != nil {
			t.Fatal(err)
		}
	}
	if tokenCalls != 2 || requests != 2 {
		t.Fatalf("token calls=%d requests=%d", tokenCalls, requests)
	}

	_, err := adapter.Send(context.Background(), intent)
	providerError, ok := AsProviderError(err)
	if !ok ||
		providerError.StableCode != "PERGO_PLATFORM_IDENTITY_UNAVAILABLE" ||
		!providerError.Retry ||
		providerError.Unknown {
		t.Fatalf("platform identity error=%#v err=%v", providerError, err)
	}
	if requests != 2 {
		t.Fatalf("request crossed boundary after token error: %d", requests)
	}
}

func TestPerGoClassifiesLostResponseAsUncertainAndFourHundredAsTerminal(t *testing.T) {
	intent := domain.Intent{
		ID: "notification-1", OrganizationID: "org-1",
		Kind: domain.KindReminder, RecipientE164: "+5491112345678",
		TemplateName: "booking.reminder", TemplateVersion: 1,
		Locale: "es_AR", Body: "Recordatorio", IdempotencyKey: "reminder-1",
		CorrelationID:   "correlation-1",
		DeliveryChannel: "whatsapp",
		SenderIdentity:  "5491100000000",
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
