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
)

type verifierFunc func([]byte, http.Header) (clerk.WebhookEvent, error)

func (f verifierFunc) VerifyAndDecode(p []byte, h http.Header) (clerk.WebhookEvent, error) {
	return f(p, h)
}

type inboxFunc func(context.Context, Event) (bool, error)

func (f inboxFunc) Receive(c context.Context, e Event) (bool, error) { return f(c, e) }
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
