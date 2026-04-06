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
)

type repositoryPort interface {
	ResolveOrgID(ctx context.Context, ref string) (uuid.UUID, error)
	GetBusinessInfo(ctx context.Context, orgID uuid.UUID) (BusinessInfo, error)
	ListPublicServices(ctx context.Context, orgID uuid.UUID, limit int) ([]PublicService, error)
	GetAvailability(ctx context.Context, orgID uuid.UUID, query AvailabilityQuery) ([]AvailabilitySlot, error)
	Book(ctx context.Context, orgID uuid.UUID, payload map[string]any) (BookingPublic, error)
	ListByPhone(ctx context.Context, orgID uuid.UUID, phone string, limit int) ([]BookingPublic, error)
}

type Handler struct {
	repo repositoryPort
}

func NewHandler(repo repositoryPort) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/info", h.GetBusinessInfo)
	group.GET("/services", h.GetPublicServices)
	group.GET("/availability", h.GetAvailability)
	group.POST("/book", h.BookScheduling)
	group.GET("/my-bookings", h.GetMyBookings)
}

func (h *Handler) resolveOrgID(c *gin.Context) (uuid.UUID, bool) {
	orgID, err := h.repo.ResolveOrgID(c.Request.Context(), c.Param("org_id"))
	if err != nil {
		if errors.Is(err, ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return uuid.Nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve organization"})
		return uuid.Nil, false
	}
	return orgID, true
}

func (h *Handler) GetBusinessInfo(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	info, err := h.repo.GetBusinessInfo(c.Request.Context(), orgID)
	if err != nil {
		if errors.Is(err, ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch organization info"})
		return
	}

	c.JSON(http.StatusOK, info)
}

func (h *Handler) GetPublicServices(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	limit := handlers.ParseLimitQuery(c, "limit", "20", pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	items, err := h.repo.ListPublicServices(c.Request.Context(), orgID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch services"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) GetAvailability(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	rawDate := strings.TrimSpace(c.Query("date"))
	if rawDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date query param is required (YYYY-MM-DD)"})
		return
	}
	day, err := time.Parse("2006-01-02", rawDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date (expected YYYY-MM-DD)"})
		return
	}
	duration, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("duration", "60")))
	branchID, err := parseUUIDQuery(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	serviceID, err := parseUUIDQuery(c.Query("service_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service_id"})
		return
	}
	resourceID, err := parseUUIDQuery(c.Query("resource_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource_id"})
		return
	}

	slots, err := h.repo.GetAvailability(c.Request.Context(), orgID, AvailabilityQuery{
		Date:       day.UTC(),
		Duration:   duration,
		BranchID:   branchID,
		ServiceID:  serviceID,
		ResourceID: resourceID,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid availability query"})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch availability"})
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
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}

	booking, err := h.repo.Book(c.Request.Context(), orgID, payload)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking payload"})
			return
		case errors.Is(err, ErrSlotUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": "slot not available"})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create booking"})
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
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	phone := strings.TrimSpace(c.Query("phone"))
	if phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "phone query param is required"})
		return
	}

	items, err := h.repo.ListByPhone(c.Request.Context(), orgID, phone, 20)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid phone"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch bookings"})
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

	c.JSON(http.StatusOK, gin.H{"items": out})
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
