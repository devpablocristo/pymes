// architecture:adapter handler
package scheduling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	dto "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/handler/dto"
	httphelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/handler/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Principal = domain.Principal

type Authenticator interface {
	Principal(*http.Request) (domain.Principal, error)
}

type FeatureGate interface {
	Enabled(context.Context, string, string) (bool, error)
}

// SchedulingUsecases is the HTTP adapter-owned input port.
type SchedulingUsecases interface {
	CreateBranch(context.Context, domain.Branch) (domain.Branch, error)
	CreateService(context.Context, domain.Service, []domain.ResourceRequirement) (domain.Service, error)
	CreateResource(context.Context, domain.Resource) (domain.Resource, error)
	CreateAvailabilityRule(context.Context, domain.AvailabilityRule) (domain.AvailabilityRule, error)
	CreateAvailabilityException(context.Context, domain.AvailabilityException) (domain.AvailabilityException, error)
	ListBranches(context.Context, string) ([]domain.Branch, error)
	ListServices(context.Context, string) ([]domain.Service, error)
	ListResources(context.Context, string, uuid.UUID) ([]domain.Resource, error)
	ListAvailabilityRules(context.Context, string, uuid.UUID) ([]domain.AvailabilityRule, error)
	ListAvailabilityExceptions(context.Context, string, uuid.UUID, time.Time, time.Time) ([]domain.AvailabilityException, error)
	PublicCatalog(context.Context, string) (PublicCatalog, error)
	AvailableSlots(context.Context, domain.AvailabilityQuery) ([]domain.Slot, error)
	CreateBooking(context.Context, domain.CommandMetadata, CreateBookingInput) ([]domain.Booking, error)
	CreateGroupSession(context.Context, domain.CommandMetadata, CreateGroupSessionInput) (domain.GroupSession, error)
	GetBooking(context.Context, string, uuid.UUID) (domain.Booking, error)
	ListBookings(context.Context, string, uuid.UUID, time.Time, time.Time) ([]domain.Booking, error)
	UpdateBooking(context.Context, domain.CommandMetadata, UpdateBookingInput) (domain.Booking, error)
	RescheduleBooking(context.Context, domain.CommandMetadata, RescheduleInput) (domain.Booking, error)
	TransitionBooking(context.Context, domain.CommandMetadata, string, uuid.UUID, int, domain.BookingStatus, string) (domain.Booking, error)
	ConfigureBookingStatus(context.Context, domain.CommandMetadata, domain.BookingStatusConfiguration) (domain.BookingStatusConfiguration, error)
	ListBookingStatusConfigurations(context.Context, string) ([]domain.BookingStatusConfiguration, error)
	SetBookingSubstate(context.Context, domain.CommandMetadata, string, uuid.UUID, int, string) (domain.Booking, error)
	CreateWaitlistEntry(context.Context, domain.CommandMetadata, CreateWaitlistInput) (domain.WaitlistEntry, error)
	ListWaitlist(context.Context, string, uuid.UUID) ([]domain.WaitlistEntry, error)
	CreateQueueTicket(context.Context, domain.CommandMetadata, domain.QueueTicket) (domain.QueueTicket, error)
	AdvanceQueueTicket(context.Context, domain.CommandMetadata, string, uuid.UUID, int, domain.QueueStatus) (domain.QueueTicket, error)
	ListQueue(context.Context, string, uuid.UUID) ([]domain.QueueTicket, error)
	ResolvePublicOrganization(context.Context, string) (string, error)
	ResolveActionOrganization(context.Context, string) (string, error)
	ConsumeBookingAction(context.Context, string, domain.ActionPurpose, domain.CommandMetadata, int, *time.Time, int, string) (domain.Booking, error)
	ConsumeWaitlistAction(context.Context, string, domain.CommandMetadata, int) (domain.WaitlistEntry, error)
}

type HTTPHandler struct {
	usecases SchedulingUsecases
	auth     Authenticator
	features FeatureGate
}

func NewHTTPHandler(
	usecases SchedulingUsecases,
	auth Authenticator,
	features FeatureGate,
) *HTTPHandler {
	return &HTTPHandler{usecases: usecases, auth: auth, features: features}
}

