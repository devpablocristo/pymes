package wire

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	ctxkeys "github.com/devpablocristo/core/backend/go/contextkeys"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalconfig"
)

// NewGinDevForceOrgMiddleware reemplaza org_id en Gin tras SaaS auth (solo local) para alinear JWT/Clerk con seeds demo.
func NewGinDevForceOrgMiddleware(environment, forceOrgUUID string) gin.HandlerFunc {
	force := strings.TrimSpace(forceOrgUUID)
	if force == "" || !verticalconfig.IsLocalEnvironment(environment) {
		return func(c *gin.Context) { c.Next() }
	}
	if _, err := uuid.Parse(force); err != nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		c.Set(ctxkeys.CtxKeyOrgID, force)
		c.Next()
	}
}
