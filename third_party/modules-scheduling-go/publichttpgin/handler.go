package publichttpgin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	schedulingpublic "github.com/devpablocristo/modules/scheduling/go/publicapi"
	publicdto "github.com/devpablocristo/modules/scheduling/go/publichttpgin/dto"
)

type servicePort interface {
	ResolveOrgID(ctx context.Context, ref string) (uuid.UUID, error)
	ListPublicServices(ctx context.Context, orgID uuid.UUID, limit int) ([]schedulingpublic.Service, error)
	GetAvailability(ctx context.Context, orgID uuid.UUID, query schedulingpublic.AvailabilityQuery) ([]schedulingpublic.AvailabilitySlot, error)
	Book(ctx context.Context, orgID uuid.UUID, payload map[string]any) (schedulingpublic.Booking, error)
	ListByPhone(ctx context.Context, orgID uuid.UUID, phone string, limit int) ([]schedulingpublic.Booking, error)
	ListPublicQueues(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]schedulingpublic.QueueSummary, error)
	CreatePublicQueueTicket(ctx context.Context, orgID, queueID uuid.UUID, payload map[string]any) (schedulingpublic.QueueTicket, schedulingpublic.QueuePosition, error)
	GetPublicQueueTicketPosition(ctx context.Context, orgID, queueID, ticketID uuid.UUID) (schedulingpublic.QueuePosition, error)
	JoinWaitlist(ctx context.Context, orgID uuid.UUID, payload map[string]any) (schedulingpublic.WaitlistEntry, error)
	ConfirmBookingByToken(ctx context.Context, orgID uuid.UUID, token string) (schedulingpublic.Booking, error)
	CancelBookingByToken(ctx context.Context, orgID uuid.UUID, token, reason string) (schedulingpublic.Booking, error)
}

type Handler struct {
	svc          servicePort
	isNotFoundFn func(error) bool
}

func NewHandler(svc servicePort, isNotFoundFn func(error) bool) *Handler {
	return &Handler{svc: svc, isNotFoundFn: isNotFoundFn}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	scheduling := group.Group("/scheduling")
	scheduling.GET("/services", h.ListServices)
	scheduling.GET("/availability", h.GetAvailability)
	scheduling.POST("/book", h.Book)
	scheduling.GET("/my-bookings", h.ListBookingsByPhone)
	scheduling.GET("/queues", h.ListQueues)
	scheduling.POST("/queues/:id/tickets", h.CreateQueueTicket)
	scheduling.GET("/queues/:id/tickets/:ticket_id/position", h.GetQueueTicketPosition)
	scheduling.POST("/waitlist", h.JoinWaitlist)
	scheduling.POST("/bookings/actions/confirm", h.ConfirmBookingByToken)
	scheduling.POST("/bookings/actions/cancel", h.CancelBookingByToken)
}

func (h *Handler) resolveOrgID(c *gin.Context) (uuid.UUID, bool) {
	orgID, err := h.svc.ResolveOrgID(c.Request.Context(), c.Param("org_id"))
	if err != nil {
		if h.isNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return uuid.Nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve organization"})
		return uuid.Nil, false
	}
	return orgID, true
}

func (h *Handler) ListServices(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "20")))
	items, err := h.svc.ListPublicServices(c.Request.Context(), orgID, limit)
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
	items, err := h.svc.GetAvailability(c.Request.Context(), orgID, schedulingpublic.AvailabilityQuery{
		Date:       day.UTC(),
		Duration:   duration,
		BranchID:   branchID,
		ServiceID:  serviceID,
		ResourceID: resourceID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid availability query"})
		return
	}
	response := make([]gin.H, 0, len(items))
	for _, slot := range items {
		response = append(response, gin.H{
			"start_at":  slot.StartAt.UTC().Format(time.RFC3339),
			"end_at":    slot.EndAt.UTC().Format(time.RFC3339),
			"remaining": slot.Remaining,
		})
	}
	c.JSON(http.StatusOK, gin.H{"date": day.UTC().Format("2006-01-02"), "slots": response})
}

func (h *Handler) Book(c *gin.Context) {
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
	item, err := h.svc.Book(c.Request.Context(), orgID, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to create booking"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) ListBookingsByPhone(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	phone := strings.TrimSpace(c.Query("phone"))
	if phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "phone query param is required"})
		return
	}
	items, err := h.svc.ListByPhone(c.Request.Context(), orgID, phone, 20)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to fetch bookings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) ListQueues(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	branchID, err := parseUUIDQuery(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	items, err := h.svc.ListPublicQueues(c.Request.Context(), orgID, branchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch queues"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateQueueTicket(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	queueID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid queue id"})
		return
	}
	var req publicdto.CreateQueueTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	payload := map[string]any{
		"party_id":        req.PartyID,
		"customer_name":   req.CustomerName,
		"customer_phone":  req.CustomerPhone,
		"customer_email":  req.CustomerEmail,
		"priority":        req.Priority,
		"source":          req.Source,
		"idempotency_key": req.IdempotencyKey,
		"notes":           req.Notes,
		"metadata":        req.Metadata,
	}
	ticket, position, err := h.svc.CreatePublicQueueTicket(c.Request.Context(), orgID, queueID, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to create ticket"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ticket": ticket, "position": position})
}

func (h *Handler) GetQueueTicketPosition(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	queueID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid queue id"})
		return
	}
	ticketID, err := uuid.Parse(strings.TrimSpace(c.Param("ticket_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}
	item, err := h.svc.GetPublicQueueTicketPosition(c.Request.Context(), orgID, queueID, ticketID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to fetch queue position"})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) JoinWaitlist(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	var req publicdto.CreateWaitlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	payload := map[string]any{
		"branch_id":          req.BranchID,
		"service_id":         req.ServiceID,
		"resource_id":        req.ResourceID,
		"party_id":           req.PartyID,
		"customer_name":      req.CustomerName,
		"customer_phone":     req.CustomerPhone,
		"customer_email":     req.CustomerEmail,
		"requested_start_at": req.RequestedStartAt,
		"source":             req.Source,
		"idempotency_key":    req.IdempotencyKey,
		"notes":              req.Notes,
		"metadata":           req.Metadata,
	}
	item, err := h.svc.JoinWaitlist(c.Request.Context(), orgID, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to join waitlist"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) ConfirmBookingByToken(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token query param is required"})
		return
	}
	item, err := h.svc.ConfirmBookingByToken(c.Request.Context(), orgID, token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to confirm booking"})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) CancelBookingByToken(c *gin.Context) {
	orgID, ok := h.resolveOrgID(c)
	if !ok {
		return
	}
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token query param is required"})
		return
	}
	var req publicdto.CancelBookingActionRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	item, err := h.svc.CancelBookingByToken(c.Request.Context(), orgID, token, req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to cancel booking"})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if h.isNotFoundFn != nil {
		return h.isNotFoundFn(err)
	}
	return false
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
