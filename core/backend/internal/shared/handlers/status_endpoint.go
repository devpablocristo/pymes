// Package handlers — RegisterStatusEndpoint now lives in platform/http/gin/go
// (v0.2.2+). The pymes shim below preserves the previous call site shape:
// same generic signature, same RBAC + body + error mapping.
//
// New code should prefer the canonical:
//
//	ginmw.RegisterStatusEndpoint[T](auth, rbac, resource, perm, basePath, update, mapper, nil)
package handlers

import (
	"context"

	ginmw "github.com/devpablocristo/platform/http/gin/go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// StatusUpdater is the signature of a usecase that changes a resource's status.
type StatusUpdater[T any] = ginmw.StatusUpdater[T]

// StatusResponseMapper converts the domain value to the JSON response.
type StatusResponseMapper[T any] = ginmw.StatusResponseMapper[T]

// RegisterStatusEndpoint delegates to platform/http/gin/go.RegisterStatusEndpoint.
// Default extractors preserve pymes' previous behavior (tenant from
// AuthContext.OrgID, :id UUID param, /:id/status suffix).
func RegisterStatusEndpoint[T any](
	auth *gin.RouterGroup,
	rbac *RBACMiddleware,
	resource, permission, basePath string,
	update StatusUpdater[T],
	mapper StatusResponseMapper[T],
) {
	ginmw.RegisterStatusEndpoint[T](auth, rbac, resource, permission, basePath, update, mapper, nil)
}

// (kept to satisfy the import set when local tests reuse the names)
var _ context.Context
var _ uuid.UUID