func (h *HTTPHandler) Handler() http.Handler {
	router := chi.NewRouter()
	router.Route("/api/v1/organizations/{organizationId}/scheduling", func(router chi.Router) {
		router.Post("/branches", h.createBranch)
		router.Get("/branches", h.listBranches)
		router.Post("/services", h.createService)
		router.Get("/services", h.listServices)
		router.Post("/resources", h.createResource)
		router.Get("/resources", h.listResources)
		router.Post("/availability", h.adminAvailability)
		router.Post("/availability/rules", h.createAvailabilityRule)
		router.Get("/availability/rules", h.listAvailabilityRules)
		router.Post("/blocks", h.createBlock)
		router.Get("/blocks", h.listBlocks)
		router.Post("/sessions", h.createGroupSession)
		router.Put("/status-configurations/{status}", h.configureBookingStatus)
		router.Get("/status-configurations", h.listBookingStatusConfigurations)
		router.Post("/bookings", h.createAdminBooking)
		router.Get("/bookings", h.listBookings)
		router.Get("/bookings/{bookingId}", h.getBooking)
		router.Patch("/bookings/{bookingId}", h.updateBooking)
		router.Post("/bookings/{bookingId}/confirm", h.transition(domain.BookingConfirmed, domain.PermissionOperate))
		router.Post("/bookings/{bookingId}/cancel", h.transition(domain.BookingCancelled, domain.PermissionOperate))
		router.Post("/bookings/{bookingId}/check-in", h.transition(domain.BookingCheckedIn, domain.PermissionOperate))
		router.Post("/bookings/{bookingId}/complete", h.transition(domain.BookingCompleted, domain.PermissionOperate))
		router.Post("/bookings/{bookingId}/no-show", h.transition(domain.BookingNoShow, domain.PermissionOperate))
		router.Post("/bookings/{bookingId}/reschedule", h.rescheduleBooking)
		router.Post("/bookings/{bookingId}/substate", h.setBookingSubstate)
		router.Post("/waitlist", h.createAdminWaitlist)
		router.Get("/waitlist", h.listWaitlist)
		router.Post("/queue", h.createQueueTicket)
		router.Get("/queue", h.listQueue)
		router.Post("/queue/{ticketId}", h.advanceQueueTicket)
	})
	router.Post("/api/v1/public/scheduling/{organizationSlug}/availability", h.publicAvailability)
	router.Get("/api/v1/public/scheduling/{organizationSlug}/catalog", h.publicCatalog)
	router.Post("/api/v1/public/scheduling/{organizationSlug}/bookings", h.createPublicBooking)
	router.Post("/api/v1/public/scheduling/{organizationSlug}/waitlist", h.createPublicWaitlist)
	router.Post("/api/v1/public/scheduling/actions/{token}", h.consumePublicAction)
	return router
}

func (h *HTTPHandler) createBranch(w http.ResponseWriter, request *http.Request) {
	organizationID, principal, ok := h.authorize(w, request, domain.PermissionManage)
	if !ok {
		return
	}
	var input dto.CreateBranch
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	if input.ID == uuid.Nil {
		input.ID = uuid.New()
	}
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	result, err := h.usecases.CreateBranch(request.Context(), domain.Branch{
		OrganizationID: organizationID, ID: input.ID, Code: input.Code, Slug: input.Slug,
		Name: input.Name, Timezone: input.Timezone, Address: input.Address, Active: active,
	})
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	_ = principal
	httphelpers.WriteJSON(w, http.StatusCreated, dto.BranchFromDomain(result))
}

func (h *HTTPHandler) listBranches(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionRead)
	if !ok {
		return
	}
	result, err := h.usecases.ListBranches(request.Context(), organizationID)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.BranchesFromDomain(result))
}

