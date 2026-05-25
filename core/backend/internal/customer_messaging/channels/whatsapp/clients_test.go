package whatsapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	cm "github.com/devpablocristo/pymes/core/backend/internal/customer_messaging"
)

func TestCompanionClientProcessWhatsAppUsesPublicContractAndInternalJWT(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	var claims map[string]any
	var body CustomerMessagingInboundRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/customer-messaging/inbound" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			t.Fatal("expected internal JWT bearer")
		}
		if got := r.Header.Get("X-API-Key"); got != "" {
			t.Fatalf("did not expect API key when internal JWT is configured, got %q", got)
		}
		claims = decodeClaims(t, token)
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"conversation_id":"conv-1","reply":"ok","tokens_used":3,"tool_calls":[]}`))
	}))
	t.Cleanup(server.Close)

	client := NewCompanionClientWithConfig(CompanionConfig{
		BaseURL:             server.URL,
		APIKey:              "dev-key",
		InternalJWTSecret:   "secret",
		InternalJWTIssuer:   "axis-bff",
		InternalJWTAudience: "companion",
		Now: func() time.Time {
			return time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
		},
	})
	out, err := client.ProcessWhatsApp(context.Background(), cm.InboundMessage{
		OrgID:         orgID,
		PhoneNumberID: "phone-1",
		FromPhone:     "5491112345678",
		Text:          "hola",
		MessageID:     "wamid-1",
		ProfileName:   "Juan",
	})
	if err != nil {
		t.Fatalf("ProcessWhatsApp() error = %v", err)
	}
	if out.ConversationID != "conv-1" || out.Reply != "ok" {
		t.Fatalf("unexpected response %+v", out)
	}
	if body.OrgID != orgID.String() || body.PhoneNumberID != "phone-1" || body.Message != "hola" {
		t.Fatalf("unexpected forwarded body %+v", body)
	}
	if claims["org_id"] != orgID.String() || claims["product_surface"] != "pymes" || claims["sub"] != "pymes-whatsapp-bridge" {
		t.Fatalf("unexpected claims %+v", claims)
	}
}

func TestCompanionClientProcessWhatsAppFallsBackToAPIKeyWhenJWTNotConfigured(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "companion-key" {
			t.Fatalf("expected api key fallback, got %q", r.Header.Get("X-API-Key"))
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("did not expect bearer without JWT secret")
		}
		_, _ = w.Write([]byte(`{"conversation_id":"conv-1","reply":"ok","tokens_used":0,"tool_calls":[]}`))
	}))
	t.Cleanup(server.Close)

	client := NewCompanionClient(server.URL, "companion-key")
	_, err := client.ProcessWhatsApp(context.Background(), cm.InboundMessage{
		OrgID:         uuid.New(),
		PhoneNumberID: "phone-1",
		FromPhone:     "5491112345678",
		Text:          "hola",
	})
	if err != nil {
		t.Fatalf("ProcessWhatsApp() error = %v", err)
	}
}

func TestCompanionClientProcessWhatsAppPropagatesServiceErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"code":"internal","message":"bad"}`, http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := NewCompanionClientWithConfig(CompanionConfig{BaseURL: server.URL, InternalJWTSecret: "secret"})
	_, err := client.ProcessWhatsApp(context.Background(), cm.InboundMessage{
		OrgID:         uuid.New(),
		PhoneNumberID: "phone-1",
		FromPhone:     "5491112345678",
		Text:          "hola",
	})
	if err == nil || !strings.Contains(err.Error(), "companion service returned 500") {
		t.Fatalf("expected service error, got %v", err)
	}
}

func TestCompanionClientProcessWhatsAppRejectsPartialResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"reply":"ok","tokens_used":0,"tool_calls":[]}`))
	}))
	t.Cleanup(server.Close)

	client := NewCompanionClientWithConfig(CompanionConfig{BaseURL: server.URL, InternalJWTSecret: "secret"})
	_, err := client.ProcessWhatsApp(context.Background(), cm.InboundMessage{
		OrgID:         uuid.New(),
		PhoneNumberID: "phone-1",
		FromPhone:     "5491112345678",
		Text:          "hola",
	})
	if err == nil || !strings.Contains(err.Error(), "conversation_id is required") {
		t.Fatalf("expected partial response error, got %v", err)
	}
}

func TestCompanionClientProcessWhatsAppHonorsTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"conversation_id":"conv-1","reply":"ok","tokens_used":0,"tool_calls":[]}`))
	}))
	t.Cleanup(server.Close)

	client := NewCompanionClientWithConfig(CompanionConfig{
		BaseURL:           server.URL,
		InternalJWTSecret: "secret",
		HTTP:              &http.Client{Timeout: time.Millisecond},
	})
	_, err := client.ProcessWhatsApp(context.Background(), cm.InboundMessage{
		OrgID:         uuid.New(),
		PhoneNumberID: "phone-1",
		FromPhone:     "5491112345678",
		Text:          "hola",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func decodeClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("invalid jwt %q", token)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	return out
}
