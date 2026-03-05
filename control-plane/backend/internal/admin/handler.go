package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/admin/handler/dto"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/authz"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
)

type Handler struct {
	uc *Usecases
}

func NewHandler(uc *Usecases) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup) {
	auth.GET("/admin/bootstrap", h.GetBootstrap)
	auth.GET("/admin/tenant-settings", h.GetTenantSettings)
	auth.PUT("/admin/tenant-settings", h.UpdateTenantSettings)
	auth.GET("/admin/activity", h.ListActivity)
}

func (h *Handler) GetBootstrap(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	if !authz.IsAdmin(authCtx.Role, authCtx.Scopes) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin permissions required"})
		return
	}
	payload, err := h.uc.GetBootstrap(c.Request.Context(), authCtx.OrgID, authCtx.Role, authCtx.Scopes, authCtx.Actor, authCtx.AuthMethod)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (h *Handler) GetTenantSettings(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	if !(authCtx.Role == "admin" || authz.HasScope(authCtx.Scopes, "admin:console:read") || authz.HasScope(authCtx.Scopes, "admin:console:write")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin read permission required"})
		return
	}
	settings, err := h.uc.GetTenantSettings(c.Request.Context(), authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *Handler) UpdateTenantSettings(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	if !(authCtx.Role == "admin" || authz.HasScope(authCtx.Scopes, "admin:console:write")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin write permission required"})
		return
	}
	var req dto.UpdateTenantSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := h.uc.UpdateTenantSettings(c.Request.Context(), authCtx.OrgID, req.PlanCode, req.HardLimits, &authCtx.Actor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) ListActivity(c *gin.Context) {
	authCtx := handlers.GetAuthContext(c)
	if !(authCtx.Role == "admin" || authz.HasScope(authCtx.Scopes, "admin:console:read") || authz.HasScope(authCtx.Scopes, "admin:console:write")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin read permission required"})
		return
	}
	items, err := h.uc.ListActivity(c.Request.Context(), authCtx.OrgID, 200)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