func (h *HTTPHandler) createService(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionManage)
	if !ok {
		return
	}
	var input dto.CreateService
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	if input.ID == uuid.Nil {
		input.ID = uuid.New()
	}
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	requirements := make([]domain.ResourceRequirement, 0, len(input.Requirements))
	for _, requirement := range input.Requirements {
		requirements = append(requirements, domain.ResourceRequirement{
			OrganizationID: organizationID, ID: requirement.ID, ServiceID: input.ID,
			ResourceID: requirement.ResourceID, Kind: requirement.Kind, Mode: requirement.Mode,
			Units: requirement.Units, Optional: requirement.Optional,
		})
	}
	result, err := h.usecases.CreateService(request.Context(), domain.Service{
		OrganizationID: organizationID, ID: input.ID, Code: input.Code, Name: input.Name,
		Description: input.Description, DurationMinutes: input.DurationMinutes,
		BufferBeforeMinutes: input.BufferBeforeMinutes, BufferAfterMinutes: input.BufferAfterMinutes,
		SlotMinutes: input.SlotMinutes, Price: input.Price, Currency: input.Currency,
		Mode: input.FulfillmentMode, MaxParticipants: input.MaxParticipants,
		AllowGroup: input.AllowGroup, AllowWaitlist: input.AllowWaitlist,
		ConfirmationRequired: input.ConfirmationRequired, Active: active,
	}, requirements)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusCreated, dto.ServiceFromDomain(result))
}

func (h *HTTPHandler) listServices(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionRead)
	if !ok {
		return
	}
	result, err := h.usecases.ListServices(request.Context(), organizationID)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.ServicesFromDomain(result))
}

func (h *HTTPHandler) createResource(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionManage)
	if !ok {
		return
	}
	var input dto.CreateResource
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	if input.ID == uuid.Nil {
		input.ID = uuid.New()
	}
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	result, err := h.usecases.CreateResource(request.Context(), domain.Resource{
		OrganizationID: organizationID, ID: input.ID, BranchID: input.BranchID,
		Code: input.Code, Name: input.Name, Kind: input.Kind, Capacity: input.Capacity,
		Timezone: input.Timezone, Active: active,
	})
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusCreated, dto.ResourceFromDomain(result))
}

func (h *HTTPHandler) listResources(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionRead)
	if !ok {
		return
	}
	branchID := uuid.Nil
	if raw := strings.TrimSpace(request.URL.Query().Get("branch_id")); raw != "" {
		var parsed bool
		branchID, parsed = httphelpers.ParseUUID(w, raw)
		if !parsed {
			return
		}
	}
	result, err := h.usecases.ListResources(request.Context(), organizationID, branchID)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.ResourcesFromDomain(result))
}

func (h *HTTPHandler) createAvailabilityRule(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionManage)
	if !ok {
		return
	}
	var input dto.CreateAvailabilityRule
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	if input.ID == uuid.Nil {
		input.ID = uuid.New()
	}
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	result, err := h.usecases.CreateAvailabilityRule(request.Context(), domain.AvailabilityRule{
		OrganizationID: organizationID, ID: input.ID, BranchID: input.BranchID,
		ResourceID: input.ResourceID, Kind: input.Kind, Weekday: time.Weekday(input.Weekday),
		StartMinute: input.StartMinute, EndMinute: input.EndMinute,
		ValidFrom: input.ValidFrom, ValidUntil: input.ValidUntil,
		Timezone: input.Timezone, Active: active,
	})
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusCreated, dto.AvailabilityRuleFromDomain(result))
}

func (h *HTTPHandler) listAvailabilityRules(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionRead)
	if !ok {
		return
	}
	branchID, ok := httphelpers.ParseUUID(w, request.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	result, err := h.usecases.ListAvailabilityRules(request.Context(), organizationID, branchID)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.AvailabilityRulesFromDomain(result))
}

func (h *HTTPHandler) createBlock(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionManage)
	if !ok {
		return
	}
	var input dto.CreateBlock
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	if input.ID == uuid.Nil {
		input.ID = uuid.New()
	}
	result, err := h.usecases.CreateAvailabilityException(request.Context(), domain.AvailabilityException{
		OrganizationID: organizationID, ID: input.ID, BranchID: input.BranchID,
		ResourceID: input.ResourceID, Kind: input.Kind, StartAt: input.StartAt.UTC(),
		EndAt: input.EndAt.UTC(), Reason: input.Reason,
	})
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusCreated, dto.AvailabilityExceptionFromDomain(result))
}

