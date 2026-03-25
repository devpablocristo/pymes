package verticalgin

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	types "github.com/devpablocristo/core/security/go/contextkeys"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalconfig"
)

// DevForceOrgMiddleware reemplaza org_id en el contexto (solo ambientes locales) para alinear JWT/Clerk con datos seed.
func DevForceOrgMiddleware(environment, forceOrgUUID string) gin.HandlerFunc {
	force := strings.TrimSpace(forceOrgUUID)
	if force == "" || !verticalconfig.IsLocalEnvironment(environment) {
		return func(c *gin.Context) { c.Next() }
	}
	if _, err := uuid.Parse(force); err != nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		c.Set(types.CtxKeyOrgID, force)
		c.Next()
	}
}
