package public

import (
	"context"
	"net/http"
	"strings"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/pymescore"
	"github.com/gin-gonic/gin"
)

// coreServicesPort expone el catálogo público de servicios servido por pymes-core.
type coreServicesPort interface {
	ListPublicServices(ctx context.Context, orgRef, vertical, segment, search string) ([]pymescore.CoreService, error)
}

type bookingPort interface {
	BookScheduling(ctx context.Context, orgRef string, payload map[string]any) (map[string]any, error)
}

type Handler struct {
	coreServices coreServicesPort
	bookings     bookingPort
}

func NewHandler(coreServices coreServicesPort, bookings bookingPort) *Handler {
	return &Handler{
		coreServices: coreServices,
		bookings:     bookings,
	}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/public/:org_slug/auto-repair/services", h.ListServices)
	group.POST("/public/:org_slug/auto-repair/bookings", h.BookScheduling)

	group.GET("/public/:org_slug/workshops/services", h.ListServices)
	group.POST("/public/:org_slug/workshops/bookings", h.BookScheduling)
}

func (h *Handler) resolveOrgRef(c *gin.Context) (string, bool) {
	orgSlug := strings.TrimSpace(c.Param("org_slug"))
	if orgSlug == "" {
		verticalgin.WriteValidation(c, "org_slug is required")
		return "", false
	}
	return orgSlug, true
}

func (h *Handler) listSegmentServices(c *gin.Context, segment string) {
	orgRef, ok := h.resolveOrgRef(c)
	if !ok {
		return
	}
	if h.coreServices == nil {
		verticalgin.WriteError(c, http.StatusServiceUnavailable, "UPSTREAM_UNAVAILABLE", "core services not configured")
		return
	}
	items, err := h.coreServices.ListPublicServices(
		c.Request.Context(),
		orgRef,
		"workshops",
		segment,
		strings.TrimSpace(c.Query("search")),
	)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	publicItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, map[string]any{
			"id":              item.ID,
			"code":            item.Code,
			"name":            item.Name,
			"description":     item.Description,
			"category":        item.CategoryCode,
			"estimated_hours": estimatedHoursFromMetadata(item),
			"base_price":      item.SalePrice,
			"currency":        item.Currency,
			"tax_rate":        item.TaxRate,
		})
	}
	verticalgin.WriteListResponse(c, publicItems, int64(len(publicItems)), false, "")
}

func (h *Handler) ListServices(c *gin.Context) {
	h.listSegmentServices(c, "auto_repair")
}

// estimatedHoursFromMetadata convierte metadata.estimated_hours (si existe) a float;
// si no, deriva desde DefaultDurationMinutes / 60.
func estimatedHoursFromMetadata(svc pymescore.CoreService) float64 {
	if svc.Metadata != nil {
		if v, ok := svc.Metadata["estimated_hours"].(float64); ok {
			return v
		}
	}
	if svc.DefaultDurationMinutes != nil && *svc.DefaultDurationMinutes > 0 {
		return float64(*svc.DefaultDurationMinutes) / 60.0
	}
	return 0
}

func (h *Handler) BookScheduling(c *gin.Context) {
	if h.bookings == nil {
		verticalgin.WriteError(c, http.StatusNotImplemented, "UPSTREAM_UNAVAILABLE", "booking not configured")
		return
	}
	orgSlug, ok := h.resolveOrgRef(c)
	if !ok {
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		verticalgin.WriteValidation(c, "invalid request body")
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	out, err := h.bookings.BookScheduling(c.Request.Context(), orgSlug, payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}