func (h *HTTPHandler) listBlocks(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionRead)
	if !ok {
		return
	}
	branchID, ok := httphelpers.ParseUUID(w, request.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	from, ok := httphelpers.ParseTime(w, request.URL.Query().Get("from"))
	if !ok {
		return
	}
	until, ok := httphelpers.ParseTime(w, request.URL.Query().Get("until"))
	if !ok {
		return
	}
	result, err := h.usecases.ListAvailabilityExceptions(
		request.Context(), organizationID, branchID, from, until,
	)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.AvailabilityExceptionsFromDomain(result))
}

func (h *HTTPHandler) adminAvailability(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionRead)
	if !ok {
		return
	}
	h.availability(w, request, organizationID)
}

func (h *HTTPHandler) publicAvailability(w http.ResponseWriter, request *http.Request) {
	organizationID, err := h.usecases.ResolvePublicOrganization(
		request.Context(), chi.URLParam(request, "organizationSlug"),
	)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	if !h.requireEnabled(w, request, organizationID) {
		return
	}
	h.availability(w, request, organizationID)
}

func (h *HTTPHandler) publicCatalog(w http.ResponseWriter, request *http.Request) {
	organizationID, err := h.usecases.ResolvePublicOrganization(
		request.Context(), chi.URLParam(request, "organizationSlug"),
	)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	if !h.requireEnabled(w, request, organizationID) {
		return
	}
	result, err := h.usecases.PublicCatalog(request.Context(), organizationID)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.PublicCatalogFromDomain(
		result.Branches,
		result.Services,
		result.Resources,
	))
}

func (h *HTTPHandler) availability(w http.ResponseWriter, request *http.Request, organizationID string) {
	var input dto.AvailabilityQuery
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	result, err := h.usecases.AvailableSlots(request.Context(), domain.AvailabilityQuery{
		OrganizationID: organizationID, BranchID: input.BranchID, ServiceID: input.ServiceID,
		From: input.From.UTC(), Until: input.Until.UTC(), Participants: input.Participants,
		Allocations: dto.Allocations(input.Allocations),
	})
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.SlotsFromDomain(result))
}

func (h *HTTPHandler) createGroupSession(w http.ResponseWriter, request *http.Request) {
	organizationID, principal, ok := h.authorize(w, request, domain.PermissionManage)
	if !ok {
		return
	}
	var input dto.CreateGroupSession
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	metadata, ok := commandMetadata(w, request, organizationID, principal.ActorID, input)
	if !ok {
		return
	}
	result, err := h.usecases.CreateGroupSession(request.Context(), metadata, CreateGroupSessionInput{
		OrganizationID: organizationID, BranchID: input.BranchID, ServiceID: input.ServiceID,
		StartAt: input.StartAt.UTC(), Capacity: input.Capacity,
		Allocations: dto.Allocations(input.Allocations),
	})
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusCreated, dto.GroupSessionFromDomain(result))
}

func (h *HTTPHandler) createAdminBooking(w http.ResponseWriter, request *http.Request) {
	organizationID, principal, ok := h.authorize(w, request, domain.PermissionOperate)
	if !ok {
		return
	}
	h.createBooking(w, request, organizationID, principal.ActorID, false)
}

func (h *HTTPHandler) createPublicBooking(w http.ResponseWriter, request *http.Request) {
	organizationID, err := h.usecases.ResolvePublicOrganization(
		request.Context(), chi.URLParam(request, "organizationSlug"),
	)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	if !h.requireEnabled(w, request, organizationID) {
		return
	}
	h.createBooking(w, request, organizationID, "public:scheduling", true)
}

