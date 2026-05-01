package wire

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	kerneldomain "github.com/devpablocristo/core/saas/go/kernel/usecases/domain"
	saasmiddleware "github.com/devpablocristo/core/saas/go/middleware"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
)

type scopedAPIKeyVerifier struct{}

func (scopedAPIKeyVerifier) Verify(ctx context.Context, token string) (kerneldomain.Principal, error) {
	_ = ctx
	_ = token
	return kerneldomain.Principal{
		TenantID:   "00000000-0000-0000-0000-000000000001",
		Actor:      "api_key:key-1",
		Role:       "service",
		Scopes:     []string{"admin:console:read", "admin:console:write"},
		AuthMethod: "api_key",
	}, nil
}

func TestGinSaaSAuthMiddlewareCopiesPrincipalScopes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GinSaaSAuthMiddleware(&SaaSServices{
		AuthMiddleware: saasmiddleware.NewAuthMiddleware(nil, scopedAPIKeyVerifier{}),
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
		t.Fatalf("unexpected org id %q", auth.OrgID)
	}
	if auth.AuthMethod != "api_key" {
		t.Fatalf("unexpected auth method %q", auth.AuthMethod)
	}
	if len(auth.Scopes) != 1 || auth.Scopes[0] != "admin:console:read" {
		t.Fatalf("unexpected scopes %#v", auth.Scopes)
	}
}
