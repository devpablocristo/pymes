// Package httperrors wrapper thin de core/backend/gin/go para Gin.
// Mantiene compatibilidad con los 87+ archivos que importan este paquete.
// Los sentinels, Respond y Write delegan a core.
package httperrors

import (
	"strings"

	"github.com/gin-gonic/gin"

	ginmw "github.com/devpablocristo/core/backend/gin/go"
	"github.com/devpablocristo/core/backend/go/domainerr"
)

// Sentinel errors — domainerr.Error, soportan errors.Is por Kind.
var (
	ErrNotFound  = domainerr.NotFound("not found")
	ErrConflict  = domainerr.Conflict("conflict")
	ErrForbidden = domainerr.Forbidden("forbidden")
	ErrBadInput  = domainerr.Validation("bad input")
	ErrNotDraft  = domainerr.Conflict("resource is not in draft status")
)

// ErrorResponse re-exporta el tipo de core para compatibilidad.
type ErrorResponse = ginmw.ErrorResponse

// IsUniqueViolation detecta errores de constraint UNIQUE de PostgreSQL.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "23505") ||
		strings.Contains(msg, "unique") ||
		strings.Contains(msg, "duplicate")
}

// Write escribe un error HTTP a Gin. Delega a ginmw.WriteError.
func Write(c *gin.Context, status int, code, message string) {
	ginmw.WriteError(c, status, code, message)
}

// Respond mapea un error a respuesta HTTP en Gin. Delega a ginmw.Respond.
func Respond(c *gin.Context, err error) {
	ginmw.Respond(c, err)
}