func (h *HTTPHandler) createBooking(
	w http.ResponseWriter,
	request *http.Request,
	organizationID, actorID string,
	public bool,
) {
	var input dto.CreateBooking
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	if public && input.Status != "" {
		httphelpers.WriteProblem(
			w,
			http.StatusBadRequest,
			domain.CodeValidation,
			"public bookings cannot select an internal status",
		)
		return
	}
	if input.ID == uuid.Nil {
		input.ID = uuid.New()
	}
	metadata, ok := commandMetadata(w, request, organizationID, actorID, input)
	if !ok {
		return
	}
	result, err := h.usecases.CreateBooking(request.Context(), metadata, CreateBookingInput{
		OrganizationID: organizationID, BranchID: input.BranchID, ServiceID: input.ServiceID,
		SessionID: input.SessionID, PartyID: input.Customer.PartyID,
		Customer: PublicCustomer{
			PartyID: input.Customer.PartyID, Name: input.Customer.Name,
			Email: input.Customer.Email, Phone: input.Customer.Phone,
		},
		StartAt: input.StartAt.UTC(), Participants: input.Participants, Status: input.Status,
		HoldFor:     time.Duration(input.HoldMinutes) * time.Minute,
		Allocations: dto.Allocations(input.Allocations), MeetRequested: input.MeetRequested,
		Notes:      input.Notes,
		Recurrence: input.Recurrence.Domain(),
	})
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	if public {
		httphelpers.WriteJSON(w, http.StatusCreated, dto.PublicBookingsFromDomain(result))
		return
	}
	httphelpers.WriteJSON(w, http.StatusCreated, dto.BookingsFromDomain(result))
}

func (h *HTTPHandler) getBooking(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionRead)
	if !ok {
		return
	}
	id, ok := httphelpers.ParseUUID(w, chi.URLParam(request, "bookingId"))
	if !ok {
		return
	}
	result, err := h.usecases.GetBooking(request.Context(), organizationID, id)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.BookingFromDomain(result))
}

func (h *HTTPHandler) listBookings(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionRead)
	if !ok {
		return
	}
	branchID, ok := httphelpers.ParseUUID(w, request.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	from, ok := httphelpers.ParseTime(w, request.URL.Query().Get("from"))
	if !ok {
		return
	}
	until, ok := httphelpers.ParseTime(w, request.URL.Query().Get("until"))
	if !ok {
		return
	}
	result, err := h.usecases.ListBookings(request.Context(), organizationID, branchID, from, until)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.BookingsFromDomain(result))
}

func (h *HTTPHandler) updateBooking(w http.ResponseWriter, request *http.Request) {
	organizationID, principal, ok := h.authorize(w, request, domain.PermissionOperate)
	if !ok {
		return
	}
	bookingID, ok := httphelpers.ParseUUID(w, chi.URLParam(request, "bookingId"))
	if !ok {
		return
	}
	var input dto.UpdateBooking
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	metadata, ok := commandMetadata(
		w,
		request,
		organizationID,
		principal.ActorID,
		dto.UpdateBookingIdempotencyScope{
			BookingID: bookingID,
			Body:      input,
		},
	)
	if !ok {
		return
	}
	var customer *PublicCustomer
	if input.Customer != nil {
		customer = &PublicCustomer{
			PartyID: input.Customer.PartyID,
			Name:    input.Customer.Name,
			Email:   input.Customer.Email,
			Phone:   input.Customer.Phone,
		}
	}
	result, err := h.usecases.UpdateBooking(request.Context(), metadata, UpdateBookingInput{
		OrganizationID:  organizationID,
		BookingID:       bookingID,
		ExpectedVersion: input.ExpectedVersion,
		Customer:        customer,
		Participants:    input.Participants,
		Notes:           input.Notes,
		SubstateCode:    input.SubstateCode,
	})
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.BookingFromDomain(result))
}

func (h *HTTPHandler) rescheduleBooking(w http.ResponseWriter, request *http.Request) {
	organizationID, principal, ok := h.authorize(w, request, domain.PermissionOperate)
	if !ok {
		return
	}
	id, ok := httphelpers.ParseUUID(w, chi.URLParam(request, "bookingId"))
	if !ok {
		return
	}
	var input dto.Reschedule
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	metadata, ok := commandMetadata(w, request, organizationID, principal.ActorID, input)
	if !ok {
		return
	}
	result, err := h.usecases.RescheduleBooking(request.Context(), metadata, RescheduleInput{
		OrganizationID: organizationID, BookingID: id, ExpectedVersion: input.ExpectedVersion,
		StartAt: input.StartAt.UTC(), DurationMinutes: input.DurationMinutes,
		Allocations: dto.Allocations(input.Allocations),
	})
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.BookingFromDomain(result))
}

