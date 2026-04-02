package scheduling

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
	schedulingdto "github.com/devpablocristo/pymes/pymes-core/backend/internal/scheduling/handler/dto"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/handlers"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type usecasesPort interface {
	ListBranches(ctx context.Context, orgID uuid.UUID) ([]schedulingdomain.Branch, error)
	CreateBranch(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.Branch) (schedulingdomain.Branch, error)
	ListServices(ctx context.Context, orgID uuid.UUID) ([]schedulingdomain.Service, error)
	CreateService(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.Service) (schedulingdomain.Service, error)
	ListResources(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]schedulingdomain.Resource, error)
	CreateResource(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.Resource) (schedulingdomain.Resource, error)
	ListAvailabilityRules(ctx context.Context, orgID uuid.UUID, branchID, resourceID *uuid.UUID) ([]schedulingdomain.AvailabilityRule, error)
	CreateAvailabilityRule(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.AvailabilityRule) (schedulingdomain.AvailabilityRule, error)
	ListBlockedRanges(ctx context.Context, orgID uuid.UUID, branchID, resourceID *uuid.UUID, day *time.Time) ([]schedulingdomain.BlockedRange, error)
	CreateBlockedRange(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.BlockedRange) (schedulingdomain.BlockedRange, error)
	ListAvailableSlots(ctx context.Context, orgID uuid.UUID, query schedulingdomain.SlotQuery) ([]schedulingdomain.TimeSlot, error)
	ListBookings(ctx context.Context, orgID uuid.UUID, filter schedulingdomain.ListBookingsFilter) ([]schedulingdomain.Booking, error)
	GetBookingByID(ctx context.Context, orgID, bookingID uuid.UUID) (schedulingdomain.Booking, error)
	CreateBooking(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.CreateBookingInput) (schedulingdomain.Booking, error)
	ConfirmBooking(ctx context.Context, orgID, bookingID uuid.UUID, actor string) (schedulingdomain.Booking, error)
	CancelBooking(ctx context.Context, orgID, bookingID uuid.UUID, actor, reason string) (schedulingdomain.Booking, error)
	RescheduleBooking(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.RescheduleBookingInput) (schedulingdomain.Booking, error)
	ListQueues(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]schedulingdomain.Queue, error)
	CreateQueue(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.Queue) (schedulingdomain.Queue, error)
	IssueQueueTicket(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.CreateQueueTicketInput) (schedulingdomain.QueueTicket, error)
	GetQueueTicketPosition(ctx context.Context, orgID, queueID, ticketID uuid.UUID) (schedulingdomain.QueuePosition, error)
	CallNextTicket(ctx context.Context, orgID, queueID uuid.UUID, actor string, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error)
	MarkTicketServing(ctx context.Context, orgID, queueID, ticketID uuid.UUID, actor string, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error)
	CompleteTicket(ctx context.Context, orgID, queueID, ticketID uuid.UUID, actor string, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error)
	MarkTicketNoShow(ctx context.Context, orgID, queueID, ticketID uuid.UUID, actor string) (schedulingdomain.QueueTicket, error)
	ReassignTicket(ctx context.Context, orgID, queueID, ticketID uuid.UUID, actor string, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error)
	Dashboard(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, day time.Time) (schedulingdomain.DashboardStats, error)
	DayAgenda(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, day time.Time) ([]schedulingdomain.DayAgendaItem, error)
}

type Handler struct {
	uc usecasesPort
}

