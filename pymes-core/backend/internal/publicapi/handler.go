package publicapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/core/http/go/pagination"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type repositoryPort interface {
	ResolveTenantID(ctx context.Context, ref string) (uuid.UUID, error)
	GetBusinessInfo(ctx context.Context, tenantID uuid.UUID) (BusinessInfo, error)
	ListPublicServiceCatalog(ctx context.Context, tenantID uuid.UUID, vertical, segment, search string, limit int) ([]PublicServiceCatalogItem, error)
	GetAvailability(ctx context.Context, tenantID uuid.UUID, query AvailabilityQuery) ([]AvailabilitySlot, error)
	Book(ctx context.Context, tenantID uuid.UUID, payload map[string]any) (BookingPublic, error)
	ListByPhone(ctx context.Context, tenantID uuid.UUID, phone string, limit int) ([]BookingPublic, error)
}

type Handler struct {
	repo repositoryPort
}

func NewHandler(repo repositoryPort) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/info", h.GetBusinessInfo)
	group.GET("/catalog/services", h.GetPublicServiceCatalog)
	group.GET("/availability", h.GetAvailability)
	group.POST("/book", h.BookScheduling)
	group.GET("/my-bookings", h.GetMyBookings)
}

func (h *Handler) resolveTenantID(c *gin.Context) (uuid.UUID, bool) {
	tenantID, err := h.repo.ResolveTenantID(c.Request.Context(), c.Param("tenant_id"))
	if err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			httperrors.Write(c, http.StatusNotFound, "NOT_FOUND", "tenant not found")
			return uuid.Nil, false
		}
		httperrors.Write(c, http.StatusInternalServerError, "INTERNAL", "failed to resolve tenant")
		return uuid.Nil, false
	}
	return tenantID, true
}

func (h *Handler) GetBusinessInfo(c *gin.Context) {
	tenantID, ok := h.resolveTenantID(c)
	if !ok {
		return
	}

	info, err := h.repo.GetBusinessInfo(c.Request.Context(), tenantID)
	if err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			httperrors.Write(c, http.StatusNotFound, "NOT_FOUND", "tenant not found")
			return
		}
		httperrors.Write(c, http.StatusInternalServerError, "INTERNAL", "failed to fetch tenant info")
		return
	}

	c.JSON(http.StatusOK, info)
}

// GetPublicServiceCatalog devuelve el catálogo rico desde public.services con filtros
// opcionales por vertical/segment (almacenados en metadata jsonb).
func (h *Handler) GetPublicServiceCatalog(c *gin.Context) {
	tenantID, ok := h.resolveTenantID(c)
	if !ok {
		return
	}
	limit := handlers.ParseLimitQuery(c, "limit", "100", pagination.Config{DefaultLimit: 100, MaxLimit: 200})
	items, err := h.repo.ListPublicServiceCatalog(
		c.Request.Context(),
		tenantID,
		strings.TrimSpace(c.Query("vertical")),
		strings.TrimSpace(c.Query("segment")),
		strings.TrimSpace(c.Query("search")),
		limit,
	)
	if err != nil {
		httperrors.Write(c, http.StatusInternalServerError, "INTERNAL", "failed to fetch service catalog")
		return
	}
	handlers.WriteListResponse(c, items, int64(len(items)), false, "")
}

func (h *Handler) GetAvailability(c *gin.Context) {
	tenantID, ok := h.resolveTenantID(c)
	if !ok {
		return
	}

	rawDate := strings.TrimSpace(c.Query("date"))
	if rawDate == "" {
		handlers.WriteValidation(c, "date query param is required (YYYY-MM-DD)")
		return
	}
	day, err := time.Parse("2006-01-02", rawDate)
	if err != nil {
		handlers.WriteValidation(c, "invalid date (expected YYYY-MM-DD)")
		return
	}
	duration, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("duration", "60")))
	branchID, err := parseUUIDQuery(c.Query("branch_id"))
	if err != nil {
		handlers.WriteValidation(c, "invalid branch_id")
		return
	}
	serviceID, err := parseUUIDQuery(c.Query("service_id"))
	if err != nil {
		handlers.WriteValidation(c, "invalid service_id")
		return
	}
	resourceID, err := parseUUIDQuery(c.Query("resource_id"))
	if err != nil {
		handlers.WriteValidation(c, "invalid resource_id")
		return
	}

	slots, err := h.repo.GetAvailability(c.Request.Context(), tenantID, AvailabilityQuery{
		Date:       day.UTC(),
		Duration:   duration,
		BranchID:   branchID,
		ServiceID:  serviceID,
		ResourceID: resourceID,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			handlers.WriteValidation(c, "invalid availability query")
			return
		default:
			httperrors.Write(c, http.StatusInternalServerError, "INTERNAL", "failed to fetch availability")
			return
		}
	}

	response := make([]gin.H, 0, len(slots))
	for _, slot := range slots {
		response = append(response, gin.H{
			"start_at":  slot.StartAt.UTC().Format(time.RFC3339),
			"end_at":    slot.EndAt.UTC().Format(time.RFC3339),
			"remaining": slot.Remaining,
		})
	}

	out := gin.H{"date": day.UTC().Format("2006-01-02"), "slots": response}
	if branchID != nil {
		out["branch_id"] = branchID.String()
	}
	if serviceID != nil {
		out["service_id"] = serviceID.String()
	}
	if resourceID != nil {
		out["resource_id"] = resourceID.String()
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) BookScheduling(c *gin.Context) {
	tenantID, ok := h.resolveTenantID(c)
	if !ok {
		return
	}

	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		handlers.WriteValidation(c, "invalid request body")
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}

	booking, err := h.repo.Book(c.Request.Context(), tenantID, payload)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			handlers.WriteValidation(c, "invalid booking payload")
			return
		case errors.Is(err, ErrSlotUnavailable):
			httperrors.Write(c, http.StatusConflict, "CONFLICT", "slot not available")
			return
		default:
			httperrors.Write(c, http.StatusInternalServerError, "INTERNAL", "failed to create booking")
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          booking.ID.String(),
		"party_name":  booking.CustomerName,
		"party_phone": booking.CustomerPhone,
		"title":       booking.Title,
		"status":      booking.Status,
		"start_at":    booking.StartAt.UTC().Format(time.RFC3339),
		"end_at":      booking.EndAt.UTC().Format(time.RFC3339),
		"duration":    booking.Duration,
	})
}

func (h *Handler) GetMyBookings(c *gin.Context) {
	tenantID, ok := h.resolveTenantID(c)
	if !ok {
		return
	}

	phone := strings.TrimSpace(c.Query("phone"))
	if phone == "" {
		handlers.WriteValidation(c, "phone query param is required")
		return
	}

	items, err := h.repo.ListByPhone(c.Request.Context(), tenantID, phone, 20)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			handlers.WriteValidation(c, "invalid phone")
			return
		}
		httperrors.Write(c, http.StatusInternalServerError, "INTERNAL", "failed to fetch bookings")
		return
	}

	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, gin.H{
			"id":          item.ID.String(),
			"party_name":  item.CustomerName,
			"party_phone": item.CustomerPhone,
			"title":       item.Title,
			"status":      item.Status,
			"start_at":    item.StartAt.UTC().Format(time.RFC3339),
			"end_at":      item.EndAt.UTC().Format(time.RFC3339),
			"duration":    item.Duration,
		})
	}

	handlers.WriteListResponse(c, out, int64(len(out)), false, "")
}

func parseUUIDQuery(raw string) (*uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
