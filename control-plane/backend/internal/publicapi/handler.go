package publicapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/info", h.GetBusinessInfo)
	group.GET("/services", h.GetPublicServices)
	group.GET("/availability", h.GetAvailability)
	group.POST("/book", h.BookAppointment)
	group.GET("/my-appointments", h.GetMyAppointments)
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

	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "20")))
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

	slots, err := h.repo.GetAvailability(c.Request.Context(), orgID, day.UTC(), duration)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid duration"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch availability"})
		return
	}

	response := make([]gin.H, 0, len(slots))
	for _, slot := range slots {
		response = append(response, gin.H{
			"start_at":  slot.StartAt.UTC().Format(time.RFC3339),
			"end_at":    slot.EndAt.UTC().Format(time.RFC3339),
			"remaining": slot.Remaining,
		})
	}

	c.JSON(http.StatusOK, gin.H{"date": day.UTC().Format("2006-01-02"), "slots": response})
}

type bookRequest struct {
	CustomerName  string `json:"party_name"`
	CustomerPhone string `json:"party_phone"`
	Title         string `json:"title"`
	StartAt       string `json:"start_at"`
	Duration      int    `json:"duration"`
}

func (h *Handler) BookAppointment(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}

	var req bookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.StartAt))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_at (expected RFC3339)"})
		return
	}

	appointment, err := h.repo.Book(c.Request.Context(), orgID, BookInput{
		CustomerName:  req.CustomerName,
		CustomerPhone: req.CustomerPhone,
		Title:         req.Title,
		StartAt:       startAt.UTC(),
		Duration:      req.Duration,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid booking payload"})
			return
		case errors.Is(err, ErrSlotUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": "slot not available"})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create appointment"})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          appointment.ID.String(),
		"party_name":  appointment.CustomerName,
		"party_phone": appointment.CustomerPhone,
		"title":       appointment.Title,
		"status":      appointment.Status,
		"start_at":    appointment.StartAt.UTC().Format(time.RFC3339),
		"end_at":      appointment.EndAt.UTC().Format(time.RFC3339),
		"duration":    appointment.Duration,
	})
}

func (h *Handler) GetMyAppointments(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch appointments"})
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