var errMissingQuery = errors.New("required")

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
	auth.GET("/scheduling/branches", rbac.RequirePermission("scheduling", "read"), h.ListBranches)
	auth.POST("/scheduling/branches", rbac.RequirePermission("scheduling", "create"), h.CreateBranch)
	auth.GET("/scheduling/services", rbac.RequirePermission("scheduling", "read"), h.ListServices)
	auth.POST("/scheduling/services", rbac.RequirePermission("scheduling", "create"), h.CreateService)
	auth.GET("/scheduling/resources", rbac.RequirePermission("scheduling", "read"), h.ListResources)
	auth.POST("/scheduling/resources", rbac.RequirePermission("scheduling", "create"), h.CreateResource)
	auth.GET("/scheduling/availability-rules", rbac.RequirePermission("scheduling", "read"), h.ListAvailabilityRules)
	auth.POST("/scheduling/availability-rules", rbac.RequirePermission("scheduling", "create"), h.CreateAvailabilityRule)
	auth.GET("/scheduling/blocked-ranges", rbac.RequirePermission("scheduling", "read"), h.ListBlockedRanges)
	auth.POST("/scheduling/blocked-ranges", rbac.RequirePermission("scheduling", "create"), h.CreateBlockedRange)
	auth.GET("/scheduling/slots", rbac.RequirePermission("scheduling", "read"), h.ListAvailableSlots)
	auth.GET("/scheduling/bookings", rbac.RequirePermission("scheduling", "read"), h.ListBookings)
	auth.GET("/scheduling/bookings/:id", rbac.RequirePermission("scheduling", "read"), h.GetBooking)
	auth.POST("/scheduling/bookings", rbac.RequirePermission("scheduling", "create"), h.CreateBooking)
	auth.POST("/scheduling/bookings/:id/confirm", rbac.RequirePermission("scheduling", "update"), h.ConfirmBooking)
	auth.POST("/scheduling/bookings/:id/cancel", rbac.RequirePermission("scheduling", "update"), h.CancelBooking)
	auth.POST("/scheduling/bookings/:id/reschedule", rbac.RequirePermission("scheduling", "update"), h.RescheduleBooking)
	auth.GET("/scheduling/queues", rbac.RequirePermission("scheduling", "read"), h.ListQueues)
	auth.POST("/scheduling/queues", rbac.RequirePermission("scheduling", "create"), h.CreateQueue)
	auth.POST("/scheduling/queues/:id/tickets", rbac.RequirePermission("scheduling", "create"), h.CreateQueueTicket)
	auth.GET("/scheduling/queues/:id/tickets/:ticket_id/position", rbac.RequirePermission("scheduling", "read"), h.GetQueueTicketPosition)
	auth.POST("/scheduling/queues/:id/next", rbac.RequirePermission("scheduling", "operate"), h.CallNextTicket)
	auth.POST("/scheduling/queues/:id/tickets/:ticket_id/serve", rbac.RequirePermission("scheduling", "operate"), h.MarkTicketServing)
	auth.POST("/scheduling/queues/:id/tickets/:ticket_id/complete", rbac.RequirePermission("scheduling", "operate"), h.CompleteTicket)
	auth.POST("/scheduling/queues/:id/tickets/:ticket_id/no-show", rbac.RequirePermission("scheduling", "operate"), h.MarkTicketNoShow)
	auth.POST("/scheduling/queues/:id/tickets/:ticket_id/reassign", rbac.RequirePermission("scheduling", "operate"), h.ReassignTicket)
	auth.GET("/scheduling/dashboard", rbac.RequirePermission("scheduling", "read"), h.Dashboard)
	auth.GET("/scheduling/day", rbac.RequirePermission("scheduling", "read"), h.DayAgenda)
}

