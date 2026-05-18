package public

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devpablocristo/pymes/beauty/backend/internal/shared/pymescore"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalgin"
)

// coreServicesPort expone el catálogo público de servicios servido por pymes-core.
type coreServicesPort interface {
	ListPublicServices(ctx context.Context, tenantRef, vertical, segment, search string) ([]pymescore.CoreService, error)
}

type bookingPort interface {
	BookScheduling(ctx context.Context, tenantRef string, payload map[string]any) (map[string]any, error)
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
	group.GET("/public/:tenant_slug/beauty/services", h.ListServices)
	group.POST("/public/:tenant_slug/beauty/bookings", h.BookScheduling)
}

func (h *Handler) ListServices(c *gin.Context) {
	tenantSlug := strings.TrimSpace(c.Param("tenant_slug"))
	if tenantSlug == "" {
		verticalgin.WriteValidation(c, "tenant_slug is required")
		return
	}
	if h.coreServices == nil {
		verticalgin.WriteError(c, http.StatusServiceUnavailable, "UPSTREAM_UNAVAILABLE", "core services not configured")
		return
	}
	items, err := h.coreServices.ListPublicServices(
		c.Request.Context(),
		tenantSlug,
		"beauty",
		"salon",
		strings.TrimSpace(c.Query("search")),
	)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	publicItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		duration := 0
		if item.DefaultDurationMinutes != nil {
			duration = *item.DefaultDurationMinutes
		}
		publicItems = append(publicItems, map[string]any{
			"id":               item.ID,
			"code":             item.Code,
			"name":             item.Name,
			"description":      item.Description,
			"category":         item.CategoryCode,
			"duration_minutes": duration,
			"base_price":       item.SalePrice,
			"currency":         item.Currency,
			"tax_rate":         item.TaxRate,
		})
	}
	verticalgin.WriteListResponse(c, publicItems, int64(len(publicItems)), false, "")
}

func (h *Handler) BookScheduling(c *gin.Context) {
	if h.bookings == nil {
		verticalgin.WriteError(c, http.StatusNotImplemented, "UPSTREAM_UNAVAILABLE", "booking not configured")
		return
	}
	tenantSlug := strings.TrimSpace(c.Param("tenant_slug"))
	if tenantSlug == "" {
		verticalgin.WriteValidation(c, "tenant_slug is required")
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
	out, err := h.bookings.BookScheduling(c.Request.Context(), tenantSlug, payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}