func (h *HTTPHandler) transition(
	status domain.BookingStatus,
	permission string,
) http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {
		organizationID, principal, ok := h.authorize(w, request, permission)
		if !ok {
			return
		}
		id, ok := httphelpers.ParseUUID(w, chi.URLParam(request, "bookingId"))
		if !ok {
			return
		}
		var input dto.Transition
		if !httphelpers.Decode(w, request, &input) {
			return
		}
		metadata, ok := commandMetadata(w, request, organizationID, principal.ActorID, input)
		if !ok {
			return
		}
		result, err := h.usecases.TransitionBooking(
			request.Context(), metadata, organizationID, id, input.ExpectedVersion, status, input.Reason,
		)
		if err != nil {
			httphelpers.WriteError(w, err)
			return
		}
		httphelpers.WriteJSON(w, http.StatusOK, dto.BookingFromDomain(result))
	}
}

func (h *HTTPHandler) configureBookingStatus(w http.ResponseWriter, request *http.Request) {
	organizationID, principal, ok := h.authorize(w, request, domain.PermissionManage)
	if !ok {
		return
	}
	status := domain.BookingStatus(chi.URLParam(request, "status"))
	var input dto.ConfigureBookingStatus
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	metadata, ok := commandMetadata(
		w,
		request,
		organizationID,
		principal.ActorID,
		map[string]any{"status": status, "configuration": input},
	)
	if !ok {
		return
	}
	substates := make([]domain.BookingSubstateDefinition, 0, len(input.Substates))
	for _, value := range input.Substates {
		substates = append(substates, domain.BookingSubstateDefinition{
			Code: value.Code, Label: value.Label, Active: value.Active, SortOrder: value.SortOrder,
		})
	}
	result, err := h.usecases.ConfigureBookingStatus(
		request.Context(),
		metadata,
		domain.BookingStatusConfiguration{
			OrganizationID: organizationID,
			Status:         status,
			Label:          input.Label,
			Substates:      substates,
		},
	)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.BookingStatusConfigurationFromDomain(result))
}

func (h *HTTPHandler) listBookingStatusConfigurations(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionRead)
	if !ok {
		return
	}
	result, err := h.usecases.ListBookingStatusConfigurations(request.Context(), organizationID)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.BookingStatusConfigurationsFromDomain(result))
}

func (h *HTTPHandler) setBookingSubstate(w http.ResponseWriter, request *http.Request) {
	organizationID, principal, ok := h.authorize(w, request, domain.PermissionOperate)
	if !ok {
		return
	}
	bookingID, ok := httphelpers.ParseUUID(w, chi.URLParam(request, "bookingId"))
	if !ok {
		return
	}
	var input dto.SetBookingSubstate
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	metadata, ok := commandMetadata(
		w,
		request,
		organizationID,
		principal.ActorID,
		map[string]any{"booking_id": bookingID, "substate": input},
	)
	if !ok {
		return
	}
	result, err := h.usecases.SetBookingSubstate(
		request.Context(),
		metadata,
		organizationID,
		bookingID,
		input.ExpectedVersion,
		input.SubstateCode,
	)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.BookingFromDomain(result))
}

func (h *HTTPHandler) createAdminWaitlist(w http.ResponseWriter, request *http.Request) {
	organizationID, principal, ok := h.authorize(w, request, domain.PermissionOperate)
	if !ok {
		return
	}
	h.createWaitlist(w, request, organizationID, principal.ActorID, false)
}

func (h *HTTPHandler) createPublicWaitlist(w http.ResponseWriter, request *http.Request) {
	organizationID, err := h.usecases.ResolvePublicOrganization(
		request.Context(), chi.URLParam(request, "organizationSlug"),
	)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	if !h.requireEnabled(w, request, organizationID) {
		return
	}
	h.createWaitlist(w, request, organizationID, "public:scheduling", true)
}

