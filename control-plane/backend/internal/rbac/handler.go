package rbac

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/rbac/handler/dto"
	rbacdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/rbac/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/authz"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/pkg/http/errors"
)

type usecasesPort interface {
	ListRoles(ctx context.Context, orgID string) ([]rbacdomain.Role, error)
	GetRole(ctx context.Context, orgID, roleID string) (rbacdomain.Role, error)
	CreateRole(ctx context.Context, orgID, actor, name, description string, perms []rbacdomain.Permission) (rbacdomain.Role, error)
	UpdateRole(ctx context.Context, orgID, roleID, actor string, description *string, permissions []rbacdomain.Permission) (rbacdomain.Role, error)
	DeleteRole(ctx context.Context, orgID, roleID, actor string) error
	AssignRole(ctx context.Context, orgID, roleID, userID, actor string) error
	RemoveRole(ctx context.Context, orgID, roleID, userID, actor string) error
	EffectivePermissions(ctx context.Context, orgID, userID string) (map[string][]string, error)
}

type Handler struct {
	uc usecasesPort
}

func NewHandler(uc usecasesPort) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	auth.GET("/roles", h.ListRoles)
	auth.POST("/roles", h.CreateRole)
	auth.GET("/roles/:id", h.GetRole)
	auth.PUT("/roles/:id", h.UpdateRole)
	auth.DELETE("/roles/:id", h.DeleteRole)
	auth.POST("/roles/:id/assign/:user_id", h.AssignRole)
	auth.DELETE("/roles/:id/assign/:user_id", h.RemoveRole)
	auth.GET("/users/:user_id/permissions", h.UserPermissions)
}

func (h *Handler) ListRoles(c *gin.Context) {
	authCtx, ok := requireAdmin(c)
	if !ok {
		return
	}
	roles, err := h.uc.ListRoles(c.Request.Context(), authCtx.OrgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": roles})
}

func (h *Handler) CreateRole(c *gin.Context) {
	authCtx, ok := requireAdmin(c)
	if !ok {
		return
	}

	var req dto.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	out, err := h.uc.CreateRole(
		c.Request.Context(),
		authCtx.OrgID,
		authCtx.Actor,
		req.Name,
		req.Description,
		toPermissions(req.Permissions),
	)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) GetRole(c *gin.Context) {
	authCtx, ok := requireAdmin(c)
	if !ok {
		return
	}
	out, err := h.uc.GetRole(c.Request.Context(), authCtx.OrgID, c.Param("id"))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) UpdateRole(c *gin.Context) {
	authCtx, ok := requireAdmin(c)
	if !ok {
		return
	}
	var req dto.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var perms []rbacdomain.Permission
	if req.Permissions != nil {
		perms = toPermissions(req.Permissions)
	}
	updated, err := h.uc.UpdateRole(c.Request.Context(), authCtx.OrgID, c.Param("id"), authCtx.Actor, req.Description, perms)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) DeleteRole(c *gin.Context) {
	authCtx, ok := requireAdmin(c)
	if !ok {
		return
	}
	if err := h.uc.DeleteRole(c.Request.Context(), authCtx.OrgID, c.Param("id"), authCtx.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) AssignRole(c *gin.Context) {
	authCtx, ok := requireAdmin(c)
	if !ok {
		return
	}
	if err := h.uc.AssignRole(c.Request.Context(), authCtx.OrgID, c.Param("id"), c.Param("user_id"), authCtx.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RemoveRole(c *gin.Context) {
	authCtx, ok := requireAdmin(c)
	if !ok {
		return
	}
	if err := h.uc.RemoveRole(c.Request.Context(), authCtx.OrgID, c.Param("id"), c.Param("user_id"), authCtx.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) UserPermissions(c *gin.Context) {
	authCtx, ok := requireAdmin(c)
	if !ok {
		return
	}
	permissions, err := h.uc.EffectivePermissions(c.Request.Context(), authCtx.OrgID, c.Param("user_id"))
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"permissions": permissions})
}

func toPermissions(in []dto.PermissionInput) []rbacdomain.Permission {
	out := make([]rbacdomain.Permission, 0, len(in))
	for _, p := range in {
		resource := strings.TrimSpace(p.Resource)
		action := strings.TrimSpace(p.Action)
		if resource == "" || action == "" {
			continue
		}
		out = append(out, rbacdomain.Permission{Resource: resource, Action: action})
	}
	return out
}

func requireAdmin(c *gin.Context) (handlers.AuthContext, bool) {
	authCtx := handlers.GetAuthContext(c)
	if !authz.IsAdmin(authCtx.Role, authCtx.Scopes) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin permissions required"})
		return handlers.AuthContext{}, false
	}
	return authCtx, true
}
