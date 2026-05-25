package assistantproxy

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ctxkeys "github.com/devpablocristo/platform/security/go/contextkeys"
	"github.com/gin-gonic/gin"
)

func TestChatForwardsOnlyCompanionContractWithInternalJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var forwarded map[string]any
	var claims map[string]any
	companion := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&forwarded); err != nil {
			t.Fatalf("decode forwarded body: %v", err)
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			t.Fatal("expected internal jwt bearer")
		}
		claims = decodeJWTClaims(t, token)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chat_id":"00000000-0000-0000-0000-000000000011","task_id":"00000000-0000-0000-0000-000000000022","reply":"ok","task":{},"messages":[]}`))
	}))
	t.Cleanup(companion.Close)

	router := authenticatedRouter()
	NewHandler(NewClient(Config{
		BaseURL:             companion.URL,
		InternalJWTSecret:   "secret",
		InternalJWTIssuer:   "axis-bff",
		InternalJWTAudience: "companion",
		Now: func() time.Time {
			return time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
		},
	})).RegisterRoutes(router.Group("/v1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/chat", strings.NewReader(`{
		"chat_id":"00000000-0000-0000-0000-000000000011",
		"message":"hola",
		"route_hint":"sales",
		"preferred_language":"es",
		"confirmed_actions":["a-1"]
	}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := forwarded["route_hint"]; ok {
		t.Fatalf("route_hint must not be forwarded to Companion: %+v", forwarded)
	}
	if _, ok := forwarded["preferred_language"]; ok {
		t.Fatalf("preferred_language must not be forwarded to Companion: %+v", forwarded)
	}
	if forwarded["product_surface"] != productSurface {
		t.Fatalf("expected product_surface %q, got %+v", productSurface, forwarded["product_surface"])
	}
	if claims["org_id"] != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("tenant claim mismatch: %+v", claims)
	}
	if claims["actor_id"] != "owner@example.com" || claims["on_behalf_of"] != "owner@example.com" {
		t.Fatalf("actor claim mismatch: %+v", claims)
	}
	if claims["product_surface"] != productSurface {
		t.Fatalf("surface claim mismatch: %+v", claims)
	}
}

func TestProxyUsesAPIKeyWhenInternalJWTIsNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	companion := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "companion-key" {
			t.Fatalf("expected companion api key, got %q", r.Header.Get("X-API-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	t.Cleanup(companion.Close)

	router := authenticatedRouter()
	NewHandler(NewClient(Config{BaseURL: companion.URL, APIKey: "companion-key"})).RegisterRoutes(router.Group("/v1"))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/ai/chat/conversations?limit=30", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func authenticatedRouter() *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyTenantID, "00000000-0000-0000-0000-000000000001")
		c.Set(ctxkeys.CtxKeyActor, "owner@example.com")
		c.Set(ctxkeys.CtxKeyRole, "owner")
		c.Set(ctxkeys.CtxKeyAuthMethod, "jwt")
		c.Next()
	})
	return router
}

func decodeJWTClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("invalid jwt parts: %q", token)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	return claims
}