func (h *HTTPHandler) createWaitlist(
	w http.ResponseWriter,
	request *http.Request,
	organizationID, actorID string,
	public bool,
) {
	var input dto.CreateWaitlist
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	if input.ID == uuid.Nil {
		input.ID = uuid.New()
	}
	metadata, ok := commandMetadata(w, request, organizationID, actorID, input)
	if !ok {
		return
	}
	result, err := h.usecases.CreateWaitlistEntry(request.Context(), metadata, CreateWaitlistInput{
		OrganizationID: organizationID, ID: input.ID, BranchID: input.BranchID,
		ServiceID: input.ServiceID,
		Customer: PublicCustomer{
			PartyID: input.Customer.PartyID,
			Name:    input.Customer.Name,
			Email:   input.Customer.Email,
			Phone:   input.Customer.Phone,
		},
		PreferredFrom: input.PreferredFrom.UTC(), PreferredUntil: input.PreferredUntil.UTC(),
		Participants: input.Participants, MeetRequested: input.MeetRequested,
	})
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	if public {
		httphelpers.WriteJSON(w, http.StatusCreated, dto.PublicWaitlistEntryFromDomain(result))
		return
	}
	httphelpers.WriteJSON(w, http.StatusCreated, dto.WaitlistEntryFromDomain(result))
}

func (h *HTTPHandler) listWaitlist(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionOperate)
	if !ok {
		return
	}
	branchID, ok := httphelpers.ParseUUID(w, request.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	result, err := h.usecases.ListWaitlist(request.Context(), organizationID, branchID)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.WaitlistFromDomain(result))
}

func (h *HTTPHandler) createQueueTicket(w http.ResponseWriter, request *http.Request) {
	organizationID, principal, ok := h.authorize(w, request, domain.PermissionOperate)
	if !ok {
		return
	}
	var input dto.CreateQueueTicket
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	if input.ID == uuid.Nil {
		input.ID = uuid.New()
	}
	metadata, ok := commandMetadata(w, request, organizationID, principal.ActorID, input)
	if !ok {
		return
	}
	result, err := h.usecases.CreateQueueTicket(request.Context(), metadata, domain.QueueTicket{
		OrganizationID: organizationID, ID: input.ID, BranchID: input.BranchID,
		ServiceID: input.ServiceID, PartyID: input.PartyID, Priority: input.Priority,
	})
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusCreated, dto.QueueTicketFromDomain(result))
}

func (h *HTTPHandler) listQueue(w http.ResponseWriter, request *http.Request) {
	organizationID, _, ok := h.authorize(w, request, domain.PermissionOperate)
	if !ok {
		return
	}
	branchID, ok := httphelpers.ParseUUID(w, request.URL.Query().Get("branch_id"))
	if !ok {
		return
	}
	result, err := h.usecases.ListQueue(request.Context(), organizationID, branchID)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.QueueFromDomain(result))
}

func (h *HTTPHandler) advanceQueueTicket(w http.ResponseWriter, request *http.Request) {
	organizationID, principal, ok := h.authorize(w, request, domain.PermissionOperate)
	if !ok {
		return
	}
	id, ok := httphelpers.ParseUUID(w, chi.URLParam(request, "ticketId"))
	if !ok {
		return
	}
	var input dto.AdvanceQueueTicket
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	metadata, ok := commandMetadata(w, request, organizationID, principal.ActorID, input)
	if !ok {
		return
	}
	result, err := h.usecases.AdvanceQueueTicket(
		request.Context(), metadata, organizationID, id, input.ExpectedVersion, input.Status,
	)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.QueueTicketFromDomain(result))
}

