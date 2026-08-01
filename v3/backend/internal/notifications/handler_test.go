package notifications

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pergohelpers "github.com/devpablocristo/pymes/v3/backend/internal/notifications/pergo/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type actorAuth struct {
	actor Actor
	err   error
}

func (auth actorAuth) Authenticate(*http.Request) (Actor, error) {
	return auth.actor, auth.err
}

type notificationReader struct{ intent domain.Intent }

func (reader notificationReader) Execute(
	context.Context, string, string,
) (domain.Intent, error) {
	return reader.intent, nil
}

type webhookProcessor struct {
	calls int
}

func (processor *webhookProcessor) Execute(
	_ context.Context,
	_ string,
	_ domain.DeliveryEvent,
	_ string,
) (bool, error) {
	processor.calls++
	return false, nil
}

func TestHandlerNeverExposesRecipientBodyOrVariables(t *testing.T) {
	handler := Handler{
		Auth: actorAuth{actor: Actor{
			OrganizationID: "org-1", ActorID: "user-1",
			Role: "member", MembershipStatus: "active",
		}},
		Reader: notificationReader{domain.Intent{
			ID: "notification-1", OrganizationID: "org-1",
			Kind: domain.KindReminder, AggregateType: "booking",
			AggregateID: "booking-1", RecipientE164: "+5491112345678",
			Body: "sensitive body", Variables: map[string]string{"name": "Pablo"},
			Status: domain.StatusSent,
		}},
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/organizations/org-1/notifications/notification-1",
		nil,
	)
	response := httptest.NewRecorder()
	handler.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body)
	}
	for _, secret := range []string{"+5491112345678", "sensitive body", "Pablo"} {
		if bytes.Contains(response.Body.Bytes(), []byte(secret)) {
			t.Fatalf("sensitive value %q leaked in response", secret)
		}
	}
}

func TestHandlerFailsClosedWhenClerkSessionIsInvalid(t *testing.T) {
	handler := Handler{
		Auth:   actorAuth{err: fmt.Errorf("invalid session")},
		Reader: notificationReader{},
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/organizations/org-1/notifications/notification-1",
		nil,
	)
	response := httptest.NewRecorder()
	handler.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden ||
		!bytes.Contains(response.Body.Bytes(), []byte(`"FORBIDDEN"`)) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
}

func TestPerGoWebhookRejectsInvalidAndAcceptsValidSignature(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	secret := []byte("0123456789abcdef0123456789abcdef")
	processor := &webhookProcessor{}
	handler := Handler{
		Webhooks: processor,
		Verifier: PerGoSignatureVerifier{
			Secrets: [][]byte{secret}, Clock: func() time.Time { return now },
		},
	}
	traceID, err := pergohelpers.TraceID("org-1", "notification-1")
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(fmt.Sprintf(
		`{"event":"message.sent","trace_id":%q,"message_id":"external-1","channel":"whatsapp","timestamp":"2023-11-14T22:13:20Z","workspace_id":"workspace-1"}`,
		traceID,
	))
	request := httptest.NewRequest(
		http.MethodPost, "/api/v1/webhooks/pergo",
		bytes.NewReader(body),
	)
	request.Header.Set("X-PerGo-Signature", "t=1,v1=bad")
	response := httptest.NewRecorder()
	handler.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || processor.calls != 0 {
		t.Fatalf("invalid signature status=%d calls=%d", response.Code, processor.calls)
	}

	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(fmt.Sprintf("%d.", now.Unix())))
	_, _ = mac.Write(body)
	request = httptest.NewRequest(
		http.MethodPost, "/api/v1/webhooks/pergo",
		bytes.NewReader(body),
	)
	request.Header.Set(
		"X-PerGo-Signature",
		fmt.Sprintf("t=%d,v1=%s", now.Unix(), hex.EncodeToString(mac.Sum(nil))),
	)
	response = httptest.NewRecorder()
	handler.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || processor.calls != 1 {
		t.Fatalf("valid signature status=%d calls=%d body=%s", response.Code, processor.calls, response.Body)
	}
}