func (h *Handler) ListBranches(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	items, err := h.uc.ListBranches(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateBranch(c *gin.Context) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	var req schedulingdto.CreateBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	out, err := h.uc.CreateBranch(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.Branch{
		Code:     req.Code,
		Name:     req.Name,
		Timezone: req.Timezone,
		Address:  req.Address,
		Active:   active,
		Metadata: req.Metadata,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) ListServices(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	items, err := h.uc.ListServices(c.Request.Context(), orgID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateService(c *gin.Context) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	var req schedulingdto.CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	resourceIDs, err := parseUUIDList(req.ResourceIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource_ids"})
		return
	}
	out, err := h.uc.CreateService(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.Service{
		Code:                   req.Code,
		Name:                   req.Name,
		Description:            req.Description,
		FulfillmentMode:        schedulingdomain.FulfillmentMode(req.FulfillmentMode),
		DefaultDurationMinutes: req.DefaultDurationMinutes,
		BufferBeforeMinutes:    req.BufferBeforeMinutes,
		BufferAfterMinutes:     req.BufferAfterMinutes,
		SlotGranularityMinutes: req.SlotGranularityMinutes,
		MaxConcurrentBookings:  req.MaxConcurrentBookings,
		Active:                 active,
		ResourceIDs:            resourceIDs,
		Metadata:               req.Metadata,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) ListResources(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	branchID, err := parseUUIDQuery(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	items, err := h.uc.ListResources(c.Request.Context(), orgID, branchID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateResource(c *gin.Context) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	var req schedulingdto.CreateResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	branchID, err := uuid.Parse(strings.TrimSpace(req.BranchID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	out, err := h.uc.CreateResource(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.Resource{
		BranchID: branchID,
		Code:     req.Code,
		Name:     req.Name,
		Kind:     schedulingdomain.ResourceKind(req.Kind),
		Capacity: req.Capacity,
		Timezone: req.Timezone,
		Active:   active,
		Metadata: req.Metadata,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) ListAvailabilityRules(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	branchID, err := parseUUIDQuery(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	resourceID, err := parseUUIDQuery(c.Query("resource_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource_id"})
		return
	}
	items, err := h.uc.ListAvailabilityRules(c.Request.Context(), orgID, branchID, resourceID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateAvailabilityRule(c *gin.Context) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	var req schedulingdto.CreateAvailabilityRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	branchID, err := uuid.Parse(strings.TrimSpace(req.BranchID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	resourceID, err := parseUUIDPtr(req.ResourceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource_id"})
		return
	}
	validFrom, err := parseDatePtr(req.ValidFrom)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid valid_from"})
		return
	}
	validUntil, err := parseDatePtr(req.ValidUntil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid valid_until"})
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	out, err := h.uc.CreateAvailabilityRule(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.AvailabilityRule{
		BranchID:               branchID,
		ResourceID:             resourceID,
		Kind:                   schedulingdomain.AvailabilityRuleKind(req.Kind),
		Weekday:                req.Weekday,
		StartTime:              req.StartTime,
		EndTime:                req.EndTime,
		SlotGranularityMinutes: req.SlotGranularityMinutes,
		ValidFrom:              validFrom,
		ValidUntil:             validUntil,
		Active:                 active,
		Metadata:               req.Metadata,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) ListBlockedRanges(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	branchID, err := parseUUIDQuery(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	resourceID, err := parseUUIDQuery(c.Query("resource_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource_id"})
		return
	}
	day, err := parseDateQuery(c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date"})
		return
	}
	items, err := h.uc.ListBlockedRanges(c.Request.Context(), orgID, branchID, resourceID, day)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateBlockedRange(c *gin.Context) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	var req schedulingdto.CreateBlockedRangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	branchID, err := uuid.Parse(strings.TrimSpace(req.BranchID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	resourceID, err := parseUUIDPtr(req.ResourceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource_id"})
		return
	}
	startAt, err := parseRFC3339(req.StartAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_at"})
		return
	}
	endAt, err := parseRFC3339(req.EndAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_at"})
		return
	}
	out, err := h.uc.CreateBlockedRange(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.BlockedRange{
		BranchID:   branchID,
		ResourceID: resourceID,
		Kind:       schedulingdomain.BlockedRangeKind(req.Kind),
		Reason:     req.Reason,
		StartAt:    startAt,
		EndAt:      endAt,
		AllDay:     req.AllDay,
		Metadata:   req.Metadata,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) ListAvailableSlots(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	branchID, err := parseRequiredUUIDQuery(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	serviceID, err := parseRequiredUUIDQuery(c.Query("service_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service_id"})
		return
	}
	resourceID, err := parseUUIDQuery(c.Query("resource_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource_id"})
		return
	}
	day, err := parseRequiredDateQuery(c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date"})
		return
	}
	items, err := h.uc.ListAvailableSlots(c.Request.Context(), orgID, schedulingdomain.SlotQuery{
		BranchID:   branchID,
		ServiceID:  serviceID,
		Date:       day,
		ResourceID: resourceID,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) ListBookings(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	branchID, err := parseUUIDQuery(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	day, err := parseDateQuery(c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date"})
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "100")))
	items, err := h.uc.ListBookings(c.Request.Context(), orgID, schedulingdomain.ListBookingsFilter{
		BranchID: branchID,
		Date:     day,
		Status:   c.Query("status"),
		Limit:    limit,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) GetBooking(c *gin.Context) {
	orgID, bookingID, ok := authOrgAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetBookingByID(c.Request.Context(), orgID, bookingID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CreateBooking(c *gin.Context) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	var req schedulingdto.CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	branchID, err := uuid.Parse(strings.TrimSpace(req.BranchID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	serviceID, err := uuid.Parse(strings.TrimSpace(req.ServiceID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service_id"})
		return
	}
	resourceID, err := parseUUIDPtr(req.ResourceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource_id"})
		return
	}
	partyID, err := parseUUIDPtr(req.PartyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid party_id"})
		return
	}
	startAt, err := parseRFC3339(req.StartAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_at"})
		return
	}
	holdUntil, err := parseRFC3339Ptr(req.HoldUntil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid hold_until"})
		return
	}
	out, err := h.uc.CreateBooking(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.CreateBookingInput{
		BranchID:       branchID,
		ServiceID:      serviceID,
		ResourceID:     resourceID,
		PartyID:        partyID,
		CustomerName:   req.CustomerName,
		CustomerPhone:  req.CustomerPhone,
		StartAt:        startAt,
		Status:         schedulingdomain.BookingStatus(req.Status),
		Source:         schedulingdomain.BookingSource(req.Source),
		IdempotencyKey: req.IdempotencyKey,
		HoldUntil:      holdUntil,
		Notes:          req.Notes,
		Metadata:       req.Metadata,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) ConfirmBooking(c *gin.Context) {
	orgID, authCtx, bookingID, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.ConfirmBooking(c.Request.Context(), orgID, bookingID, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CancelBooking(c *gin.Context) {
	orgID, authCtx, bookingID, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	var req schedulingdto.CancelBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.CancelBooking(c.Request.Context(), orgID, bookingID, authCtx.Actor, req.Reason)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) RescheduleBooking(c *gin.Context) {
	orgID, authCtx, bookingID, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	var req schedulingdto.RescheduleBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	branchID, err := parseUUIDPtr(req.BranchID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	resourceID, err := parseUUIDPtr(req.ResourceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource_id"})
		return
	}
	startAt, err := parseRFC3339(req.StartAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_at"})
		return
	}
	payload := schedulingdomain.RescheduleBookingInput{
		BookingID: bookingID,
		StartAt:   startAt,
	}
	if branchID != nil {
		payload.BranchID = *branchID
	}
	payload.ResourceID = resourceID
	out, err := h.uc.RescheduleBooking(c.Request.Context(), orgID, authCtx.Actor, payload)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ListQueues(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	branchID, err := parseUUIDQuery(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	items, err := h.uc.ListQueues(c.Request.Context(), orgID, branchID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateQueue(c *gin.Context) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	var req schedulingdto.CreateQueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	branchID, err := uuid.Parse(strings.TrimSpace(req.BranchID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	serviceID, err := parseUUIDPtr(req.ServiceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service_id"})
		return
	}
	allowRemoteJoin := true
	if req.AllowRemoteJoin != nil {
		allowRemoteJoin = *req.AllowRemoteJoin
	}
	out, err := h.uc.CreateQueue(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.Queue{
		BranchID:         branchID,
		ServiceID:        serviceID,
		Code:             req.Code,
		Name:             req.Name,
		Status:           schedulingdomain.QueueStatus(req.Status),
		Strategy:         schedulingdomain.QueueStrategy(req.Strategy),
		TicketPrefix:     req.TicketPrefix,
		AvgServiceSecond: req.AvgServiceSeconds,
		AllowRemoteJoin:  allowRemoteJoin,
		Metadata:         req.Metadata,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) CreateQueueTicket(c *gin.Context) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	queueID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid queue id"})
		return
	}
	var req schedulingdto.CreateQueueTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	partyID, err := parseUUIDPtr(req.PartyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid party_id"})
		return
	}
	out, err := h.uc.IssueQueueTicket(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.CreateQueueTicketInput{
		QueueID:        queueID,
		PartyID:        partyID,
		CustomerName:   req.CustomerName,
		CustomerPhone:  req.CustomerPhone,
		Priority:       req.Priority,
		Source:         schedulingdomain.QueueTicketSource(req.Source),
		IdempotencyKey: req.IdempotencyKey,
		Notes:          req.Notes,
		Metadata:       req.Metadata,
	})
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) GetQueueTicketPosition(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	queueID, ticketID, ok := parseQueueTicketIDs(c)
	if !ok {
		return
	}
	out, err := h.uc.GetQueueTicketPosition(c.Request.Context(), orgID, queueID, ticketID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CallNextTicket(c *gin.Context) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	queueID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid queue id"})
		return
	}
	var req schedulingdto.TicketOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	servingResourceID, err := parseUUIDPtr(req.ServingResourceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid serving_resource_id"})
		return
	}
	operatorUserID, err := parseUUIDPtr(req.OperatorUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid operator_user_id"})
		return
	}
	out, err := h.uc.CallNextTicket(c.Request.Context(), orgID, queueID, authCtx.Actor, servingResourceID, operatorUserID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) MarkTicketServing(c *gin.Context) {
	orgID, authCtx, queueID, ticketID, req, ok := h.parseTicketOperation(c)
	if !ok {
		return
	}
	out, err := h.uc.MarkTicketServing(c.Request.Context(), orgID, queueID, ticketID, authCtx.Actor, req.servingResourceID, req.operatorUserID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CompleteTicket(c *gin.Context) {
	orgID, authCtx, queueID, ticketID, req, ok := h.parseTicketOperation(c)
	if !ok {
		return
	}
	out, err := h.uc.CompleteTicket(c.Request.Context(), orgID, queueID, ticketID, authCtx.Actor, req.servingResourceID, req.operatorUserID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) MarkTicketNoShow(c *gin.Context) {
	orgID, authCtx, queueID, ticketID, _, ok := h.parseTicketOperation(c)
	if !ok {
		return
	}
	out, err := h.uc.MarkTicketNoShow(c.Request.Context(), orgID, queueID, ticketID, authCtx.Actor)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ReassignTicket(c *gin.Context) {
	orgID, authCtx, queueID, ticketID, req, ok := h.parseTicketOperation(c)
	if !ok {
		return
	}
	out, err := h.uc.ReassignTicket(c.Request.Context(), orgID, queueID, ticketID, authCtx.Actor, req.servingResourceID, req.operatorUserID)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Dashboard(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	branchID, err := parseUUIDQuery(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	day, err := parseRequiredDateQueryWithDefault(c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date"})
		return
	}
	out, err := h.uc.Dashboard(c.Request.Context(), orgID, branchID, day)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) DayAgenda(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	branchID, err := parseUUIDQuery(c.Query("branch_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
		return
	}
	day, err := parseRequiredDateQueryWithDefault(c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date"})
		return
	}
	items, err := h.uc.DayAgenda(c.Request.Context(), orgID, branchID, day)
	if err != nil {
		httperrors.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type parsedTicketOperation struct {
	servingResourceID *uuid.UUID
	operatorUserID    *uuid.UUID
}

func (h *Handler) parseTicketOperation(c *gin.Context) (uuid.UUID, handlers.AuthContext, uuid.UUID, uuid.UUID, parsedTicketOperation, bool) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return uuid.Nil, handlers.AuthContext{}, uuid.Nil, uuid.Nil, parsedTicketOperation{}, false
	}
	queueID, ticketID, ok := parseQueueTicketIDs(c)
	if !ok {
		return uuid.Nil, handlers.AuthContext{}, uuid.Nil, uuid.Nil, parsedTicketOperation{}, false
	}
	var req schedulingdto.TicketOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return uuid.Nil, handlers.AuthContext{}, uuid.Nil, uuid.Nil, parsedTicketOperation{}, false
	}
	servingResourceID, err := parseUUIDPtr(req.ServingResourceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid serving_resource_id"})
		return uuid.Nil, handlers.AuthContext{}, uuid.Nil, uuid.Nil, parsedTicketOperation{}, false
	}
	operatorUserID, err := parseUUIDPtr(req.OperatorUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid operator_user_id"})
		return uuid.Nil, handlers.AuthContext{}, uuid.Nil, uuid.Nil, parsedTicketOperation{}, false
	}
	return orgID, authCtx, queueID, ticketID, parsedTicketOperation{servingResourceID: servingResourceID, operatorUserID: operatorUserID}, true
}

func authOrg(c *gin.Context) (uuid.UUID, bool) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, false
	}
	return orgID, true
}

func authOrgActor(c *gin.Context) (uuid.UUID, handlers.AuthContext, bool) {
	authCtx := handlers.GetAuthContext(c)
	orgID, err := uuid.Parse(authCtx.OrgID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid org"})
		return uuid.Nil, handlers.AuthContext{}, false
	}
	return orgID, authCtx, true
}

func authOrgAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := authOrg(c)
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

func authOrgActorAndID(c *gin.Context) (uuid.UUID, handlers.AuthContext, uuid.UUID, bool) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return uuid.Nil, handlers.AuthContext{}, uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return uuid.Nil, handlers.AuthContext{}, uuid.Nil, false
	}
	return orgID, authCtx, id, true
}

func parseQueueTicketIDs(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	queueID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid queue id"})
		return uuid.Nil, uuid.Nil, false
	}
	ticketID, err := uuid.Parse(strings.TrimSpace(c.Param("ticket_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return uuid.Nil, uuid.Nil, false
	}
	return queueID, ticketID, true
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

func parseRequiredUUIDQuery(raw string) (uuid.UUID, error) {
	parsed, err := parseUUIDQuery(raw)
	if err != nil {
		return uuid.Nil, err
	}
	if parsed == nil {
		return uuid.Nil, errMissingQuery
	}
	return *parsed, nil
}

func parseUUIDPtr(raw *string) (*uuid.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	return parseUUIDQuery(*raw)
}

func parseUUIDList(values []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		id, err := uuid.Parse(strings.TrimSpace(value))
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func parseRFC3339(raw string) (time.Time, error) {
	return time.Parse(time.RFC3339, strings.TrimSpace(raw))
}

func parseRFC3339Ptr(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	parsed, err := parseRFC3339(*raw)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func parseDateQuery(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func parseRequiredDateQuery(raw string) (time.Time, error) {
	parsed, err := parseDateQuery(raw)
	if err != nil {
		return time.Time{}, err
	}
	if parsed == nil {
		return time.Time{}, errMissingQuery
	}
	return *parsed, nil
}

func parseRequiredDateQueryWithDefault(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return parseRequiredDateQuery(raw)
}

func parseDatePtr(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	return parseDateQuery(*raw)
}
