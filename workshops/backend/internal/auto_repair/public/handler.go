package public

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workshopservices"
	svcdomain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workshopservices/usecases/domain"
)

type servicePort interface {
	List(ctx context.Context, p workshopservices.ListParams) ([]svcdomain.Service, int64, bool, *uuid.UUID, error)
}

type bookingPort interface {
	BookAppointment(ctx context.Context, orgRef string, payload map[string]any) (map[string]any, error)
}

type orgResolver interface {
	ResolveOrgID(ctx context.Context, orgSlug string) (uuid.UUID, error)
}

type Handler struct {
	services servicePort
	bookings bookingPort
	orgs     orgResolver
}

func NewHandler(services servicePort, bookings bookingPort, orgs orgResolver) *Handler {
	return &Handler{
		services: services,
		bookings: bookings,
		orgs:     orgs,
	}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/public/:org_slug/auto-repair/services", h.ListServices)
	group.POST("/public/:org_slug/auto-repair/appointments", h.BookAppointment)

	group.GET("/public/:org_slug/workshops/services", h.ListServices)
	group.POST("/public/:org_slug/workshops/appointments", h.BookAppointment)
}

func (h *Handler) resolveOrgID(c *gin.Context) (uuid.UUID, bool) {
	orgSlug := strings.TrimSpace(c.Param("org_slug"))
	if orgSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org_slug is required"})
		return uuid.Nil, false
	}
	if orgID, err := uuid.Parse(orgSlug); err == nil {
		return orgID, true
	}
	if h.orgs == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org identifier"})
		return uuid.Nil, false
	}
	orgID, err := h.orgs.ResolveOrgID(c.Request.Context(), orgSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
		return uuid.Nil, false
	}
	return orgID, true
}

func (h *Handler) ListServices(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	items, _, _, _, err := h.services.List(c.Request.Context(), workshopservices.ListParams{
		OrgID:  orgID,
		Limit:  100,
		Search: strings.TrimSpace(c.Query("search")),
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	publicItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if !item.IsActive {
			continue
		}
		publicItems = append(publicItems, map[string]any{
			"id":              item.ID.String(),
			"code":            item.Code,
			"name":            item.Name,
			"description":     item.Description,
			"category":        item.Category,
			"estimated_hours": item.EstimatedHours,
			"base_price":      item.BasePrice,
			"currency":        item.Currency,
			"tax_rate":        item.TaxRate,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": publicItems})
}

func (h *Handler) BookAppointment(c *gin.Context) {
	if h.bookings == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "booking not configured"})
		return
	}
	orgSlug := strings.TrimSpace(c.Param("org_slug"))
	if orgSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org_slug is required"})
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	out, err := h.bookings.BookAppointment(c.Request.Context(), orgSlug, payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}
