package handlers

import (
	"github.com/gin-gonic/gin"

	ginmw "github.com/devpablocristo/core/http/go/gin"
)

// AuthMiddleware re-exporta el tipo de core.
type AuthMiddleware = ginmw.AuthMiddleware

// AuthContext re-exporta el tipo de core.
type AuthContext = ginmw.AuthContext

// GetAuthContext delega a core.
func GetAuthContext(c *gin.Context) AuthContext {
	return ginmw.GetAuthContext(c)
}
