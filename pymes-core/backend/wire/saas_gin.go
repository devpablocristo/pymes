package wire

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/core/saas/go/shared/ctxkeys"
	pymestypes "github.com/devpablocristo/pymes/pkgs/go-pkg/types"
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
	authMethod, _ := reqCtx.Value(ctxkeys.AuthMethod).(string)

	var orgIDStr string
	if v := reqCtx.Value(ctxkeys.OrgID); v != nil {
		if id, ok := v.(uuid.UUID); ok {
			orgIDStr = id.String()
			c.Set(pymestypes.CtxKeyOrgID, orgIDStr)
		}
	}

	if authMethod == "api_key" {
		c.Set(pymestypes.CtxKeyActor, "api_key:"+orgIDStr)
		c.Set(pymestypes.CtxKeyRole, "service")
	} else {
		if v := reqCtx.Value(ctxkeys.Actor); v != nil {
			if s, ok := v.(string); ok && s != "" {
				c.Set(pymestypes.CtxKeyActor, s)
			}
		}
		if v := reqCtx.Value(ctxkeys.Role); v != nil {
			if s, ok := v.(string); ok && s != "" {
				c.Set(pymestypes.CtxKeyRole, s)
			}
		}
	}

	var scopes []string
	if v := reqCtx.Value(ctxkeys.Scopes); v != nil {
		scopes, _ = v.([]string)
	}
	if authMethod == "api_key" {
		reqScopes := splitScopesCSV(c.GetHeader("X-Scopes"))
		if len(reqScopes) > 0 {
			scopes = intersectScopes(scopes, reqScopes)
		}
	}
	c.Set(pymestypes.CtxKeyScopes, scopes)
	c.Set(pymestypes.CtxKeyAuthMethod, authMethod)
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
