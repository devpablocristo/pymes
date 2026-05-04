package wire

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	saasmiddleware "github.com/devpablocristo/core/saas/go/shared/middleware"
	ctxkeys "github.com/devpablocristo/core/security/go/contextkeys"
)

// GinSaaSAuthMiddleware runs core/saas/go net/http auth and copies principal into Gin context
// using the same keys as the legacy handlers package (org_id, actor, role, scopes, auth_method).
func GinSaaSAuthMiddleware(svc *SaaSServices) gin.HandlerFunc {
	if svc == nil || svc.AuthMiddleware == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		nextCalled := false
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			c.Request = r
			copyPrincipalToGin(c, r.Context())
		})
		svc.AuthMiddleware(inner).ServeHTTP(c.Writer, c.Request)
		if !nextCalled {
			c.Abort()
		}
	}
}

func copyPrincipalToGin(c *gin.Context, reqCtx context.Context) {
	principal, ok := saasmiddleware.PrincipalFromContext(reqCtx)
	if !ok {
		return
	}
	authMethod := strings.TrimSpace(principal.AuthMethod)

	var orgIDStr string
	if id, err := uuid.Parse(strings.TrimSpace(principal.TenantID)); err == nil {
		orgIDStr = id.String()
		c.Set(ctxkeys.CtxKeyOrgID, orgIDStr)
	} else if strings.TrimSpace(principal.TenantID) != "" {
		orgIDStr = strings.TrimSpace(principal.TenantID)
		c.Set(ctxkeys.CtxKeyOrgID, orgIDStr)
	}

	if authMethod == "api_key" {
		if strings.TrimSpace(principal.Actor) != "" {
			c.Set(ctxkeys.CtxKeyActor, strings.TrimSpace(principal.Actor))
		} else if orgIDStr != "" {
			c.Set(ctxkeys.CtxKeyActor, "api_key:"+orgIDStr)
		}
		if strings.TrimSpace(principal.Role) != "" {
			c.Set(ctxkeys.CtxKeyRole, strings.TrimSpace(principal.Role))
		} else {
			c.Set(ctxkeys.CtxKeyRole, "service")
		}
	} else {
		if strings.TrimSpace(principal.Actor) != "" {
			c.Set(ctxkeys.CtxKeyActor, strings.TrimSpace(principal.Actor))
		}
		if strings.TrimSpace(principal.Role) != "" {
			c.Set(ctxkeys.CtxKeyRole, strings.TrimSpace(principal.Role))
		}
	}

	scopes := append([]string(nil), principal.Scopes...)
	if authMethod == "api_key" {
		reqScopes := splitScopesCSV(c.GetHeader("X-Scopes"))
		if len(reqScopes) > 0 {
			scopes = intersectScopes(scopes, reqScopes)
		}
	}
	c.Set(ctxkeys.CtxKeyScopes, scopes)
	c.Set(ctxkeys.CtxKeyAuthMethod, authMethod)
}

func splitScopesCSV(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	res := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		res = append(res, p)
	}
	return res
}

func intersectScopes(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(a))
	for _, s := range a {
		m[s] = struct{}{}
	}
	res := make([]string, 0)
	for _, s := range b {
		if _, ok := m[s]; ok {
			res = append(res, s)
		}
	}
	return res
}
