package httpgin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	handlers "github.com/devpablocristo/core/http/gin/go"
	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
	schedulingdto "github.com/devpablocristo/modules/scheduling/go/httpgin/dto"
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
	UpdateBlockedRange(ctx context.Context, orgID uuid.UUID, actor string, id uuid.UUID, in schedulingdomain.BlockedRange) (schedulingdomain.BlockedRange, error)
	DeleteBlockedRange(ctx context.Context, orgID uuid.UUID, actor string, id uuid.UUID) error
	ListCalendarEvents(ctx context.Context, orgID uuid.UUID, filter schedulingdomain.ListCalendarEventsFilter) ([]schedulingdomain.CalendarEvent, error)
	GetCalendarEvent(ctx context.Context, orgID, id uuid.UUID) (schedulingdomain.CalendarEvent, error)
	CreateCalendarEvent(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.CalendarEvent) (schedulingdomain.CalendarEvent, error)
	UpdateCalendarEvent(ctx context.Context, orgID uuid.UUID, actor string, id uuid.UUID, in schedulingdomain.CalendarEvent) (schedulingdomain.CalendarEvent, error)
	DeleteCalendarEvent(ctx context.Context, orgID uuid.UUID, actor string, id uuid.UUID) error
	ListAvailableSlots(ctx context.Context, orgID uuid.UUID, query schedulingdomain.SlotQuery) ([]schedulingdomain.TimeSlot, error)
	ListBookings(ctx context.Context, orgID uuid.UUID, filter schedulingdomain.ListBookingsFilter) ([]schedulingdomain.Booking, error)
	GetBookingByID(ctx context.Context, orgID, bookingID uuid.UUID) (schedulingdomain.Booking, error)
	CreateBooking(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.CreateBookingInput) (schedulingdomain.Booking, error)
	ConfirmBooking(ctx context.Context, orgID, bookingID uuid.UUID, actor string) (schedulingdomain.Booking, error)
	CancelBooking(ctx context.Context, orgID, bookingID uuid.UUID, actor, reason string) (schedulingdomain.Booking, error)
	CheckInBooking(ctx context.Context, orgID, bookingID uuid.UUID, actor string) (schedulingdomain.Booking, error)
	StartBookingService(ctx context.Context, orgID, bookingID uuid.UUID, actor string) (schedulingdomain.Booking, error)
	CompleteBooking(ctx context.Context, orgID, bookingID uuid.UUID, actor string) (schedulingdomain.Booking, error)
	MarkBookingNoShow(ctx context.Context, orgID, bookingID uuid.UUID, actor string, reason string) (schedulingdomain.Booking, error)
	RescheduleBooking(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.RescheduleBookingInput) (schedulingdomain.Booking, error)
	ListWaitlistEntries(ctx context.Context, orgID uuid.UUID, filter schedulingdomain.ListWaitlistFilter) ([]schedulingdomain.WaitlistEntry, error)
	JoinWaitlist(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.CreateWaitlistInput) (schedulingdomain.WaitlistEntry, error)
	ListQueues(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID) ([]schedulingdomain.Queue, error)
	CreateQueue(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.Queue) (schedulingdomain.Queue, error)
	PauseQueue(ctx context.Context, orgID, queueID uuid.UUID, actor string) (schedulingdomain.Queue, error)
	ReopenQueue(ctx context.Context, orgID, queueID uuid.UUID, actor string) (schedulingdomain.Queue, error)
	CloseQueue(ctx context.Context, orgID, queueID uuid.UUID, actor string) (schedulingdomain.Queue, error)
	IssueQueueTicket(ctx context.Context, orgID uuid.UUID, actor string, in schedulingdomain.CreateQueueTicketInput) (schedulingdomain.QueueTicket, error)
	GetQueueTicketPosition(ctx context.Context, orgID, queueID, ticketID uuid.UUID) (schedulingdomain.QueuePosition, error)
	CallNextTicket(ctx context.Context, orgID, queueID uuid.UUID, actor string, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error)
	MarkTicketServing(ctx context.Context, orgID, queueID, ticketID uuid.UUID, actor string, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error)
	CompleteTicket(ctx context.Context, orgID, queueID, ticketID uuid.UUID, actor string, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error)
	MarkTicketNoShow(ctx context.Context, orgID, queueID, ticketID uuid.UUID, actor string) (schedulingdomain.QueueTicket, error)
	CancelTicket(ctx context.Context, orgID, queueID, ticketID uuid.UUID, actor string) (schedulingdomain.QueueTicket, error)
	ReassignTicket(ctx context.Context, orgID, queueID, ticketID uuid.UUID, actor string, servingResourceID, operatorUserID *uuid.UUID) (schedulingdomain.QueueTicket, error)
	ReturnTicketToWaiting(ctx context.Context, orgID, queueID, ticketID uuid.UUID, actor string) (schedulingdomain.QueueTicket, error)
	Dashboard(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, day time.Time) (schedulingdomain.DashboardStats, error)
	DayAgenda(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, day time.Time) ([]schedulingdomain.DayAgendaItem, error)
}

