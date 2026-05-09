package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ctxkeys "github.com/devpablocristo/core/security/go/contextkeys"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubAPIKeyResolver struct {
	key ResolvedKey
	ok  bool
}

func (s stubAPIKeyResolver) ResolveAPIKey(string) (ResolvedKey, bool) {
	return s.key, s.ok
}

type stubOrgRefResolver struct {
	orgID string
	err      error
}

func (s stubOrgRefResolver) ResolveOrgID(context.Context, string) (string, error) {
	return s.orgID, s.err
}

func TestRequireAuthAPIKeyUsesServiceIdentity(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	keyID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000020")
	middleware := NewAuthMiddleware(nil, stubAPIKeyResolver{
		ok: true,
		key: ResolvedKey{
			ID:       keyID,
			OrgID: orgID,
			Scopes:   []string{"customers:read", "customers:write"},
		},
	}, false, true)

	router := gin.New()
	router.Use(middleware.RequireAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, GetAuthContext(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-API-KEY", "psk_test")
	req.Header.Set("X-Actor", "spoofed-user")
	req.Header.Set("X-Role", "admin")
	req.Header.Set("X-Scopes", "customers:read,unknown:scope")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var got AuthContext
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got.Actor != "api_key:"+keyID.String() {
		t.Fatalf("expected actor derived from key id, got %q", got.Actor)
	}
	if got.Role != "service" {
		t.Fatalf("expected service role, got %q", got.Role)
	}
	if got.OrgID != orgID.String() {
		t.Fatalf("expected org %q, got %q", orgID.String(), got.OrgID)
	}
	if got.AuthMethod != "api_key" {
		t.Fatalf("expected auth method api_key, got %q", got.AuthMethod)
	}
	if len(got.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %#v", got.Scopes)
	}
}

func TestRequireTenantSlugBindingRequiresHeaderForJWT(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyOrgID, "tenant-1")
		c.Set(ctxkeys.CtxKeyAuthMethod, "jwt")
		c.Next()
	})
	router.Use(RequireTenantSlugBinding(stubOrgRefResolver{orgID: "tenant-1"}))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "tenant_slug_required") {
		t.Fatalf("expected tenant_slug_required body, got %s", recorder.Body.String())
	}
}

func TestRequireTenantSlugBindingRejectsMismatch(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyOrgID, "tenant-1")
		c.Set(ctxkeys.CtxKeyAuthMethod, "jwt")
		c.Next()
	})
	router.Use(RequireTenantSlugBinding(stubOrgRefResolver{orgID: "tenant-2"}))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(TenantSlugHeader, "medlab")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "tenant_mismatch") {
		t.Fatalf("expected tenant_mismatch body, got %s", recorder.Body.String())
	}
}

func TestRequireTenantSlugBindingAllowsMatchingSlug(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyOrgID, "tenant-1")
		c.Set(ctxkeys.CtxKeyAuthMethod, "jwt")
		c.Next()
	})
	router.Use(RequireTenantSlugBinding(stubOrgRefResolver{orgID: "tenant-1"}))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(TenantSlugHeader, "bicimax")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

// TestNewInternalServiceAuth fue eliminado junto con internal_service_auth.go.
// La autenticación interna ahora usa core/security/go/apikey.
