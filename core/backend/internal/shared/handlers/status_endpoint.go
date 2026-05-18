package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/core/errors/go/domainerr"
	httperrors "github.com/devpablocristo/pymes/core/shared/backend/httperrors"
)

// StatusUpdater es la firma de un usecase que cambia el status de un recurso.
//
// El usecase es responsable de:
//   - leer el estado actual,
//   - validar la transición vía FSM (y mapear errores con
//     internal/shared/status.MapFSMError),
//   - persistir,
//   - emitir audit/timeline/webhook.
//
// Devuelve el dominio puro (no DTO).
type StatusUpdater[T any] func(ctx context.Context, orgID, id uuid.UUID, nextStatus, actor string) (T, error)

// StatusResponseMapper convierte el dominio al DTO de respuesta JSON.
type StatusResponseMapper[T any] func(T) any

// statusRequest es el shape uniforme aceptado por todos los endpoints /status.
type statusRequest struct {
	Status string `json:"status" binding:"required"`
}

// RegisterStatusEndpoint registra `PATCH <basePath>/:id/status` con parsing
// uniforme de tenant + id + body, RBAC, normalización del status (lower+trim),
// invocación del usecase y mapeo HTTP del error.
//
// `resource`/`permission` configuran el RBAC (ej. "sales", "update").
// `basePath` es el prefijo del recurso (ej. "/sales").
//
// El handler NO conoce la FSM ni los side effects. El usecase ya devuelve el
// error mapeado a `domainerr` (ej. via `status.MapFSMError`); este helper solo
// llama a `httperrors.Respond` para traducirlo a HTTP.
func RegisterStatusEndpoint[T any](
	auth *gin.RouterGroup,
	rbac *RBACMiddleware,
	resource, permission, basePath string,
	update StatusUpdater[T],
	mapper StatusResponseMapper[T],
) {
	auth.PATCH(basePath+"/:id/status", rbac.RequirePermission(resource, permission), func(c *gin.Context) {
		orgID, id, ok := ParseAuthTenantAndParamID(c, "id", "id")
		if !ok {
			return
		}
		var req statusRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			WriteValidation(c, "invalid request body")
			return
		}
		next := strings.TrimSpace(strings.ToLower(req.Status))
		if next == "" {
			httperrors.Respond(c, domainerr.Validation("status is required"))
			return
		}
		actor := GetAuthContext(c).Actor
		out, err := update(c.Request.Context(), orgID, id, next, actor)
		if err != nil {
			httperrors.Respond(c, err)
			return
		}
		c.JSON(http.StatusOK, mapper(out))
	})
}
