package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	types "github.com/devpablocristo/core/security/go/contextkeys"
)

type fakeChecker struct {
	allow bool
}

func (f fakeChecker) HasPermission(ctx context.Context, orgID, actor, role string, scopes []string, authMethod, resource, action string) bool {
	_ = ctx
	_ = orgID
	_ = actor
	_ = role
	_ = scopes
	_ = authMethod
	_ = resource
	_ = action
	return f.allow
}

func TestRBACMiddleware_RequirePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		allow      bool
		wantStatus int
	}{
		{name: "allowed", allow: true, wantStatus: http.StatusOK},
		{name: "forbidden", allow: false, wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			m := NewRBACMiddleware(fakeChecker{allow: tt.allow})

			r.Use(func(c *gin.Context) {
				c.Set(types.CtxKeyOrgID, "00000000-0000-0000-0000-000000000001")
				c.Set(types.CtxKeyActor, "local-admin")
				c.Set(types.CtxKeyRole, "member")
				c.Set(types.CtxKeyScopes, []string{})
				c.Set(types.CtxKeyAuthMethod, "jwt")
				c.Next()
			})
			r.GET("/test", m.RequirePermission("sales", "create"), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)

			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.Code, tt.wantStatus)
			}
		})
	}
}
