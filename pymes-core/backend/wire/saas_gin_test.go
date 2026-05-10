package wire

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	ginmw "github.com/devpablocristo/core/http/gin/go"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	sharedauth "github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
)

type scopedAPIKeyVerifier struct{}

func (scopedAPIKeyVerifier) Verify(ctx context.Context, token string) (tenantPrincipal, error) {
	_ = ctx
	_ = token
	return tenantPrincipal{
		OrgID:   "00000000-0000-0000-0000-000000000001",
		Actor:      "api_key:key-1",
		Role:       "service",
		Scopes:     []string{"admin:console:read", "admin:console:write"},
		AuthMethod: "api_key",
	}, nil
}

type scopedJWTVerifier struct{}

func (scopedJWTVerifier) Verify(ctx context.Context, token string) (tenantPrincipal, error) {
	_ = ctx
	_ = token
	return tenantPrincipal{
		OrgID:   "00000000-0000-0000-0000-000000000001",
		Actor:      "user-1",
		Role:       "member",
		Scopes:     []string{"admin:console:read"},
		AuthMethod: "jwt",
	}, nil
}

func testTenantResolver(_ context.Context, ref string) (uuid.UUID, bool, error) {
	switch ref {
	case "bicimax", "00000000-0000-0000-0000-000000000001":
		return uuid.MustParse("00000000-0000-0000-0000-000000000001"), true, nil
	case "medlab":
		return uuid.MustParse("00000000-0000-0000-0000-000000000002"), true, nil
	default:
		return uuid.Nil, false, nil
	}
}

func testTenantMembershipResolver(_ context.Context, orgID uuid.UUID, actor string) (string, bool, error) {
	if actor == "user-1" && orgID == uuid.MustParse("00000000-0000-0000-0000-000000000002") {
		return "owner", true, nil
	}
	return "", false, nil
}

func TestGinSaaSAuthMiddlewareCopiesPrincipalScopes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GinSaaSAuthMiddleware(&SaaSServices{
		AuthMiddleware: newTenantAuthMiddleware(nil, scopedAPIKeyVerifier{}),
	}))
	router.GET("/v1/admin/tenant-settings", func(c *gin.Context) {
		c.JSON(http.StatusOK, handlers.GetAuthContext(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/tenant-settings", nil)
	req.Header.Set("X-API-KEY", "psk_local_admin")
	req.Header.Set("X-Scopes", "admin:console:read")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}

	var auth handlers.AuthContext
	if err := json.Unmarshal(rec.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}
	if auth.OrgID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("unexpected tenant id %q", auth.OrgID)
	}
	if auth.AuthMethod != "api_key" {
		t.Fatalf("unexpected auth method %q", auth.AuthMethod)
	}
	if len(auth.Scopes) != 1 || auth.Scopes[0] != "admin:console:read" {
		t.Fatalf("unexpected scopes %#v", auth.Scopes)
	}
}

func TestGinSaaSAuthMiddlewareCopiesTenantIntoCoreOrgContext(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GinSaaSAuthMiddleware(&SaaSServices{
		AuthMiddleware: newTenantAuthMiddleware(nil, scopedAPIKeyVerifier{}),
	}))
	router.GET("/v1/scheduling/day", func(c *gin.Context) {
		c.JSON(http.StatusOK, ginmw.GetAuthContext(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/scheduling/day", nil)
	req.Header.Set("X-API-KEY", "psk_local_admin")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}

	var auth ginmw.AuthContext
	if err := json.Unmarshal(rec.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}
	if auth.OrgID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("unexpected core tenant id %q", auth.OrgID)
	}
	if auth.AuthMethod != "api_key" {
		t.Fatalf("unexpected auth method %q", auth.AuthMethod)
	}
}

func TestTenantSlugBindingRequiresHeaderForJWT(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GinSaaSAuthMiddleware(&SaaSServices{
		AuthMiddleware: withTenantSlugBinding(newTenantAuthMiddleware(scopedJWTVerifier{}, nil), testTenantResolver, nil),
	}))
	router.GET("/v1/invoices", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/invoices", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "tenant_slug_required") {
		t.Fatalf("expected tenant_slug_required body, got %s", rec.Body.String())
	}
}

func TestTenantSlugBindingRejectsSlugMismatch(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GinSaaSAuthMiddleware(&SaaSServices{
		AuthMiddleware: withTenantSlugBinding(newTenantAuthMiddleware(scopedJWTVerifier{}, nil), testTenantResolver, nil),
	}))
	router.GET("/v1/invoices", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/invoices", nil)
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set(sharedauth.TenantSlugHeader, "medlab")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "tenant_mismatch") {
		t.Fatalf("expected tenant_mismatch body, got %s", rec.Body.String())
	}
}

func TestTenantSlugBindingRejectsRequestedTenantWhenActiveClerkOrgDiffers(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GinSaaSAuthMiddleware(&SaaSServices{
		AuthMiddleware: withTenantSlugBinding(newTenantAuthMiddleware(scopedJWTVerifier{}, nil), testTenantResolver, testTenantMembershipResolver),
	}))
	router.GET("/v1/session", func(c *gin.Context) {
		c.JSON(http.StatusOK, handlers.GetAuthContext(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/session", nil)
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set(sharedauth.TenantSlugHeader, "medlab")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "tenant_mismatch") {
		t.Fatalf("expected tenant_mismatch body, got %s", rec.Body.String())
	}
}

func TestTenantSlugBindingAllowsMatchingSlug(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GinSaaSAuthMiddleware(&SaaSServices{
		AuthMiddleware: withTenantSlugBinding(newTenantAuthMiddleware(scopedJWTVerifier{}, nil), testTenantResolver, nil),
	}))
	router.GET("/v1/invoices", func(c *gin.Context) {
		c.JSON(http.StatusOK, handlers.GetAuthContext(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/invoices", nil)
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set(sharedauth.TenantSlugHeader, "bicimax")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