func (h *HTTPHandler) consumePublicAction(w http.ResponseWriter, request *http.Request) {
	var input dto.Action
	if !httphelpers.Decode(w, request, &input) {
		return
	}
	metadata, ok := commandMetadata(w, request, "", "public:action", input)
	if !ok {
		return
	}
	token := chi.URLParam(request, "token")
	organizationID, err := h.usecases.ResolveActionOrganization(
		request.Context(),
		token,
	)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	if !h.requireEnabled(w, request, organizationID) {
		return
	}
	if input.Purpose == domain.ActionAcceptWaitlist {
		result, err := h.usecases.ConsumeWaitlistAction(
			request.Context(), token, metadata, input.ExpectedVersion,
		)
		if err != nil {
			httphelpers.WriteError(w, err)
			return
		}
		httphelpers.WriteJSON(w, http.StatusOK, dto.PublicWaitlistEntryFromDomain(result))
		return
	}
	result, err := h.usecases.ConsumeBookingAction(
		request.Context(), token, input.Purpose, metadata, input.ExpectedVersion,
		input.StartAt, input.DurationMinutes, input.Reason,
	)
	if err != nil {
		httphelpers.WriteError(w, err)
		return
	}
	httphelpers.WriteJSON(w, http.StatusOK, dto.PublicBookingFromDomain(result))
}

func (h *HTTPHandler) authorize(
	w http.ResponseWriter,
	request *http.Request,
	permission string,
) (string, domain.Principal, bool) {
	organizationID := chi.URLParam(request, "organizationId")
	if h.auth == nil {
		httphelpers.WriteProblem(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
		return "", domain.Principal{}, false
	}
	principal, err := h.auth.Principal(request)
	if err != nil {
		httphelpers.WriteProblem(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
		return "", domain.Principal{}, false
	}
	if !principal.Allows(organizationID, permission) {
		httphelpers.WriteProblem(w, http.StatusForbidden, domain.CodeForbidden, "permission is required")
		return "", domain.Principal{}, false
	}
	if !h.requireEnabled(w, request, organizationID) {
		return "", domain.Principal{}, false
	}
	return organizationID, principal, true
}

func (h *HTTPHandler) requireEnabled(
	w http.ResponseWriter,
	request *http.Request,
	organizationID string,
) bool {
	if h.features == nil {
		httphelpers.WriteError(
			w,
			domain.NewError(
				domain.CodeFeatureDisabled,
				"scheduling is disabled",
			),
		)
		return false
	}
	enabled, err := h.features.Enabled(
		request.Context(),
		organizationID,
		"scheduling_enabled",
	)
	if err != nil {
		httphelpers.WriteProblem(
			w,
			http.StatusServiceUnavailable,
			"INTERNAL_ERROR",
			"feature configuration is unavailable",
		)
		return false
	}
	if !enabled {
		httphelpers.WriteError(
			w,
			domain.NewError(
				domain.CodeFeatureDisabled,
				"scheduling is disabled",
			),
		)
		return false
	}
	return true
}

func commandMetadata(
	w http.ResponseWriter,
	request *http.Request,
	organizationID, actorID string,
	payload any,
) (domain.CommandMetadata, bool) {
	key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 255 {
		httphelpers.WriteProblem(w, http.StatusBadRequest, domain.CodeValidation, "Idempotency-Key is required")
		return domain.CommandMetadata{}, false
	}
	sourceVersion := 1
	if raw := request.Header.Get("X-Source-Version"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			httphelpers.WriteProblem(w, http.StatusBadRequest, domain.CodeValidation, "X-Source-Version is invalid")
			return domain.CommandMetadata{}, false
		}
		sourceVersion = value
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		httphelpers.WriteProblem(w, http.StatusBadRequest, domain.CodeValidation, "request payload is invalid")
		return domain.CommandMetadata{}, false
	}
	digest := sha256.Sum256(encoded)
	requestID := strings.TrimSpace(request.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	correlationID := strings.TrimSpace(request.Header.Get("X-Correlation-ID"))
	if correlationID == "" {
		correlationID = requestID
	}
	return domain.CommandMetadata{
		OrganizationID: organizationID, IdempotencyKey: key, SourceID: key,
		SourceVersion: sourceVersion, PayloadHash: hex.EncodeToString(digest[:]),
		RequestID: requestID, CorrelationID: correlationID, ActorID: actorID,
	}, true
}
