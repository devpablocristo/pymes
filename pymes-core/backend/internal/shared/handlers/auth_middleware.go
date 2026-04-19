package handlers

import (
	"github.com/gin-gonic/gin"

	sharedauth "github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
)

// AuthMiddleware re-exports the shared auth middleware type.
type AuthMiddleware = sharedauth.AuthMiddleware

// AuthContext re-exports the shared auth context type.
type AuthContext = sharedauth.AuthContext

// GetAuthContext delegates to the shared auth package.
func GetAuthContext(c *gin.Context) AuthContext {
	return sharedauth.GetAuthContext(c)
}
