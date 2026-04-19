package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type PermissionChecker interface {
	HasPermission(ctx context.Context, orgID, actor, role string, scopes []string, authMethod, resource, action string) bool
}

type RBACMiddleware struct {
	checker PermissionChecker
}

func NewRBACMiddleware(checker PermissionChecker) *RBACMiddleware {
	return &RBACMiddleware{checker: checker}
}

func (m *RBACMiddleware) RequirePermission(resource, action string) gin.HandlerFunc {
	resource = strings.TrimSpace(resource)
	action = strings.TrimSpace(action)
	return func(c *gin.Context) {
		if m == nil || m.checker == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "rbac_not_configured"})
			return
		}
		authCtx := GetAuthContext(c)
		if authCtx.OrgID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if m.checker.HasPermission(c.Request.Context(), authCtx.OrgID, authCtx.Actor, authCtx.Role, authCtx.Scopes, authCtx.AuthMethod, resource, action) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":    "forbidden",
			"required": resource + ":" + action,
		})
	}
}