type Handler struct {
	uc usecasesPort
}

type PermissionBinder func(resource, action string) gin.HandlerFunc

var errMissingQuery = errors.New("required")

func NewHandler(uc usecasesPort) *Handler { return &Handler{uc: uc} }

func withPermission(bind PermissionBinder, resource, action string) gin.HandlerFunc {
	if bind == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return bind(resource, action)
}

func (h *Handler) RegisterRoutes(auth *gin.RouterGroup, require PermissionBinder) {
	auth.GET("/scheduling/branches", withPermission(require, "scheduling", "read"), h.ListBranches)
	auth.POST("/scheduling/branches", withPermission(require, "scheduling", "create"), h.CreateBranch)
	auth.GET("/scheduling/services", withPermission(require, "scheduling", "read"), h.ListServices)
	auth.POST("/scheduling/services", withPermission(require, "scheduling", "create"), h.CreateService)
	auth.GET("/scheduling/resources", withPermission(require, "scheduling", "read"), h.ListResources)
	auth.POST("/scheduling/resources", withPermission(require, "scheduling", "create"), h.CreateResource)
	auth.GET("/scheduling/availability-rules", withPermission(require, "scheduling", "read"), h.ListAvailabilityRules)
	auth.POST("/scheduling/availability-rules", withPermission(require, "scheduling", "create"), h.CreateAvailabilityRule)
	auth.GET("/scheduling/blocked-ranges", withPermission(require, "scheduling", "read"), h.ListBlockedRanges)
	auth.POST("/scheduling/blocked-ranges", withPermission(require, "scheduling", "create"), h.CreateBlockedRange)
	auth.PATCH("/scheduling/blocked-ranges/:id", withPermission(require, "scheduling", "update"), h.UpdateBlockedRange)
	auth.DELETE("/scheduling/blocked-ranges/:id", withPermission(require, "scheduling", "delete"), h.DeleteBlockedRange)
	auth.GET("/scheduling/calendar-events", withPermission(require, "scheduling", "read"), h.ListCalendarEvents)
	auth.GET("/scheduling/calendar-events/:id", withPermission(require, "scheduling", "read"), h.GetCalendarEvent)
	auth.POST("/scheduling/calendar-events", withPermission(require, "scheduling", "create"), h.CreateCalendarEvent)
	auth.PATCH("/scheduling/calendar-events/:id", withPermission(require, "scheduling", "update"), h.UpdateCalendarEvent)
	auth.DELETE("/scheduling/calendar-events/:id", withPermission(require, "scheduling", "delete"), h.DeleteCalendarEvent)
	auth.GET("/scheduling/slots", withPermission(require, "scheduling", "read"), h.ListAvailableSlots)
	auth.GET("/scheduling/bookings", withPermission(require, "scheduling", "read"), h.ListBookings)
	auth.GET("/scheduling/bookings/:id", withPermission(require, "scheduling", "read"), h.GetBooking)
	auth.POST("/scheduling/bookings", withPermission(require, "scheduling", "create"), h.CreateBooking)
	auth.POST("/scheduling/bookings/:id/confirm", withPermission(require, "scheduling", "update"), h.ConfirmBooking)
	auth.POST("/scheduling/bookings/:id/cancel", withPermission(require, "scheduling", "update"), h.CancelBooking)
	auth.POST("/scheduling/bookings/:id/check-in", withPermission(require, "scheduling", "operate"), h.CheckInBooking)
	auth.POST("/scheduling/bookings/:id/start-service", withPermission(require, "scheduling", "operate"), h.StartBookingService)
	auth.POST("/scheduling/bookings/:id/complete", withPermission(require, "scheduling", "operate"), h.CompleteBookingLifecycle)
	auth.POST("/scheduling/bookings/:id/no-show", withPermission(require, "scheduling", "operate"), h.MarkBookingNoShow)
	auth.POST("/scheduling/bookings/:id/reschedule", withPermission(require, "scheduling", "update"), h.RescheduleBooking)
	auth.GET("/scheduling/waitlist", withPermission(require, "scheduling", "read"), h.ListWaitlistEntries)
	auth.POST("/scheduling/waitlist", withPermission(require, "scheduling", "create"), h.CreateWaitlistEntry)
	auth.GET("/scheduling/queues", withPermission(require, "scheduling", "read"), h.ListQueues)
	auth.POST("/scheduling/queues", withPermission(require, "scheduling", "create"), h.CreateQueue)
	auth.POST("/scheduling/queues/:id/pause", withPermission(require, "scheduling", "operate"), h.PauseQueue)
	auth.POST("/scheduling/queues/:id/reopen", withPermission(require, "scheduling", "operate"), h.ReopenQueue)
	auth.POST("/scheduling/queues/:id/close", withPermission(require, "scheduling", "operate"), h.CloseQueue)
	auth.POST("/scheduling/queues/:id/tickets", withPermission(require, "scheduling", "create"), h.CreateQueueTicket)
	auth.GET("/scheduling/queues/:id/tickets/:ticket_id/position", withPermission(require, "scheduling", "read"), h.GetQueueTicketPosition)
	auth.POST("/scheduling/queues/:id/next", withPermission(require, "scheduling", "operate"), h.CallNextTicket)
	auth.POST("/scheduling/queues/:id/tickets/:ticket_id/serve", withPermission(require, "scheduling", "operate"), h.MarkTicketServing)
	auth.POST("/scheduling/queues/:id/tickets/:ticket_id/complete", withPermission(require, "scheduling", "operate"), h.CompleteTicket)
	auth.POST("/scheduling/queues/:id/tickets/:ticket_id/no-show", withPermission(require, "scheduling", "operate"), h.MarkTicketNoShow)
	auth.POST("/scheduling/queues/:id/tickets/:ticket_id/cancel", withPermission(require, "scheduling", "operate"), h.CancelTicket)
	auth.POST("/scheduling/queues/:id/tickets/:ticket_id/reassign", withPermission(require, "scheduling", "operate"), h.ReassignTicket)
	auth.POST("/scheduling/queues/:id/tickets/:ticket_id/return-to-waiting", withPermission(require, "scheduling", "operate"), h.ReturnTicketToWaiting)
	auth.GET("/scheduling/dashboard", withPermission(require, "scheduling", "read"), h.Dashboard)
	auth.GET("/scheduling/day", withPermission(require, "scheduling", "read"), h.DayAgenda)
}

