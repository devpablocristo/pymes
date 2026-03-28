package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	authn "github.com/devpablocristo/core/authn/go"
	ginmw "github.com/devpablocristo/core/http/gin/go"
)

// stubAuthenticator implementa authn.Authenticator para tests.
type stubAuthenticator struct {
	principal *authn.Principal
	err       error
}

func (s stubAuthenticator) Authenticate(_ context.Context, _ authn.Credential) (*authn.Principal, error) {
	return s.principal, s.err
}

func TestRequireAuthAPIKeyUsesServiceIdentity(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	middleware := ginmw.NewAuthMiddleware(nil, &stubAuthenticator{
		principal: &authn.Principal{
			OrgID:      "00000000-0000-0000-0000-000000000120",
			Actor:      "api_key:00000000-0000-0000-0000-000000000110",
			Role:       "service",
			Scopes:     []string{"customers:read", "customers:write"},
			AuthMethod: "api_key",
		},
	})

	router := gin.New()
	router.Use(middleware.RequireAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, GetAuthContext(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-API-KEY", "psk_test")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var got AuthContext
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got.Actor != "api_key:00000000-0000-0000-0000-000000000110" {
		t.Fatalf("expected actor derived from key id, got %q", got.Actor)
	}
	if got.Role != "service" {
		t.Fatalf("expected service role, got %q", got.Role)
	}
	if got.OrgID != "00000000-0000-0000-0000-000000000120" {
		t.Fatalf("expected org, got %q", got.OrgID)
	}
	if got.AuthMethod != "api_key" {
		t.Fatalf("expected auth method api_key, got %q", got.AuthMethod)
	}
}
