package httpserver

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
)

type clerkWebhookVerifierFunc func(
	[]byte,
	http.Header,
) (clerkadapter.WebhookEvent, error)

func (fn clerkWebhookVerifierFunc) VerifyAndDecode(
	payload []byte,
	headers http.Header,
) (clerkadapter.WebhookEvent, error) {
	return fn(payload, headers)
}

type webhookInboxFunc func(
	context.Context,
	platformiam.WebhookEvent,
) (platformiam.WebhookEvent, bool, error)

func (fn webhookInboxFunc) ReceiveWebhookEvent(
	ctx context.Context,
	event platformiam.WebhookEvent,
) (platformiam.WebhookEvent, bool, error) {
	return fn(ctx, event)
}

func TestClerkWebhookFailsClosedWhenNotConfigured(t *testing.T) {
	verifierCalled := false
	inboxCalled := false
	handler := clerkWebhookTestHandler(
		config.ClerkConfig{},
		clerkWebhookVerifierFunc(func(
			[]byte,
			http.Header,
		) (clerkadapter.WebhookEvent, error) {
			verifierCalled = true
			return clerkadapter.WebhookEvent{}, nil
		}),
		webhookInboxFunc(func(
			context.Context,
			platformiam.WebhookEvent,
		) (platformiam.WebhookEvent, bool, error) {
			inboxCalled = true
			return platformiam.WebhookEvent{}, false, nil
		}),
	)

	response := performClerkWebhookRequest(handler, []byte(`{"type":"user.created"}`))

	assertAPIError(t, response, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED")
	if verifierCalled {
		t.Fatal("webhook verifier was called without Clerk configuration")
	}
	if inboxCalled {
		t.Fatal("webhook inbox was called without Clerk configuration")
	}
}

func TestClerkWebhookRejectsInvalidSignature(t *testing.T) {
	inboxCalled := false
	handler := clerkWebhookTestHandler(
		config.ClerkConfig{WebhookSecret: "whsec_test"},
		clerkWebhookVerifierFunc(func(
			[]byte,
			http.Header,
		) (clerkadapter.WebhookEvent, error) {
			return clerkadapter.WebhookEvent{}, errors.Join(
				errors.New("signature verification failed"),
				clerkadapter.ErrInvalidWebhookSignature,
			)
		}),
		webhookInboxFunc(func(
			context.Context,
			platformiam.WebhookEvent,
		) (platformiam.WebhookEvent, bool, error) {
			inboxCalled = true
			return platformiam.WebhookEvent{}, false, nil
		}),
	)

	response := performClerkWebhookRequest(handler, []byte(`{"type":"user.created"}`))

	assertAPIError(t, response, http.StatusUnauthorized, "WEBHOOK_INVALID_SIGNATURE")
	if inboxCalled {
		t.Fatal("webhook inbox was called for an invalid signature")
	}
}

func TestClerkWebhookRejectsInvalidPayload(t *testing.T) {
	inboxCalled := false
	handler := clerkWebhookTestHandler(
		config.ClerkConfig{WebhookSecret: "whsec_test"},
		clerkWebhookVerifierFunc(func(
			[]byte,
			http.Header,
		) (clerkadapter.WebhookEvent, error) {
			return clerkadapter.WebhookEvent{}, errors.Join(
				errors.New("payload decode failed"),
				clerkadapter.ErrInvalidWebhookPayload,
			)
		}),
		webhookInboxFunc(func(
			context.Context,
			platformiam.WebhookEvent,
		) (platformiam.WebhookEvent, bool, error) {
			inboxCalled = true
			return platformiam.WebhookEvent{}, false, nil
		}),
	)

	response := performClerkWebhookRequest(handler, []byte(`{"broken":`))

	assertAPIError(t, response, http.StatusBadRequest, "WEBHOOK_INVALID_PAYLOAD")
	if inboxCalled {
		t.Fatal("webhook inbox was called for an invalid payload")
	}
}

func TestClerkWebhookRejectsBodyLargerThanOneMiB(t *testing.T) {
	verifierCalled := false
	inboxCalled := false
	handler := clerkWebhookTestHandler(
		config.ClerkConfig{WebhookSecret: "whsec_test"},
		clerkWebhookVerifierFunc(func(
			[]byte,
			http.Header,
		) (clerkadapter.WebhookEvent, error) {
			verifierCalled = true
			return clerkadapter.WebhookEvent{}, nil
		}),
		webhookInboxFunc(func(
			context.Context,
			platformiam.WebhookEvent,
		) (platformiam.WebhookEvent, bool, error) {
			inboxCalled = true
			return platformiam.WebhookEvent{}, false, nil
		}),
	)
	payload := bytes.Repeat([]byte("x"), (1<<20)+1)

	response := performClerkWebhookRequest(handler, payload)

	assertAPIError(t, response, http.StatusBadRequest, "WEBHOOK_INVALID_PAYLOAD")
	if verifierCalled {
		t.Fatal("webhook verifier was called for an oversized body")
	}
	if inboxCalled {
		t.Fatal("webhook inbox was called for an oversized body")
	}
}

func TestClerkWebhookPersistsVerifiedEvent(t *testing.T) {
	payload := []byte(
		`{"data":{"id":"user_123"},"object":"event","type":"user.updated"}`,
	)
	occurredAt := time.Date(2026, time.July, 23, 14, 30, 0, 0, time.UTC)
	providerEvent := clerkadapter.WebhookEvent{
		ID:        "evt_123",
		Type:      clerkadapter.WebhookUserUpdated,
		Timestamp: occurredAt,
	}
	var received platformiam.WebhookEvent
	handler := clerkWebhookTestHandler(
		config.ClerkConfig{WebhookSecret: "whsec_test"},
		clerkWebhookVerifierFunc(func(
			gotPayload []byte,
			headers http.Header,
		) (clerkadapter.WebhookEvent, error) {
			if !bytes.Equal(gotPayload, payload) {
				t.Fatalf("verifier payload = %q, want %q", gotPayload, payload)
			}
			if headers.Get("Svix-Id") != providerEvent.ID {
				t.Fatalf("Svix-Id = %q", headers.Get("Svix-Id"))
			}
			return providerEvent, nil
		}),
		webhookInboxFunc(func(
			_ context.Context,
			event platformiam.WebhookEvent,
		) (platformiam.WebhookEvent, bool, error) {
			received = event
			return event, true, nil
		}),
	)

	response := performClerkWebhookRequestWithID(handler, payload, providerEvent.ID)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if received.Provider != "clerk" {
		t.Fatalf("provider = %q", received.Provider)
	}
	if received.ExternalID != providerEvent.ID {
		t.Fatalf("external id = %q", received.ExternalID)
	}
	if received.EventType != string(providerEvent.Type) {
		t.Fatalf("event type = %q", received.EventType)
	}
	if !bytes.Equal(received.Payload, payload) {
		t.Fatalf("stored payload = %q, want %q", received.Payload, payload)
	}
	if !received.OccurredAt.Equal(occurredAt) {
		t.Fatalf("occurred at = %s, want %s", received.OccurredAt, occurredAt)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

func TestClerkWebhookDuplicateReturnsNoContent(t *testing.T) {
	handler := clerkWebhookTestHandler(
		config.ClerkConfig{WebhookSecret: "whsec_test"},
		clerkWebhookVerifierFunc(func(
			[]byte,
			http.Header,
		) (clerkadapter.WebhookEvent, error) {
			return clerkadapter.WebhookEvent{
				ID:        "evt_duplicate",
				Type:      clerkadapter.WebhookOrganizationUpdated,
				Timestamp: time.Now().UTC(),
			}, nil
		}),
		webhookInboxFunc(func(
			_ context.Context,
			event platformiam.WebhookEvent,
		) (platformiam.WebhookEvent, bool, error) {
			return event, false, nil
		}),
	)

	response := performClerkWebhookRequest(handler, []byte(`{"type":"organization.updated"}`))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusNoContent, response.Body.String())
	}
}

func TestClerkWebhookReturnsServiceUnavailableWhenInboxFails(t *testing.T) {
	handler := clerkWebhookTestHandler(
		config.ClerkConfig{WebhookSecret: "whsec_test"},
		clerkWebhookVerifierFunc(func(
			[]byte,
			http.Header,
		) (clerkadapter.WebhookEvent, error) {
			return clerkadapter.WebhookEvent{
				ID:        "evt_failed",
				Type:      clerkadapter.WebhookSessionRevoked,
				Timestamp: time.Now().UTC(),
			}, nil
		}),
		webhookInboxFunc(func(
			context.Context,
			platformiam.WebhookEvent,
		) (platformiam.WebhookEvent, bool, error) {
			return platformiam.WebhookEvent{}, false, errors.New("database unavailable")
		}),
	)

	response := performClerkWebhookRequest(handler, []byte(`{"type":"session.revoked"}`))

	assertAPIError(t, response, http.StatusServiceUnavailable, "IAM_UNAVAILABLE")
}

func clerkWebhookTestHandler(
	clerk config.ClerkConfig,
	verifier ClerkWebhookVerifier,
	inbox WebhookInbox,
) http.Handler {
	return NewHandlerWithIAM(
		discardLogger(),
		nil,
		time.Second,
		NewIAMAPI(clerk, IAMDependencies{
			WebhookVerifier: verifier,
			WebhookInbox:    inbox,
		}),
	)
}

func performClerkWebhookRequest(
	handler http.Handler,
	payload []byte,
) *httptest.ResponseRecorder {
	return performClerkWebhookRequestWithID(handler, payload, "evt_test")
}

func performClerkWebhookRequestWithID(
	handler http.Handler,
	payload []byte,
	eventID string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		http.MethodPost,
		"/webhooks/clerk",
		bytes.NewReader(payload),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Svix-Id", eventID)
	request.Header.Set("Svix-Timestamp", "1784827800")
	request.Header.Set("Svix-Signature", "v1,test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
