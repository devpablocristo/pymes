package outwebhooks

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	webhookdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/outwebhooks/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/internal/shared/httperrors"
)

type usecasesPort interface {
	ListEndpoints(ctx context.Context, orgID uuid.UUID) ([]webhookdomain.Endpoint, error)
	CreateEndpoint(ctx context.Context, in webhookdomain.Endpoint) (webhookdomain.Endpoint, error)
	GetEndpoint(ctx context.Context, orgID, id uuid.UUID) (webhookdomain.Endpoint, error)
	UpdateEndpoint(ctx context.Context, in webhookdomain.Endpoint) (webhookdomain.Endpoint, error)
	DeleteEndpoint(ctx context.Context, orgID, id uuid.UUID) error
	ListDeliveries(ctx context.Context, orgID, endpointID uuid.UUID, limit int) ([]webhookdomain.Delivery, error)
	SendTest(ctx context.Context, orgID, endpointID uuid.UUID, actor string) error
	ReplayDelivery(ctx context.Context, deliveryID uuid.UUID) error
}

type Handler struct{ uc usecasesPort }

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/webhook-endpoints", rbac.RequirePermission("admin", "read"), h.ListEndpoints)
	auth.POST("/webhook-endpoints", rbac.RequirePermission("admin", "update"), h.CreateEndpoint)
	auth.GET("/webhook-endpoints/:id", rbac.RequirePermission("admin", "read"), h.GetEndpoint)
	auth.PUT("/webhook-endpoints/:id", rbac.RequirePermission("admin", "update"), h.UpdateEndpoint)
	auth.DELETE("/webhook-endpoints/:id", rbac.RequirePermission("admin", "update"), h.DeleteEndpoint)
	auth.GET("/webhook-endpoints/:id/deliveries", rbac.RequirePermission("admin", "read"), h.ListDeliveries)
	auth.POST("/webhook-endpoints/:id/test", rbac.RequirePermission("admin", "update"), h.TestEndpoint)
	auth.POST("/webhook-deliveries/:id/replay", rbac.RequirePermission("admin", "update"), h.ReplayDelivery)
}

func (h *Handler) ListEndpoints(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	items, err := h.uc.ListEndpoints(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateEndpoint(c *gin.Context) {
	orgID, ok := parseOrg(c)
	if !ok {
		return
	}
	var req endpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	auth := handlers.GetAuthContext(c)
	out, err := h.uc.CreateEndpoint(c.Request.Context(), webhookdomain.Endpoint{OrgID: orgID, URL: req.URL, Secret: req.Secret, Events: req.Events, IsActive: req.IsActiveOrDefault(), CreatedBy: auth.Actor})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) GetEndpoint(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetEndpoint(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) UpdateEndpoint(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	var req endpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	current, err := h.uc.GetEndpoint(c.Request.Context(), orgID, id)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	current.URL = req.URL
	current.Secret = req.Secret
	current.Events = req.Events
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}
	out, err := h.uc.UpdateEndpoint(c.Request.Context(), current)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) DeleteEndpoint(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	if err := h.uc.DeleteEndpoint(c.Request.Context(), orgID, id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListDeliveries(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.uc.ListDeliveries(c.Request.Context(), orgID, id, limit)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) TestEndpoint(c *gin.Context) {
	orgID, id, ok := parseOrgID(c)
	if !ok {
		return
	}
	auth := handlers.GetAuthContext(c)
	if err := h.uc.SendTest(c.Request.Context(), orgID, id, auth.Actor); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) ReplayDelivery(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.ReplayDelivery(c.Request.Context(), id); err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type endpointRequest struct {
	URL      string   `json:"url" binding:"required,url"`
	Secret   string   `json:"secret"`
	Events   []string `json:"events"`
	IsActive *bool    `json:"is_active"`
}

func (r endpointRequest) IsActiveOrDefault() bool {
	if r.IsActive == nil {
		return true
	}
	return *r.IsActive
}

func parseOrg(c *gin.Context) (uuid.UUID, bool) {
	auth := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(auth.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, false
	}
	return orgID, true
}

func parseOrgID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := parseOrg(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
}