func (h *Handler) ListBranches(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
	items, err := h.uc.ListBranches(c.Request.Context(), orgID)
	if err != nil {
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
	commercialServiceID, err := parseUUIDPtr(req.CommercialServiceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid commercial_service_id"})
		return
	}
	out, err := h.uc.CreateService(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.Service{
		CommercialServiceID:    commercialServiceID,
		Code:                   req.Code,
		Name:                   req.Name,
		Description:            req.Description,
		FulfillmentMode:        schedulingdomain.FulfillmentMode(req.FulfillmentMode),
		DefaultDurationMinutes: req.DefaultDurationMinutes,
		BufferBeforeMinutes:    req.BufferBeforeMinutes,
		BufferAfterMinutes:     req.BufferAfterMinutes,
		SlotGranularityMinutes: req.SlotGranularityMinutes,
		MaxConcurrentBookings:  req.MaxConcurrentBookings,
		MinCancelNoticeMinutes: req.MinCancelNoticeMinutes,
		AllowWaitlist:          req.AllowWaitlist != nil && *req.AllowWaitlist,
		Active:                 active,
		ResourceIDs:            resourceIDs,
		Metadata:               req.Metadata,
	})
	if err != nil {
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) UpdateBlockedRange(c *gin.Context) {
	orgID, authCtx, id, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	var req schedulingdto.UpdateBlockedRangeRequest
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
	out, err := h.uc.UpdateBlockedRange(c.Request.Context(), orgID, authCtx.Actor, id, schedulingdomain.BlockedRange{
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
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) DeleteBlockedRange(c *gin.Context) {
	orgID, authCtx, id, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	if err := h.uc.DeleteBlockedRange(c.Request.Context(), orgID, authCtx.Actor, id); err != nil {
		handlers.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── calendar events (agenda interna) ────────────────────────────────────────

func (h *Handler) ListCalendarEvents(c *gin.Context) {
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
	from, err := parseRFC3339Query(c.Query("from"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"})
		return
	}
	to, err := parseRFC3339Query(c.Query("to"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"})
		return
	}
	filter := schedulingdomain.ListCalendarEventsFilter{
		BranchID:   branchID,
		ResourceID: resourceID,
		From:       from,
		To:         to,
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		s := schedulingdomain.CalendarEventStatus(strings.ToLower(status))
		filter.Status = &s
	}
	items, err := h.uc.ListCalendarEvents(c.Request.Context(), orgID, filter)
	if err != nil {
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) GetCalendarEvent(c *gin.Context) {
	orgID, _, id, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.GetCalendarEvent(c.Request.Context(), orgID, id)
	if err != nil {
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CreateCalendarEvent(c *gin.Context) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	var req schedulingdto.CreateCalendarEventRequest
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
	endAt, err := parseRFC3339(req.EndAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_at"})
		return
	}
	out, err := h.uc.CreateCalendarEvent(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.CalendarEvent{
		BranchID:    branchID,
		ResourceID:  resourceID,
		Title:       req.Title,
		Description: req.Description,
		StartAt:     startAt,
		EndAt:       endAt,
		AllDay:      req.AllDay,
		Status:      schedulingdomain.CalendarEventStatus(req.Status),
		Visibility:  schedulingdomain.CalendarEventVisibility(req.Visibility),
		Metadata:    req.Metadata,
	})
	if err != nil {
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) UpdateCalendarEvent(c *gin.Context) {
	orgID, authCtx, id, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	var req schedulingdto.UpdateCalendarEventRequest
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
	endAt, err := parseRFC3339(req.EndAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_at"})
		return
	}
	out, err := h.uc.UpdateCalendarEvent(c.Request.Context(), orgID, authCtx.Actor, id, schedulingdomain.CalendarEvent{
		BranchID:    branchID,
		ResourceID:  resourceID,
		Title:       req.Title,
		Description: req.Description,
		StartAt:     startAt,
		EndAt:       endAt,
		AllDay:      req.AllDay,
		Status:      schedulingdomain.CalendarEventStatus(req.Status),
		Visibility:  schedulingdomain.CalendarEventVisibility(req.Visibility),
		Metadata:    req.Metadata,
	})
	if err != nil {
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) DeleteCalendarEvent(c *gin.Context) {
	orgID, authCtx, id, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	if err := h.uc.DeleteCalendarEvent(c.Request.Context(), orgID, authCtx.Actor, id); err != nil {
		handlers.Respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
	endAt, err := parseRFC3339Ptr(req.EndAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_at"})
		return
	}
	holdUntil, err := parseRFC3339Ptr(req.HoldUntil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid hold_until"})
		return
	}
	var recurrence *schedulingdomain.BookingRecurrence
	if req.Recurrence != nil {
		recurrence = &schedulingdomain.BookingRecurrence{
			Freq:      req.Recurrence.Freq,
			Interval:  req.Recurrence.Interval,
			Count:     req.Recurrence.Count,
			ByWeekday: req.Recurrence.ByWeekday,
		}
		if strings.TrimSpace(req.Recurrence.Until) != "" {
			until, err := parseRFC3339(req.Recurrence.Until)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recurrence.until"})
				return
			}
			recurrence.Until = &until
		}
	}
	out, err := h.uc.CreateBooking(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.CreateBookingInput{
		BranchID:       branchID,
		ServiceID:      serviceID,
		ResourceID:     resourceID,
		PartyID:        partyID,
		CustomerName:   req.CustomerName,
		CustomerPhone:  req.CustomerPhone,
		CustomerEmail:  req.CustomerEmail,
		StartAt:        startAt,
		EndAt:          endAt,
		Status:         schedulingdomain.BookingStatus(req.Status),
		Source:         schedulingdomain.BookingSource(req.Source),
		IdempotencyKey: req.IdempotencyKey,
		HoldUntil:      holdUntil,
		Notes:          req.Notes,
		Metadata:       req.Metadata,
		Recurrence:     recurrence,
	})
	if err != nil {
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CheckInBooking(c *gin.Context) {
	orgID, authCtx, bookingID, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.CheckInBooking(c.Request.Context(), orgID, bookingID, authCtx.Actor)
	if err != nil {
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) StartBookingService(c *gin.Context) {
	orgID, authCtx, bookingID, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.StartBookingService(c.Request.Context(), orgID, bookingID, authCtx.Actor)
	if err != nil {
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CompleteBookingLifecycle(c *gin.Context) {
	orgID, authCtx, bookingID, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	out, err := h.uc.CompleteBooking(c.Request.Context(), orgID, bookingID, authCtx.Actor)
	if err != nil {
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) MarkBookingNoShow(c *gin.Context) {
	orgID, authCtx, bookingID, ok := authOrgActorAndID(c)
	if !ok {
		return
	}
	var req schedulingdto.CancelBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.uc.MarkBookingNoShow(c.Request.Context(), orgID, bookingID, authCtx.Actor, req.Reason)
	if err != nil {
		handlers.Respond(c, err)
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
	endAt, err := parseRFC3339Ptr(req.EndAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_at"})
		return
	}
	payload := schedulingdomain.RescheduleBookingInput{
		BookingID: bookingID,
		StartAt:   startAt,
		EndAt:     endAt,
	}
	if branchID != nil {
		payload.BranchID = *branchID
	}
	payload.ResourceID = resourceID
	out, err := h.uc.RescheduleBooking(c.Request.Context(), orgID, authCtx.Actor, payload)
	if err != nil {
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ListWaitlistEntries(c *gin.Context) {
	orgID, ok := authOrg(c)
	if !ok {
		return
	}
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
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "100")))
	items, err := h.uc.ListWaitlistEntries(c.Request.Context(), orgID, schedulingdomain.ListWaitlistFilter{
		BranchID:  branchID,
		ServiceID: serviceID,
		Status:    c.Query("status"),
		Limit:     limit,
	})
	if err != nil {
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CreateWaitlistEntry(c *gin.Context) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	var req schedulingdto.CreateWaitlistRequest
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
	requestedStartAt, err := parseRFC3339(req.RequestedStartAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid requested_start_at"})
		return
	}
	out, err := h.uc.JoinWaitlist(c.Request.Context(), orgID, authCtx.Actor, schedulingdomain.CreateWaitlistInput{
		BranchID:         branchID,
		ServiceID:        serviceID,
		ResourceID:       resourceID,
		PartyID:          partyID,
		CustomerName:     req.CustomerName,
		CustomerPhone:    req.CustomerPhone,
		CustomerEmail:    req.CustomerEmail,
		RequestedStartAt: requestedStartAt,
		Source:           schedulingdomain.WaitlistSource(req.Source),
		IdempotencyKey:   req.IdempotencyKey,
		Notes:            req.Notes,
		Metadata:         req.Metadata,
	})
	if err != nil {
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) PauseQueue(c *gin.Context) {
	h.transitionQueueStatus(c, func(ctx context.Context, orgID, queueID uuid.UUID, actor string) (schedulingdomain.Queue, error) {
		return h.uc.PauseQueue(ctx, orgID, queueID, actor)
	})
}

func (h *Handler) ReopenQueue(c *gin.Context) {
	h.transitionQueueStatus(c, func(ctx context.Context, orgID, queueID uuid.UUID, actor string) (schedulingdomain.Queue, error) {
		return h.uc.ReopenQueue(ctx, orgID, queueID, actor)
	})
}

func (h *Handler) CloseQueue(c *gin.Context) {
	h.transitionQueueStatus(c, func(ctx context.Context, orgID, queueID uuid.UUID, actor string) (schedulingdomain.Queue, error) {
		return h.uc.CloseQueue(ctx, orgID, queueID, actor)
	})
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
		CustomerEmail:  req.CustomerEmail,
		Priority:       req.Priority,
		Source:         schedulingdomain.QueueTicketSource(req.Source),
		IdempotencyKey: req.IdempotencyKey,
		Notes:          req.Notes,
		Metadata:       req.Metadata,
	})
	if err != nil {
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) CancelTicket(c *gin.Context) {
	orgID, authCtx, queueID, ticketID, _, ok := h.parseTicketOperation(c)
	if !ok {
		return
	}
	out, err := h.uc.CancelTicket(c.Request.Context(), orgID, queueID, ticketID, authCtx.Actor)
	if err != nil {
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) ReturnTicketToWaiting(c *gin.Context) {
	orgID, authCtx, queueID, ticketID, _, ok := h.parseTicketOperation(c)
	if !ok {
		return
	}
	out, err := h.uc.ReturnTicketToWaiting(c.Request.Context(), orgID, queueID, ticketID, authCtx.Actor)
	if err != nil {
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
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
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type parsedTicketOperation struct {
	servingResourceID *uuid.UUID
	operatorUserID    *uuid.UUID
}

type queueTransitionFunc func(ctx context.Context, orgID, queueID uuid.UUID, actor string) (schedulingdomain.Queue, error)

func (h *Handler) transitionQueueStatus(c *gin.Context, fn queueTransitionFunc) {
	orgID, authCtx, ok := authOrgActor(c)
	if !ok {
		return
	}
	queueID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid queue id"})
		return
	}
	out, err := fn(c.Request.Context(), orgID, queueID, authCtx.Actor)
	if err != nil {
		handlers.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
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

func parseRFC3339Query(raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := parseRFC3339(trimmed)
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
