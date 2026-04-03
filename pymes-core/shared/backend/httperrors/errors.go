// Package httperrors wrapper thin de core/http/go/gin para Gin.
// Mantiene compatibilidad con los 87+ archivos que importan este paquete.
// Los sentinels, Respond y Write delegan a core.
package httperrors

import (
	corepostgres "github.com/devpablocristo/core/databases/postgres/go"
	"github.com/gin-gonic/gin"

	ginmw "github.com/devpablocristo/core/http/gin/go"
	"github.com/devpablocristo/core/errors/go/domainerr"
)

// Sentinel errors — domainerr.Error, soportan errors.Is por Kind.
var (
	ErrNotFound  = ginmw.ErrNotFound
	ErrConflict  = ginmw.ErrConflict
	ErrForbidden = ginmw.ErrForbidden
	ErrBadInput  = ginmw.ErrBadInput
	ErrNotDraft  = domainerr.Conflict("resource is not in draft status")
)

// ErrorResponse re-exporta el tipo de core para compatibilidad.
type ErrorResponse = ginmw.ErrorResponse

// IsUniqueViolation detecta errores de constraint UNIQUE de PostgreSQL.
func IsUniqueViolation(err error) bool {
	return corepostgres.IsUniqueViolation(err)
}

// Write escribe un error HTTP a Gin. Delega a ginmw.WriteError.
func Write(c *gin.Context, status int, code, message string) {
	ginmw.WriteError(c, status, code, message)
}

// Respond mapea un error a respuesta HTTP en Gin. Delega a ginmw.Respond.
func Respond(c *gin.Context, err error) {
	ginmw.Respond(c, err)
}
