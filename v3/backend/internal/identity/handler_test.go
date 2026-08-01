package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
)

type verifierFunc func([]byte, http.Header) (clerk.WebhookEvent, error)

func (f verifierFunc) VerifyAndDecode(p []byte, h http.Header) (clerk.WebhookEvent, error) {
	return f(p, h)
}

type inboxFunc func(context.Context, Event) (bool, error)

func (f inboxFunc) Receive(c context.Context, e Event) (bool, error) { return f(c, e) }

type sessionAuthenticatorFunc func(*http.Request) (identitydomain.Principal, error)

func (f sessionAuthenticatorFunc) Principal(r *http.Request) (identitydomain.Principal, error) {
	return f(r)
}

func TestSessionReturnsCanonicalLocalOrganization(t *testing.T) {
	t.Parallel()
	handler := NewSessionHandler(sessionAuthenticatorFunc(func(request *http.Request) (identitydomain.Principal, error) {
		if request.Header.Get("Authorization") != "Bearer verified-clerk-session" {
			t.Fatalf("authorization header was not forwarded")
		}
		return identitydomain.Principal{
			OrganizationID:     "org_local",
			OrganizationName:   "Centro Norte",
			OrganizationSlug:   "centro-norte",
			OrganizationStatus: "ready",
			ActorID:            "user_clerk",
			Role:               identitydomain.RoleAdmin,
			Permissions:        []string{"scheduling:read", "scheduling:operate"},
			MembershipStatus:   "active",
		}, nil
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	request.Header.Set("Authorization", "Bearer verified-clerk-session")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unsafe response headers: %v", response.Header())
	}
	for _, expected := range []string{
		`"actor_id":"user_clerk"`,
		`"id":"org_local"`,
		`"name":"Centro Norte"`,
		`"slug":"centro-norte"`,
		`"role":"admin"`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("response does not contain %s: %s", expected, response.Body.String())
		}
	}
	if strings.Contains(response.Body.String(), "org_clerk") {
		t.Fatalf("provider organization leaked into canonical session: %s", response.Body.String())
	}
}

func TestSessionFailsClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		handler *SessionHandler
		status  int
		code    string
	}{
		{
			name:    "missing authenticator",
			handler: NewSessionHandler(nil),
			status:  http.StatusServiceUnavailable,
			code:    "AUTH_NOT_CONFIGURED",
		},
		{
			name: "invalid membership",
			handler: NewSessionHandler(sessionAuthenticatorFunc(func(*http.Request) (identitydomain.Principal, error) {
				return identitydomain.Principal{}, errors.New("unknown local membership")
			})),
			status: http.StatusForbidden,
			code:   "FORBIDDEN",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			response := httptest.NewRecorder()
			test.handler.ServeHTTP(
				response,
				httptest.NewRequest(http.MethodGet, "/api/v1/session", nil),
			)
			if response.Code != test.status ||
				!strings.Contains(response.Body.String(), test.code) {
				t.Fatalf(
					"status=%d body=%s",
					response.Code,
					response.Body.String(),
				)
			}
		})
	}
}

func TestSessionEncodesMissingPermissionsAsAnEmptyArray(t *testing.T) {
	t.Parallel()
	handler := NewSessionHandler(sessionAuthenticatorFunc(func(*http.Request) (identitydomain.Principal, error) {
		return identitydomain.Principal{
			OrganizationID:     "org_local",
			OrganizationName:   "Centro Norte",
			OrganizationSlug:   "centro-norte",
			OrganizationStatus: "ready",
			ActorID:            "user_readless",
			Role:               identitydomain.RoleMember,
			MembershipStatus:   "active",
		}, nil
	}))
	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/session", nil),
	)

	if response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), `"permissions":[]`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	h := NewWebhook(verifierFunc(func([]byte, http.Header) (clerk.WebhookEvent, error) {
		return clerk.WebhookEvent{}, clerk.ErrInvalidWebhookSignature
	}), ReceiveWebhook{})
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", w.Code)
	}
}
func TestWebhookStoresVerifiedEvent(t *testing.T) {
	stored := false
	h := NewWebhook(verifierFunc(func([]byte, http.Header) (clerk.WebhookEvent, error) {
		return clerk.WebhookEvent{ID: "evt", Type: clerk.WebhookOrganizationCreated, Timestamp: time.Now()}, nil
	}), ReceiveWebhook{Inbox: inboxFunc(func(_ context.Context, e Event) (bool, error) { stored = e.ID == "evt"; return false, nil })})
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"object":"event"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent || !stored {
		t.Fatalf("status=%d stored=%v", w.Code, stored)
	}
}
func TestWebhookUnavailableInbox(t *testing.T) {
	h := NewWebhook(verifierFunc(func([]byte, http.Header) (clerk.WebhookEvent, error) {
		return clerk.WebhookEvent{ID: "evt", Timestamp: time.Now()}, nil
	}), ReceiveWebhook{Inbox: inboxFunc(func(context.Context, Event) (bool, error) { return false, errors.New("db") })})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}")))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", w.Code)
	}
}
