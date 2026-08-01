package scheduling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
)

// Repository is owned by this consumer. Implementations must apply org RLS and
// keep booking, allocations, capacity, idempotency, audit and outbox changes in
// one PostgreSQL transaction.
type Repository interface {
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
	GetBranch(context.Context, string, uuid.UUID) (domain.Branch, error)
	GetService(context.Context, string, uuid.UUID) (domain.Service, []domain.ResourceRequirement, error)
	GetResources(context.Context, string, []uuid.UUID) ([]domain.Resource, error)
	LoadAvailability(context.Context, domain.AvailabilityQuery) (domain.AvailabilitySnapshot, error)
	ReserveBookings(context.Context, domain.CommandMetadata, *domain.RecurrenceSeries, []domain.Booking, []domain.ActionToken, []domain.Event) ([]domain.Booking, error)
	GetBooking(context.Context, string, uuid.UUID) (domain.Booking, error)
	ListBookings(context.Context, string, uuid.UUID, time.Time, time.Time) ([]domain.Booking, error)
	RescheduleBooking(context.Context, domain.CommandMetadata, uuid.UUID, int, domain.Booking, []domain.Event) (domain.Booking, error)
	TransitionBooking(context.Context, domain.CommandMetadata, uuid.UUID, int, domain.BookingStatus, string, []domain.Event) (domain.Booking, error)
	ConfigureBookingStatus(context.Context, domain.CommandMetadata, domain.BookingStatusConfiguration) (domain.BookingStatusConfiguration, error)
	ListBookingStatusConfigurations(context.Context, string) ([]domain.BookingStatusConfiguration, error)
	SetBookingSubstate(context.Context, domain.CommandMetadata, uuid.UUID, int, string) (domain.Booking, error)
	CreateGroupSession(context.Context, domain.CommandMetadata, domain.GroupSession, []domain.Allocation, domain.Event) (domain.GroupSession, error)
	GetGroupSession(context.Context, string, uuid.UUID) (domain.GroupSession, []domain.Allocation, error)
	SaveActionToken(context.Context, domain.ActionToken) error
	FindActionToken(context.Context, string) (domain.ActionToken, error)
	ConsumeActionToken(context.Context, string, time.Time, uuid.UUID) error
	CreateWaitlistEntry(context.Context, domain.CommandMetadata, domain.WaitlistEntry, domain.Event) (domain.WaitlistEntry, error)
	GetWaitlist(context.Context, string, uuid.UUID) (domain.WaitlistEntry, error)
	ListWaitlist(context.Context, string, uuid.UUID) ([]domain.WaitlistEntry, error)
	OfferWaitlist(context.Context, string, uuid.UUID, domain.Slot, time.Time, domain.ActionToken, []domain.Event) (domain.WaitlistEntry, error)
	ReleaseWaitlistClaim(context.Context, string, uuid.UUID) error
	AcceptWaitlist(context.Context, string, uuid.UUID, int, uuid.UUID, domain.Event) (domain.WaitlistEntry, error)
	CreateQueueTicket(context.Context, domain.CommandMetadata, domain.QueueTicket) (domain.QueueTicket, error)
	AdvanceQueueTicket(context.Context, domain.CommandMetadata, uuid.UUID, int, domain.QueueStatus) (domain.QueueTicket, error)
	ListQueue(context.Context, string, uuid.UUID) ([]domain.QueueTicket, error)
	ExpireHolds(context.Context, int, time.Time) ([]domain.Booking, error)
	ClaimReminders(context.Context, int, time.Time, time.Time) ([]domain.Event, error)
	ClaimWaitlistCandidates(context.Context, int, time.Time) ([]domain.WaitlistEntry, error)
}

// SchedulingAlgorithms is implemented by platform_scheduling.go. The port uses
// Pymes-owned types exclusively.
type SchedulingAlgorithms interface {
	NormalizeAllocations([]domain.Allocation, int) ([]domain.Allocation, error)
	CalculateSlots(domain.AvailabilityQuery, domain.AvailabilitySnapshot) ([]domain.Slot, error)
}

type OrganizationDirectory interface {
	ResolvePublicOrganization(context.Context, string) (string, error)
}

type PartyDirectory interface {
	EnsureCustomer(context.Context, domain.CommandMetadata, PublicCustomer) (CustomerIdentity, error)
}

type ActionTokenCodec interface {
	Issue() (raw string, hash string, err error)
	HashVerified(raw string) (string, error)
}

type PublicCustomer struct {
	PartyID string
	Name    string
	Email   string
	Phone   string
}

type CustomerIdentity struct {
	PartyID string
	Name    string
	Email   string
	Phone   string
}

type PublicCatalog struct {
	Branches  []domain.Branch
	Services  []domain.Service
	Resources []domain.Resource
}

type CreateWaitlistInput struct {
	OrganizationID string
	ID             uuid.UUID
	BranchID       uuid.UUID
	ServiceID      uuid.UUID
	Customer       PublicCustomer
	PreferredFrom  time.Time
	PreferredUntil time.Time
	Participants   int
}

type Service struct {
	repository    Repository
	algorithms    SchedulingAlgorithms
	organizations OrganizationDirectory
	parties       PartyDirectory
	tokens        ActionTokenCodec
	now           func() time.Time
}

type Option func(*Service)

func WithOrganizationDirectory(value OrganizationDirectory) Option {
	return func(service *Service) { service.organizations = value }
}

func WithPartyDirectory(value PartyDirectory) Option {
	return func(service *Service) { service.parties = value }
}

func WithClock(value func() time.Time) Option {
	return func(service *Service) { service.now = value }
}

func NewService(repository Repository, algorithms SchedulingAlgorithms, tokens ActionTokenCodec, options ...Option) *Service {
	service := &Service{repository: repository, algorithms: algorithms, tokens: tokens, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) CreateBranch(ctx context.Context, value domain.Branch) (domain.Branch, error) {
	value.Code = domain.NormalizeCode(value.Code)
	value.Slug = domain.NormalizeCode(value.Slug)
	if value.Slug == "" {
		value.Slug = value.Code
	}
	if err := value.Validate(); err != nil {
		return domain.Branch{}, err
	}
	return s.repository.CreateBranch(ctx, value)
}

func (s *Service) CreateService(
	ctx context.Context,
	value domain.Service,
	requirements []domain.ResourceRequirement,
) (domain.Service, error) {
	value.Code = domain.NormalizeCode(value.Code)
	value.Currency = strings.ToUpper(strings.TrimSpace(value.Currency))
	if err := value.Validate(); err != nil {
		return domain.Service{}, err
	}
	for index := range requirements {
		requirements[index].OrganizationID = value.OrganizationID
		requirements[index].ServiceID = value.ID
		if requirements[index].ID == uuid.Nil {
			requirements[index].ID = uuid.New()
		}
		if requirements[index].Units <= 0 ||
			(requirements[index].Mode != domain.AllocationCapacity && requirements[index].Mode != domain.AllocationExclusive) {
			return domain.Service{}, domain.NewError(domain.CodeValidation, "service resource requirement is invalid")
		}
	}
	return s.repository.CreateService(ctx, value, requirements)
}

func (s *Service) CreateResource(ctx context.Context, value domain.Resource) (domain.Resource, error) {
	value.Code = domain.NormalizeCode(value.Code)
	if err := value.Validate(); err != nil {
		return domain.Resource{}, err
	}
	return s.repository.CreateResource(ctx, value)
}

func (s *Service) CreateAvailabilityRule(ctx context.Context, value domain.AvailabilityRule) (domain.AvailabilityRule, error) {
	if err := value.Validate(); err != nil {
		return domain.AvailabilityRule{}, err
	}
	return s.repository.CreateAvailabilityRule(ctx, value)
}

func (s *Service) CreateAvailabilityException(
	ctx context.Context,
	value domain.AvailabilityException,
) (domain.AvailabilityException, error) {
	if value.OrganizationID == "" || value.ID == uuid.Nil || value.BranchID == uuid.Nil {
		return domain.AvailabilityException{}, domain.NewError(domain.CodeValidation, "exception identity is required")
	}
	if err := domain.ValidateRange(value.StartAt, value.EndAt); err != nil {
		return domain.AvailabilityException{}, err
	}
	return s.repository.CreateAvailabilityException(ctx, value)
}

func (s *Service) ListBranches(ctx context.Context, organizationID string) ([]domain.Branch, error) {
	return s.repository.ListBranches(ctx, organizationID)
}

func (s *Service) ListServices(ctx context.Context, organizationID string) ([]domain.Service, error) {
	return s.repository.ListServices(ctx, organizationID)
}

func (s *Service) ListResources(
	ctx context.Context,
	organizationID string,
	branchID uuid.UUID,
) ([]domain.Resource, error) {
	return s.repository.ListResources(ctx, organizationID, branchID)
}

func (s *Service) ListAvailabilityRules(
	ctx context.Context,
	organizationID string,
	branchID uuid.UUID,
) ([]domain.AvailabilityRule, error) {
	return s.repository.ListAvailabilityRules(ctx, organizationID, branchID)
}

func (s *Service) ListAvailabilityExceptions(
	ctx context.Context,
	organizationID string,
	branchID uuid.UUID,
	from, until time.Time,
) ([]domain.AvailabilityException, error) {
	if err := domain.ValidateRange(from, until); err != nil {
		return nil, err
	}
	return s.repository.ListAvailabilityExceptions(ctx, organizationID, branchID, from, until)
}

func (s *Service) PublicCatalog(
	ctx context.Context,
	organizationID string,
) (PublicCatalog, error) {
	branches, err := s.repository.ListBranches(ctx, organizationID)
	if err != nil {
		return PublicCatalog{}, err
	}
	services, err := s.repository.ListServices(ctx, organizationID)
	if err != nil {
		return PublicCatalog{}, err
	}
	resources, err := s.repository.ListResources(ctx, organizationID, uuid.Nil)
	if err != nil {
		return PublicCatalog{}, err
	}
	result := PublicCatalog{
		Branches:  make([]domain.Branch, 0, len(branches)),
		Services:  make([]domain.Service, 0, len(services)),
		Resources: make([]domain.Resource, 0, len(resources)),
	}
	for _, branch := range branches {
		if branch.Active {
			result.Branches = append(result.Branches, branch)
		}
	}
	for _, service := range services {
		if service.Active {
			result.Services = append(result.Services, service)
		}
	}
	for _, resource := range resources {
		if resource.Active {
			result.Resources = append(result.Resources, resource)
		}
	}
	return result, nil
}

func (s *Service) AvailableSlots(ctx context.Context, query domain.AvailabilityQuery) ([]domain.Slot, error) {
	if query.OrganizationID == "" || query.BranchID == uuid.Nil || query.ServiceID == uuid.Nil {
		return nil, domain.NewError(domain.CodeValidation, "organization, branch and service are required")
	}
	if query.Participants <= 0 {
		query.Participants = 1
	}
	if query.DurationMinutes < 0 || query.DurationMinutes > 1440 {
		return nil, domain.NewError(domain.CodeValidation, "duration override is invalid")
	}
	service, requirements, err := s.repository.GetService(ctx, query.OrganizationID, query.ServiceID)
	if err != nil {
		return nil, err
	}
	if !service.Active || query.Participants > service.MaxParticipants {
		return nil, domain.NewError(domain.CodeCapacityExceeded, "service capacity is unavailable")
	}
	resources, err := s.repository.ListResources(ctx, query.OrganizationID, query.BranchID)
	if err != nil {
		return nil, err
	}
	candidates, err := allocationCandidates(requirements, resources, query.Allocations)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Slot, 0)
	for _, allocations := range candidates {
		normalized, err := s.algorithms.NormalizeAllocations(allocations, query.Participants)
		if err != nil {
			return nil, err
		}
		candidateQuery := query
		candidateQuery.Allocations = normalized
		snapshot, err := s.repository.LoadAvailability(ctx, candidateQuery)
		if err != nil {
			return nil, err
		}
		if candidateQuery.DurationMinutes > 0 {
			snapshot.Service.DurationMinutes = candidateQuery.DurationMinutes
		}
		slots, err := s.algorithms.CalculateSlots(candidateQuery, snapshot)
		if err != nil {
			return nil, err
		}
		result = append(result, slots...)
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].StartAt.Equal(result[j].StartAt) {
			return result[i].StartAt.Before(result[j].StartAt)
		}
		return allocationKey(result[i].Allocations) < allocationKey(result[j].Allocations)
	})
	return result, nil
}

type CreateBookingInput struct {
	OrganizationID string
	BranchID       uuid.UUID
	ServiceID      uuid.UUID
	SessionID      *uuid.UUID
	PartyID        string
	Customer       PublicCustomer
	StartAt        time.Time
	Participants   int
	Status         domain.BookingStatus
	HoldFor        time.Duration
	Allocations    []domain.Allocation
	Notes          string
	Recurrence     *domain.RecurrenceRule
}

const bookingActionTokenTTL = 7 * 24 * time.Hour

func (s *Service) CreateBooking(
	ctx context.Context,
	metadata domain.CommandMetadata,
	input CreateBookingInput,
) ([]domain.Booking, error) {
	if err := metadata.Validate(); err != nil {
		return nil, err
	}
	if input.OrganizationID != metadata.OrganizationID || input.BranchID == uuid.Nil ||
		input.ServiceID == uuid.Nil || input.StartAt.IsZero() {
		return nil, domain.NewError(domain.CodeValidation, "booking identity is incomplete")
	}
	if input.Participants <= 0 {
		input.Participants = 1
	}
	branch, err := s.repository.GetBranch(ctx, input.OrganizationID, input.BranchID)
	if err != nil {
		return nil, err
	}
	service, requirements, err := s.repository.GetService(ctx, input.OrganizationID, input.ServiceID)
	if err != nil {
		return nil, err
	}
	if !branch.Active || !service.Active || input.Participants > service.MaxParticipants {
		return nil, domain.NewError(domain.CodeCapacityExceeded, "service capacity is unavailable")
	}
	partyID := strings.TrimSpace(input.PartyID)
	if partyID != "" {
		input.Customer.PartyID = partyID
	}
	if s.parties != nil {
		var customer CustomerIdentity
		customer, err = s.parties.EnsureCustomer(ctx, metadata, input.Customer)
		if err != nil {
			return nil, err
		}
		partyID = customer.PartyID
		input.Customer = PublicCustomer{
			PartyID: customer.PartyID,
			Name:    customer.Name,
			Email:   customer.Email,
			Phone:   customer.Phone,
		}
	}
	if partyID == "" {
		return nil, domain.NewError(domain.CodeValidation, "party_id is required")
	}
	var session *domain.GroupSession
	allocations := input.Allocations
	if input.SessionID != nil {
		if input.Recurrence != nil {
			return nil, domain.NewError(domain.CodeValidation, "group session booking cannot be recurrent")
		}
		value, sessionAllocations, err := s.repository.GetGroupSession(ctx, input.OrganizationID, *input.SessionID)
		if err != nil {
			return nil, err
		}
		if value.Status != "open" || value.BranchID != input.BranchID || value.ServiceID != input.ServiceID ||
			value.Booked+input.Participants > value.Capacity {
			return nil, domain.NewError(domain.CodeCapacityExceeded, "group session capacity is unavailable")
		}
		session = &value
		allocations = sessionAllocations
		input.StartAt = value.StartAt
	} else {
		allocations, err = s.algorithms.NormalizeAllocations(input.Allocations, input.Participants)
		if err != nil {
			return nil, err
		}
	}
	resourceIDs := allocationIDs(allocations)
	resources, err := s.repository.GetResources(ctx, input.OrganizationID, resourceIDs)
	if err != nil {
		return nil, err
	}
	if len(resources) != len(resourceIDs) {
		return nil, domain.NewError(domain.CodeNotFound, "one or more resources do not exist")
	}
	if err := satisfyRequirements(requirements, allocations, resources); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	status := input.Status
	if status == "" {
		if input.HoldFor > 0 {
			status = domain.BookingHeld
		} else if service.ConfirmationRequired {
			status = domain.BookingPendingConfirmation
		} else {
			status = domain.BookingConfirmed
		}
	}
	if status != domain.BookingHeld && status != domain.BookingPendingConfirmation && status != domain.BookingConfirmed {
		return nil, domain.NewError(domain.CodeBookingStateInvalid, "invalid initial booking state")
	}
	starts := []time.Time{input.StartAt.UTC()}
	var series *domain.RecurrenceSeries
	if input.Recurrence != nil {
		starts, err = domain.ExpandRecurrence(input.StartAt, branch.Timezone, *input.Recurrence)
		if err != nil {
			return nil, err
		}
		series = &domain.RecurrenceSeries{
			OrganizationID: input.OrganizationID,
			ID:             uuid.New(),
			Rule:           *input.Recurrence,
			Timezone:       branch.Timezone,
			Status:         "active",
			CreatedAt:      now,
		}
	}
	bookings := make([]domain.Booking, 0, len(starts))
	actionTokens := make([]domain.ActionToken, 0, len(starts)*3)
	events := make([]domain.Event, 0, len(starts)*3)
	for index, startAt := range starts {
		booking, err := s.buildBooking(
			ctx, input, metadata, branch, service, partyID, allocations,
			series, session, index, startAt, status, now,
		)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, booking)
		rawActions, tokens, err := s.bookingActionTokens(booking, now)
		if err != nil {
			return nil, err
		}
		actionTokens = append(actionTokens, tokens...)
		events = append(
			events,
			bookingEvents(metadata, booking, domain.EventBookingCreated, rawActions)...,
		)
	}
	return s.repository.ReserveBookings(ctx, metadata, series, bookings, actionTokens, events)
}

func (s *Service) bookingActionTokens(
	booking domain.Booking,
	now time.Time,
) (map[string]string, []domain.ActionToken, error) {
	if s.tokens == nil {
		return nil, nil, nil
	}
	purposes := []domain.ActionPurpose{domain.ActionCancel, domain.ActionReschedule}
	if booking.Status == domain.BookingPendingConfirmation {
		purposes = append(purposes, domain.ActionConfirm)
	}
	rawActions := make(map[string]string, len(purposes))
	result := make([]domain.ActionToken, 0, len(purposes))
	for _, purpose := range purposes {
		raw, hash, err := s.tokens.Issue()
		if err != nil {
			return nil, nil, err
		}
		bookingID := booking.ID
		rawActions[string(purpose)] = raw
		result = append(result, domain.ActionToken{
			OrganizationID: booking.OrganizationID,
			ID:             uuid.New(),
			BookingID:      &bookingID,
			Purpose:        purpose,
			TokenHash:      hash,
			ExpiresAt:      now.Add(bookingActionTokenTTL),
			CreatedAt:      now,
		})
	}
	return rawActions, result, nil
}

func (s *Service) buildBooking(
	ctx context.Context,
	input CreateBookingInput,
	metadata domain.CommandMetadata,
	branch domain.Branch,
	service domain.Service,
	partyID string,
	allocations []domain.Allocation,
	series *domain.RecurrenceSeries,
	session *domain.GroupSession,
	occurrence int,
	startAt time.Time,
	status domain.BookingStatus,
	now time.Time,
) (domain.Booking, error) {
	endAt := startAt.Add(time.Duration(service.DurationMinutes) * time.Minute)
	selected := &domain.Slot{}
	if session != nil {
		selected.StartAt, selected.EndAt = session.StartAt.UTC(), session.EndAt.UTC()
		selected.OccupiesFrom, selected.OccupiesUntil = session.StartAt.UTC(), session.EndAt.UTC()
		selected.Allocations = allocations
	} else {
		query := domain.AvailabilityQuery{
			OrganizationID: input.OrganizationID,
			BranchID:       input.BranchID,
			ServiceID:      input.ServiceID,
			From:           startAt,
			Until:          endAt,
			Participants:   input.Participants,
			Allocations:    allocations,
		}
		slots, err := s.AvailableSlots(ctx, query)
		if err != nil {
			return domain.Booking{}, err
		}
		selected = nil
		for index := range slots {
			if slots[index].StartAt.Equal(startAt.UTC()) {
				selected = &slots[index]
				break
			}
		}
		if selected == nil {
			return domain.Booking{}, domain.NewError(domain.CodeSlotConflict, "requested slot is no longer available")
		}
	}
	var seriesID *uuid.UUID
	if series != nil {
		value := series.ID
		seriesID = &value
	}
	booking := domain.Booking{
		OrganizationID:  input.OrganizationID,
		ID:              uuid.New(),
		SeriesID:        seriesID,
		SessionID:       input.SessionID,
		Occurrence:      occurrence,
		BranchID:        input.BranchID,
		ServiceID:       input.ServiceID,
		PartyID:         partyID,
		Status:          status,
		Participants:    input.Participants,
		StartAt:         selected.StartAt,
		EndAt:           selected.EndAt,
		OccupiesFrom:    selected.OccupiesFrom,
		OccupiesUntil:   selected.OccupiesUntil,
		Version:         1,
		ServiceName:     service.Name,
		Price:           service.Price,
		Currency:        service.Currency,
		DurationMinutes: service.DurationMinutes,
		Timezone:        branch.Timezone,
		CustomerName:    strings.TrimSpace(input.Customer.Name),
		CustomerEmail:   strings.TrimSpace(input.Customer.Email),
		CustomerPhone:   strings.TrimSpace(input.Customer.Phone),
		Notes:           strings.TrimSpace(input.Notes),
		Allocations:     append([]domain.Allocation(nil), selected.Allocations...),
		CreatedBy:       metadata.ActorID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if status == domain.BookingHeld {
		if input.HoldFor <= 0 || input.HoldFor > 30*time.Minute {
			return domain.Booking{}, domain.NewError(domain.CodeValidation, "hold duration must be between zero and 30 minutes")
		}
		expires := now.Add(input.HoldFor)
		booking.HoldExpiresAt = &expires
	}
	if err := booking.Validate(); err != nil {
		return domain.Booking{}, err
	}
	return booking, nil
}

type CreateGroupSessionInput struct {
	OrganizationID string
	BranchID       uuid.UUID
	ServiceID      uuid.UUID
	StartAt        time.Time
	Capacity       int
	Allocations    []domain.Allocation
}

func (s *Service) CreateGroupSession(
	ctx context.Context,
	metadata domain.CommandMetadata,
	input CreateGroupSessionInput,
) (domain.GroupSession, error) {
	if err := metadata.Validate(); err != nil {
		return domain.GroupSession{}, err
	}
	if input.OrganizationID != metadata.OrganizationID || input.BranchID == uuid.Nil ||
		input.ServiceID == uuid.Nil || input.StartAt.IsZero() || input.Capacity <= 0 {
		return domain.GroupSession{}, domain.NewError(domain.CodeValidation, "group session is invalid")
	}
	service, requirements, err := s.repository.GetService(ctx, input.OrganizationID, input.ServiceID)
	if err != nil {
		return domain.GroupSession{}, err
	}
	if !service.AllowGroup || input.Capacity > service.MaxParticipants {
		return domain.GroupSession{}, domain.NewError(domain.CodeCapacityExceeded, "group service capacity is unavailable")
	}
	allocations, err := s.algorithms.NormalizeAllocations(input.Allocations, 1)
	if err != nil {
		return domain.GroupSession{}, err
	}
	resources, err := s.repository.GetResources(ctx, input.OrganizationID, allocationIDs(allocations))
	if err != nil {
		return domain.GroupSession{}, err
	}
	if err := satisfyRequirements(requirements, allocations, resources); err != nil {
		return domain.GroupSession{}, err
	}
	endAt := input.StartAt.Add(time.Duration(service.DurationMinutes) * time.Minute)
	slots, err := s.AvailableSlots(ctx, domain.AvailabilityQuery{
		OrganizationID: input.OrganizationID,
		BranchID:       input.BranchID,
		ServiceID:      input.ServiceID,
		From:           input.StartAt,
		Until:          endAt,
		Participants:   1,
		Allocations:    allocations,
	})
	if err != nil {
		return domain.GroupSession{}, err
	}
	found := false
	for _, slot := range slots {
		if slot.StartAt.Equal(input.StartAt.UTC()) {
			allocations = slot.Allocations
			found = true
			break
		}
	}
	if !found {
		return domain.GroupSession{}, domain.NewError(domain.CodeSlotConflict, "group session slot is unavailable")
	}
	session := domain.GroupSession{
		OrganizationID: input.OrganizationID,
		ID:             uuid.New(),
		BranchID:       input.BranchID,
		ServiceID:      input.ServiceID,
		StartAt:        input.StartAt.UTC(),
		EndAt:          endAt.UTC(),
		Capacity:       input.Capacity,
		Version:        1,
		Status:         "open",
	}
	event := newEvent(metadata, session.ID.String(), "GroupSessionCreated", map[string]any{
		"session_id": session.ID,
		"branch_id":  session.BranchID,
		"service_id": session.ServiceID,
		"start_at":   session.StartAt,
		"capacity":   session.Capacity,
	})
	return s.repository.CreateGroupSession(ctx, metadata, session, allocations, event)
}

type RescheduleInput struct {
	OrganizationID  string
	BookingID       uuid.UUID
	ExpectedVersion int
	StartAt         time.Time
	DurationMinutes int
	Allocations     []domain.Allocation
}

func (s *Service) RescheduleBooking(
	ctx context.Context,
	metadata domain.CommandMetadata,
	input RescheduleInput,
) (domain.Booking, error) {
	if err := metadata.Validate(); err != nil {
		return domain.Booking{}, err
	}
	current, err := s.repository.GetBooking(ctx, input.OrganizationID, input.BookingID)
	if err != nil {
		return domain.Booking{}, err
	}
	if !current.Status.Active() || input.ExpectedVersion <= 0 {
		return domain.Booking{}, domain.NewError(domain.CodeBookingStateInvalid, "booking cannot be rescheduled")
	}
	durationMinutes := input.DurationMinutes
	if durationMinutes == 0 {
		durationMinutes = current.DurationMinutes
	}
	if durationMinutes <= 0 || durationMinutes > 1440 {
		return domain.Booking{}, domain.NewError(domain.CodeValidation, "duration must be between 1 and 1440 minutes")
	}
	allocations := input.Allocations
	if len(allocations) == 0 {
		allocations = current.Allocations
	}
	allocations, err = s.algorithms.NormalizeAllocations(allocations, current.Participants)
	if err != nil {
		return domain.Booking{}, err
	}
	service, requirements, err := s.repository.GetService(ctx, input.OrganizationID, current.ServiceID)
	if err != nil {
		return domain.Booking{}, err
	}
	resources, err := s.repository.GetResources(ctx, input.OrganizationID, allocationIDs(allocations))
	if err != nil {
		return domain.Booking{}, err
	}
	if err := satisfyRequirements(requirements, allocations, resources); err != nil {
		return domain.Booking{}, err
	}
	endAt := input.StartAt.Add(time.Duration(durationMinutes) * time.Minute)
	query := domain.AvailabilityQuery{
		OrganizationID:   input.OrganizationID,
		BranchID:         current.BranchID,
		ServiceID:        current.ServiceID,
		From:             input.StartAt,
		Until:            endAt,
		Participants:     current.Participants,
		DurationMinutes:  durationMinutes,
		Allocations:      allocations,
		ExcludeBookingID: &current.ID,
	}
	slots, err := s.AvailableSlots(ctx, query)
	if err != nil {
		return domain.Booking{}, err
	}
	var selected domain.Slot
	found := false
	for _, slot := range slots {
		if slot.StartAt.Equal(input.StartAt.UTC()) {
			selected, found = slot, true
			break
		}
	}
	if !found {
		return domain.Booking{}, domain.NewError(domain.CodeSlotConflict, "requested slot is no longer available")
	}
	now := s.now().UTC()
	replacement := current
	replacement.ID = uuid.New()
	replacement.SupersedesID = &current.ID
	replacement.StartAt, replacement.EndAt = selected.StartAt, selected.EndAt
	replacement.OccupiesFrom, replacement.OccupiesUntil = selected.OccupiesFrom, selected.OccupiesUntil
	replacement.Allocations = selected.Allocations
	replacement.Version = 1
	replacement.DurationMinutes = durationMinutes
	replacement.CreatedAt, replacement.UpdatedAt = now, now
	replacement.CreatedBy = metadata.ActorID
	replacement.HoldExpiresAt = nil
	replacement.SubstateCode = ""
	if current.Status == domain.BookingHeld {
		replacement.Status = domain.BookingPendingConfirmation
	}
	_ = service
	payload := map[string]any{
		"booking_id":            replacement.ID,
		"supersedes_booking_id": current.ID,
		"start_at":              replacement.StartAt,
		"end_at":                replacement.EndAt,
		"version":               replacement.Version,
	}
	events := lifecycleAndProjectionEvents(
		metadata,
		replacement.ID.String(),
		domain.EventBookingRescheduled,
		payload,
		bookingNotificationPayload(
			replacement,
			domain.EventBookingRescheduled,
			map[string]any{"supersedes_booking_id": current.ID},
		),
	)
	return s.repository.RescheduleBooking(
		ctx, metadata, current.ID, input.ExpectedVersion, replacement, events,
	)
}

func (s *Service) TransitionBooking(
	ctx context.Context,
	metadata domain.CommandMetadata,
	organizationID string,
	bookingID uuid.UUID,
	expectedVersion int,
	to domain.BookingStatus,
	reason string,
) (domain.Booking, error) {
	if err := metadata.Validate(); err != nil {
		return domain.Booking{}, err
	}
	current, err := s.repository.GetBooking(ctx, organizationID, bookingID)
	if err != nil {
		return domain.Booking{}, err
	}
	if !current.Status.CanTransition(to) {
		return domain.Booking{}, domain.NewError(domain.CodeBookingStateInvalid, "booking transition is not allowed")
	}
	reason = strings.TrimSpace(reason)
	if to == domain.BookingCancelled && reason == "" {
		return domain.Booking{}, domain.NewError(domain.CodeValidation, "cancellation reason is required")
	}
	if len(reason) > 500 {
		return domain.Booking{}, domain.NewError(domain.CodeValidation, "transition reason is too long")
	}
	eventType := map[domain.BookingStatus]string{
		domain.BookingConfirmed: EventOrDefault(domain.EventBookingConfirmed),
		domain.BookingCancelled: EventOrDefault(domain.EventBookingCancelled),
		domain.BookingCompleted: EventOrDefault(domain.EventBookingCompleted),
		domain.BookingNoShow:    EventOrDefault(domain.EventBookingNoShow),
	}[to]
	if eventType == "" {
		eventType = "BookingStateChanged"
	}
	payload := map[string]any{
		"booking_id": bookingID,
		"from":       current.Status,
		"to":         to,
		"start_at":   current.StartAt,
		"end_at":     current.EndAt,
		"version":    expectedVersion + 1,
	}
	if reason != "" {
		payload["reason"] = reason
	}
	projected := current
	projected.Status = to
	projected.Version = expectedVersion + 1
	projected.CancellationReason = reason
	events := lifecycleAndProjectionEvents(
		metadata,
		bookingID.String(),
		eventType,
		payload,
		bookingNotificationPayload(
			projected,
			eventType,
			map[string]any{"reason": reason},
		),
	)
	return s.repository.TransitionBooking(ctx, metadata, bookingID, expectedVersion, to, reason, events)
}

func EventOrDefault(value string) string { return value }

func (s *Service) GetBooking(ctx context.Context, organizationID string, bookingID uuid.UUID) (domain.Booking, error) {
	return s.repository.GetBooking(ctx, organizationID, bookingID)
}

func (s *Service) ListBookings(
	ctx context.Context,
	organizationID string,
	branchID uuid.UUID,
	from, until time.Time,
) ([]domain.Booking, error) {
	if err := domain.ValidateRange(from, until); err != nil {
		return nil, err
	}
	return s.repository.ListBookings(ctx, organizationID, branchID, from, until)
}

func (s *Service) ConfigureBookingStatus(
	ctx context.Context,
	metadata domain.CommandMetadata,
	configuration domain.BookingStatusConfiguration,
) (domain.BookingStatusConfiguration, error) {
	if err := metadata.Validate(); err != nil {
		return domain.BookingStatusConfiguration{}, err
	}
	if configuration.OrganizationID != metadata.OrganizationID {
		return domain.BookingStatusConfiguration{}, domain.NewError(
			domain.CodeValidation,
			"booking status configuration tenant does not match the command",
		)
	}
	if err := configuration.Validate(); err != nil {
		return domain.BookingStatusConfiguration{}, err
	}
	return s.repository.ConfigureBookingStatus(ctx, metadata, configuration)
}

func (s *Service) ListBookingStatusConfigurations(
	ctx context.Context,
	organizationID string,
) ([]domain.BookingStatusConfiguration, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, domain.NewError(domain.CodeValidation, "organization is required")
	}
	return s.repository.ListBookingStatusConfigurations(ctx, organizationID)
}

func (s *Service) SetBookingSubstate(
	ctx context.Context,
	metadata domain.CommandMetadata,
	organizationID string,
	bookingID uuid.UUID,
	expectedVersion int,
	substateCode string,
) (domain.Booking, error) {
	if err := metadata.Validate(); err != nil {
		return domain.Booking{}, err
	}
	substateCode = strings.TrimSpace(substateCode)
	if organizationID != metadata.OrganizationID || bookingID == uuid.Nil || expectedVersion <= 0 ||
		(substateCode != "" && !domain.ValidBookingSubstateCode(substateCode)) {
		return domain.Booking{}, domain.NewError(
			domain.CodeValidation,
			"booking substate command is invalid",
		)
	}
	return s.repository.SetBookingSubstate(
		ctx,
		metadata,
		bookingID,
		expectedVersion,
		substateCode,
	)
}

func (s *Service) IssueActionToken(
	ctx context.Context,
	organizationID string,
	bookingID *uuid.UUID,
	waitlistID *uuid.UUID,
	purpose domain.ActionPurpose,
	validFor time.Duration,
) (string, error) {
	if s.tokens == nil || validFor <= 0 || validFor > 30*24*time.Hour {
		return "", domain.NewError(domain.CodeValidation, "action token configuration is invalid")
	}
	raw, hash, err := s.tokens.Issue()
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	token := domain.ActionToken{
		OrganizationID: organizationID,
		ID:             uuid.New(),
		BookingID:      bookingID,
		WaitlistID:     waitlistID,
		Purpose:        purpose,
		TokenHash:      hash,
		ExpiresAt:      now.Add(validFor),
		CreatedAt:      now,
	}
	if err := s.repository.SaveActionToken(ctx, token); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Service) ConsumeBookingAction(
	ctx context.Context,
	rawToken string,
	purpose domain.ActionPurpose,
	metadata domain.CommandMetadata,
	expectedVersion int,
	rescheduleAt *time.Time,
	durationMinutes int,
	reason string,
) (domain.Booking, error) {
	hash, err := s.tokens.HashVerified(rawToken)
	if err != nil {
		return domain.Booking{}, domain.NewError(domain.CodeActionTokenInvalid, "action token is invalid")
	}
	token, err := s.repository.FindActionToken(ctx, hash)
	if err != nil || token.Purpose != purpose || token.BookingID == nil {
		return domain.Booking{}, domain.NewError(domain.CodeActionTokenInvalid, "action token is invalid")
	}
	if token.ConsumedAt != nil {
		if token.ResultBookingID == nil {
			return domain.Booking{}, domain.NewError(domain.CodeActionTokenInvalid, "action token result is unavailable")
		}
		return s.repository.GetBooking(ctx, token.OrganizationID, *token.ResultBookingID)
	}
	now := s.now().UTC()
	if !token.ExpiresAt.After(now) {
		return domain.Booking{}, domain.NewError(domain.CodeActionTokenExpired, "action token has expired")
	}
	metadata.OrganizationID = token.OrganizationID
	var result domain.Booking
	switch purpose {
	case domain.ActionConfirm:
		result, err = s.TransitionBooking(ctx, metadata, token.OrganizationID, *token.BookingID, expectedVersion, domain.BookingConfirmed, "")
	case domain.ActionCancel:
		result, err = s.TransitionBooking(ctx, metadata, token.OrganizationID, *token.BookingID, expectedVersion, domain.BookingCancelled, reason)
	case domain.ActionReschedule:
		if rescheduleAt == nil {
			return domain.Booking{}, domain.NewError(domain.CodeValidation, "reschedule time is required")
		}
		result, err = s.RescheduleBooking(ctx, metadata, RescheduleInput{
			OrganizationID:  token.OrganizationID,
			BookingID:       *token.BookingID,
			ExpectedVersion: expectedVersion,
			StartAt:         *rescheduleAt,
			DurationMinutes: durationMinutes,
		})
	default:
		err = domain.NewError(domain.CodeActionTokenInvalid, "action purpose is invalid")
	}
	if err != nil {
		return domain.Booking{}, err
	}
	if err := s.repository.ConsumeActionToken(ctx, hash, now, result.ID); err != nil {
		return domain.Booking{}, err
	}
	return result, nil
}

func (s *Service) ResolveActionOrganization(
	ctx context.Context,
	rawToken string,
) (string, error) {
	if s.repository == nil || s.tokens == nil ||
		strings.TrimSpace(rawToken) == "" {
		return "", domain.NewError(
			domain.CodeActionTokenInvalid,
			"action token is invalid",
		)
	}
	hash, err := s.tokens.HashVerified(rawToken)
	if err != nil {
		return "", domain.NewError(
			domain.CodeActionTokenInvalid,
			"action token is invalid",
		)
	}
	token, err := s.repository.FindActionToken(ctx, hash)
	if err != nil || token.OrganizationID == "" {
		return "", domain.NewError(
			domain.CodeActionTokenInvalid,
			"action token is invalid",
		)
	}
	return token.OrganizationID, nil
}

func (s *Service) ConsumeWaitlistAction(
	ctx context.Context,
	rawToken string,
	metadata domain.CommandMetadata,
	expectedVersion int,
) (domain.WaitlistEntry, error) {
	hash, err := s.tokens.HashVerified(rawToken)
	if err != nil {
		return domain.WaitlistEntry{}, domain.NewError(domain.CodeActionTokenInvalid, "action token is invalid")
	}
	token, err := s.repository.FindActionToken(ctx, hash)
	if err != nil || token.Purpose != domain.ActionAcceptWaitlist || token.WaitlistID == nil {
		return domain.WaitlistEntry{}, domain.NewError(domain.CodeActionTokenInvalid, "action token is invalid")
	}
	if token.ConsumedAt != nil {
		entry, getErr := s.repository.GetWaitlist(ctx, token.OrganizationID, *token.WaitlistID)
		if getErr != nil || entry.Status != domain.WaitlistAccepted ||
			entry.AcceptedBookingID == nil || token.ResultBookingID == nil ||
			*entry.AcceptedBookingID != *token.ResultBookingID {
			return domain.WaitlistEntry{}, domain.NewError(domain.CodeActionTokenInvalid, "action token result is unavailable")
		}
		return entry, nil
	}
	now := s.now().UTC()
	if !token.ExpiresAt.After(now) {
		return domain.WaitlistEntry{}, domain.NewError(domain.CodeActionTokenExpired, "action token has expired")
	}
	metadata.OrganizationID = token.OrganizationID
	if err := metadata.Validate(); err != nil {
		return domain.WaitlistEntry{}, err
	}
	current, err := s.repository.GetWaitlist(ctx, token.OrganizationID, *token.WaitlistID)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if current.Status != domain.WaitlistOffered || current.Version != expectedVersion ||
		current.OfferedStartAt == nil || current.OfferedEndAt == nil ||
		len(current.OfferedAllocations) == 0 {
		return domain.WaitlistEntry{}, domain.NewError(
			domain.CodeBookingVersionConflict,
			"waitlist offer changed, expired or has no reservable slot",
		)
	}
	bookingPayload, _ := json.Marshal(map[string]any{
		"waitlist_id":  *token.WaitlistID,
		"start_at":     current.OfferedStartAt,
		"end_at":       current.OfferedEndAt,
		"allocations":  current.OfferedAllocations,
		"participants": current.Participants,
	})
	bookingDigest := sha256.Sum256(bookingPayload)
	bookingMetadata := metadata
	bookingMetadata.SourceID = "waitlist:" + token.WaitlistID.String()
	bookingMetadata.SourceVersion = expectedVersion
	bookingMetadata.PayloadHash = hex.EncodeToString(bookingDigest[:])
	bookings, err := s.CreateBooking(ctx, bookingMetadata, CreateBookingInput{
		OrganizationID: current.OrganizationID,
		BranchID:       current.BranchID,
		ServiceID:      current.ServiceID,
		PartyID:        current.PartyID,
		Customer:       PublicCustomer{PartyID: current.PartyID},
		StartAt:        current.OfferedStartAt.UTC(),
		Participants:   current.Participants,
		Status:         domain.BookingConfirmed,
		Allocations:    append([]domain.Allocation(nil), current.OfferedAllocations...),
	})
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if len(bookings) != 1 {
		return domain.WaitlistEntry{}, domain.NewError(domain.CodeValidation, "waitlist booking result is invalid")
	}
	bookingID := bookings[0].ID
	event := newEvent(metadata, token.WaitlistID.String(), "WaitlistAccepted", map[string]any{
		"waitlist_id": *token.WaitlistID,
		"booking_id":  bookingID,
	})
	result, err := s.repository.AcceptWaitlist(
		ctx, token.OrganizationID, *token.WaitlistID, expectedVersion, bookingID, event,
	)
	if err != nil {
		return domain.WaitlistEntry{}, err
	}
	if err := s.repository.ConsumeActionToken(ctx, hash, now, bookingID); err != nil {
		return domain.WaitlistEntry{}, err
	}
	return result, nil
}

func (s *Service) CreateWaitlistEntry(
	ctx context.Context,
	metadata domain.CommandMetadata,
	input CreateWaitlistInput,
) (domain.WaitlistEntry, error) {
	if err := metadata.Validate(); err != nil {
		return domain.WaitlistEntry{}, err
	}
	partyID := strings.TrimSpace(input.Customer.PartyID)
	if s.parties != nil {
		customer, err := s.parties.EnsureCustomer(ctx, metadata, input.Customer)
		if err != nil {
			return domain.WaitlistEntry{}, err
		}
		partyID = customer.PartyID
		input.Customer = PublicCustomer{
			PartyID: customer.PartyID,
			Name:    customer.Name,
			Email:   customer.Email,
			Phone:   customer.Phone,
		}
	} else if partyID == "" {
		return domain.WaitlistEntry{}, domain.NewError(domain.CodeValidation, "customer is required")
	}
	value := domain.WaitlistEntry{
		OrganizationID: input.OrganizationID,
		ID:             input.ID,
		BranchID:       input.BranchID,
		ServiceID:      input.ServiceID,
		PartyID:        partyID,
		CustomerName:   strings.TrimSpace(input.Customer.Name),
		CustomerEmail:  strings.TrimSpace(input.Customer.Email),
		CustomerPhone:  strings.TrimSpace(input.Customer.Phone),
		PreferredFrom:  input.PreferredFrom,
		PreferredUntil: input.PreferredUntil,
		Participants:   input.Participants,
	}
	if value.OrganizationID != metadata.OrganizationID || value.ID == uuid.Nil ||
		value.BranchID == uuid.Nil || value.ServiceID == uuid.Nil || value.PartyID == "" ||
		value.Participants <= 0 || !value.PreferredUntil.After(value.PreferredFrom) {
		return domain.WaitlistEntry{}, domain.NewError(domain.CodeValidation, "waitlist entry is invalid")
	}
	value.Status, value.Version = domain.WaitlistPending, 1
	value.CreatedAt, value.UpdatedAt = s.now().UTC(), s.now().UTC()
	event := newEvent(metadata, value.ID.String(), "WaitlistCreated", map[string]any{"waitlist_id": value.ID})
	return s.repository.CreateWaitlistEntry(ctx, metadata, value, event)
}

func (s *Service) ListWaitlist(ctx context.Context, organizationID string, branchID uuid.UUID) ([]domain.WaitlistEntry, error) {
	return s.repository.ListWaitlist(ctx, organizationID, branchID)
}

func (s *Service) CreateQueueTicket(
	ctx context.Context,
	metadata domain.CommandMetadata,
	value domain.QueueTicket,
) (domain.QueueTicket, error) {
	if err := metadata.Validate(); err != nil {
		return domain.QueueTicket{}, err
	}
	if value.OrganizationID != metadata.OrganizationID || value.ID == uuid.Nil ||
		value.BranchID == uuid.Nil || value.ServiceID == uuid.Nil || value.PartyID == "" {
		return domain.QueueTicket{}, domain.NewError(domain.CodeValidation, "queue ticket is invalid")
	}
	value.Status, value.Version = domain.QueueWaiting, 1
	value.CreatedAt, value.UpdatedAt = s.now().UTC(), s.now().UTC()
	return s.repository.CreateQueueTicket(ctx, metadata, value)
}

func (s *Service) AdvanceQueueTicket(
	ctx context.Context,
	metadata domain.CommandMetadata,
	organizationID string,
	ticketID uuid.UUID,
	expectedVersion int,
	status domain.QueueStatus,
) (domain.QueueTicket, error) {
	return s.repository.AdvanceQueueTicket(ctx, metadata, ticketID, expectedVersion, status)
}

func (s *Service) ListQueue(ctx context.Context, organizationID string, branchID uuid.UUID) ([]domain.QueueTicket, error) {
	return s.repository.ListQueue(ctx, organizationID, branchID)
}

func (s *Service) ResolvePublicOrganization(ctx context.Context, slug string) (string, error) {
	if s.organizations == nil || strings.TrimSpace(slug) == "" {
		return "", domain.NewError(domain.CodeNotFound, "organization is not available")
	}
	return s.organizations.ResolvePublicOrganization(ctx, slug)
}

func (s *Service) RunMaintenance(ctx context.Context, limit int) (domain.MaintenanceResult, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	now := s.now().UTC()
	expired, err := s.repository.ExpireHolds(ctx, limit, now)
	if err != nil {
		return domain.MaintenanceResult{}, err
	}
	reminders, err := s.repository.ClaimReminders(
		ctx, limit, now.Add(23*time.Hour), now.Add(24*time.Hour),
	)
	if err != nil {
		return domain.MaintenanceResult{}, err
	}
	candidates, err := s.repository.ClaimWaitlistCandidates(ctx, limit, now)
	if err != nil {
		return domain.MaintenanceResult{}, err
	}
	offers := 0
	if len(candidates) > 0 && s.tokens == nil {
		return domain.MaintenanceResult{}, domain.NewError(domain.CodeValidation, "action token codec is not configured")
	}
	for _, candidate := range candidates {
		from := candidate.PreferredFrom
		if from.Before(now) {
			from = now
		}
		if !candidate.PreferredUntil.After(from) {
			if err := s.repository.ReleaseWaitlistClaim(
				ctx, candidate.OrganizationID, candidate.ID,
			); err != nil {
				return domain.MaintenanceResult{}, err
			}
			continue
		}
		slots, err := s.AvailableSlots(ctx, domain.AvailabilityQuery{
			OrganizationID: candidate.OrganizationID,
			BranchID:       candidate.BranchID,
			ServiceID:      candidate.ServiceID,
			From:           from,
			Until:          candidate.PreferredUntil,
			Participants:   candidate.Participants,
		})
		if err != nil {
			_ = s.repository.ReleaseWaitlistClaim(ctx, candidate.OrganizationID, candidate.ID)
			return domain.MaintenanceResult{}, err
		}
		if len(slots) == 0 {
			if err := s.repository.ReleaseWaitlistClaim(
				ctx, candidate.OrganizationID, candidate.ID,
			); err != nil {
				return domain.MaintenanceResult{}, err
			}
			continue
		}
		offeredSlot := slots[0]
		raw, hash, err := s.tokens.Issue()
		if err != nil {
			return domain.MaintenanceResult{}, err
		}
		expiresAt := now.Add(15 * time.Minute)
		token := domain.ActionToken{
			OrganizationID: candidate.OrganizationID,
			ID:             uuid.New(),
			WaitlistID:     &candidate.ID,
			Purpose:        domain.ActionAcceptWaitlist,
			TokenHash:      hash,
			ExpiresAt:      expiresAt,
			CreatedAt:      now,
		}
		metadata := domain.CommandMetadata{
			OrganizationID: candidate.OrganizationID,
			IdempotencyKey: "waitlist-offer:" + candidate.ID.String() + ":" + fmt.Sprint(candidate.Version),
			SourceID:       candidate.ID.String(),
			SourceVersion:  candidate.Version,
			PayloadHash:    strings.Repeat("0", 64),
			RequestID:      "worker:waitlist",
			CorrelationID:  "worker:waitlist:" + candidate.ID.String(),
			ActorID:        "system:scheduling-worker",
		}
		lifecyclePayload := map[string]any{
			"waitlist_id": candidate.ID,
			"expires_at":  expiresAt,
			"start_at":    offeredSlot.StartAt,
			"end_at":      offeredSlot.EndAt,
		}
		notificationPayload := map[string]any{
			"aggregate_type": "waitlist",
			"aggregate_id":   candidate.ID,
			"waitlist_id":    candidate.ID,
			"recipient_e164": candidate.CustomerPhone,
			"customer_name":  candidate.CustomerName,
			"action_token":   raw,
			"expires_at":     expiresAt,
			"start_at":       offeredSlot.StartAt,
			"end_at":         offeredSlot.EndAt,
		}
		events := []domain.Event{
			newEvent(metadata, candidate.ID.String(), domain.EventWaitlistOffered, lifecyclePayload),
			newProjectionEvent(
				metadata, candidate.ID.String(), domain.EventNotificationRequested,
				domain.EventWaitlistOffered, notificationPayload,
			),
		}
		if _, err := s.repository.OfferWaitlist(
			ctx, candidate.OrganizationID, candidate.ID, offeredSlot, expiresAt, token, events,
		); err != nil {
			return domain.MaintenanceResult{}, err
		}
		offers++
	}
	return domain.MaintenanceResult{
		ExpiredHolds:   len(expired),
		ReminderEvents: len(reminders),
		WaitlistOffers: offers,
	}, nil
}

func allocationIDs(values []domain.Allocation) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value.ResourceID]; ok {
			continue
		}
		seen[value.ResourceID] = struct{}{}
		result = append(result, value.ResourceID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func satisfyRequirements(
	requirements []domain.ResourceRequirement,
	allocations []domain.Allocation,
	resources []domain.Resource,
) error {
	resourceByID := make(map[uuid.UUID]domain.Resource, len(resources))
	for _, resource := range resources {
		resourceByID[resource.ID] = resource
	}
	for _, requirement := range requirements {
		if requirement.Optional {
			continue
		}
		matched := false
		for _, allocation := range allocations {
			if requirement.ResourceID != nil && allocation.ResourceID != *requirement.ResourceID {
				continue
			}
			if requirement.ResourceID == nil {
				resource, ok := resourceByID[allocation.ResourceID]
				if !ok || resource.Kind != requirement.Kind {
					continue
				}
			}
			if allocation.Mode != requirement.Mode || allocation.Units < requirement.Units {
				continue
			}
			matched = true
			break
		}
		if !matched {
			return domain.NewError(domain.CodeValidation, "required resource allocation is missing")
		}
	}
	return nil
}

const maxAllocationCandidates = 256

func allocationCandidates(
	requirements []domain.ResourceRequirement,
	resources []domain.Resource,
	requested []domain.Allocation,
) ([][]domain.Allocation, error) {
	resourceByID := make(map[uuid.UUID]domain.Resource, len(resources))
	eligible := append([]domain.Resource(nil), resources...)
	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].ID.String() < eligible[j].ID.String()
	})
	for _, resource := range eligible {
		if resource.Active {
			resourceByID[resource.ID] = resource
		}
	}
	base := append([]domain.Allocation(nil), requested...)
	for _, allocation := range base {
		resource, ok := resourceByID[allocation.ResourceID]
		if !ok || allocation.Units <= 0 {
			return nil, domain.NewError(domain.CodeValidation, "requested resource is unavailable")
		}
		if allocation.Mode != domain.AllocationCapacity && allocation.Mode != domain.AllocationExclusive {
			return nil, domain.NewError(domain.CodeValidation, "resource allocation mode is invalid")
		}
		if allocation.Units > resource.Capacity {
			return nil, domain.NewError(domain.CodeCapacityExceeded, "requested resource capacity is unavailable")
		}
	}
	candidates := [][]domain.Allocation{base}
	for _, requirement := range requirements {
		if requirement.Optional {
			continue
		}
		matches := make([]domain.Resource, 0)
		for _, resource := range eligible {
			if !resource.Active || resource.Capacity < requirement.Units {
				continue
			}
			if requirement.ResourceID != nil {
				if resource.ID != *requirement.ResourceID {
					continue
				}
			} else if resource.Kind != requirement.Kind {
				continue
			}
			matches = append(matches, resource)
		}
		if len(matches) == 0 {
			return nil, domain.NewError(domain.CodeCapacityExceeded, "required resource is unavailable")
		}
		expanded := make([][]domain.Allocation, 0, len(candidates)*len(matches))
		for _, candidate := range candidates {
			if requirementSatisfied(requirement, candidate, resourceByID) {
				expanded = append(expanded, candidate)
				continue
			}
			for _, resource := range matches {
				if containsAllocation(candidate, resource.ID) {
					continue
				}
				next := append([]domain.Allocation(nil), candidate...)
				next = append(next, domain.Allocation{
					ResourceID: resource.ID,
					Mode:       requirement.Mode,
					Units:      requirement.Units,
				})
				expanded = append(expanded, next)
				if len(expanded) > maxAllocationCandidates {
					return nil, domain.NewError(domain.CodeValidation, "resource selection is too broad")
				}
			}
		}
		if len(expanded) == 0 {
			return nil, domain.NewError(domain.CodeValidation, "resource requirements cannot be combined")
		}
		candidates = expanded
	}
	result := make([][]domain.Allocation, 0, len(candidates))
	for _, candidate := range candidates {
		candidateResources := make([]domain.Resource, 0, len(candidate))
		for _, allocation := range candidate {
			if resource, ok := resourceByID[allocation.ResourceID]; ok {
				candidateResources = append(candidateResources, resource)
			}
		}
		if satisfyRequirements(requirements, candidate, candidateResources) == nil {
			sort.Slice(candidate, func(i, j int) bool {
				return candidate[i].ResourceID.String() < candidate[j].ResourceID.String()
			})
			result = append(result, candidate)
		}
	}
	if len(result) == 0 {
		return nil, domain.NewError(domain.CodeValidation, "required resource allocation is missing")
	}
	return result, nil
}

func requirementSatisfied(
	requirement domain.ResourceRequirement,
	allocations []domain.Allocation,
	resources map[uuid.UUID]domain.Resource,
) bool {
	for _, allocation := range allocations {
		resource, ok := resources[allocation.ResourceID]
		if !ok || allocation.Mode != requirement.Mode || allocation.Units < requirement.Units {
			continue
		}
		if requirement.ResourceID != nil && allocation.ResourceID == *requirement.ResourceID {
			return true
		}
		if requirement.ResourceID == nil && resource.Kind == requirement.Kind {
			return true
		}
	}
	return false
}

func containsAllocation(values []domain.Allocation, resourceID uuid.UUID) bool {
	for _, value := range values {
		if value.ResourceID == resourceID {
			return true
		}
	}
	return false
}

func allocationKey(values []domain.Allocation) string {
	ordered := append([]domain.Allocation(nil), values...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].ResourceID.String() < ordered[j].ResourceID.String()
	})
	var builder strings.Builder
	for _, value := range ordered {
		fmt.Fprintf(&builder, "%s:%s:%d;", value.ResourceID, value.Mode, value.Units)
	}
	return builder.String()
}

func bookingEvents(
	metadata domain.CommandMetadata,
	booking domain.Booking,
	eventType string,
	rawActions ...map[string]string,
) []domain.Event {
	payload := map[string]any{
		"booking_id": booking.ID,
		"branch_id":  booking.BranchID,
		"service_id": booking.ServiceID,
		"party_id":   booking.PartyID,
		"status":     booking.Status,
		"start_at":   booking.StartAt,
		"end_at":     booking.EndAt,
		"version":    booking.Version,
	}
	var actions map[string]string
	if len(rawActions) > 0 {
		actions = rawActions[0]
	}
	return lifecycleAndProjectionEvents(
		metadata,
		booking.ID.String(),
		eventType,
		payload,
		bookingNotificationPayload(
			booking,
			eventType,
			map[string]any{"action_tokens": actions},
		),
	)
}

func lifecycleAndProjectionEvents(
	metadata domain.CommandMetadata,
	aggregateID, lifecycleType string,
	payload map[string]any,
	notificationPayload ...map[string]any,
) []domain.Event {
	notification := payload
	if len(notificationPayload) > 0 && notificationPayload[0] != nil {
		notification = notificationPayload[0]
	}
	return []domain.Event{
		newEvent(metadata, aggregateID, lifecycleType, payload),
		newProjectionEvent(
			metadata, aggregateID, domain.EventCalendarSyncRequested, lifecycleType, payload,
		),
		newProjectionEvent(
			metadata,
			aggregateID,
			domain.EventNotificationRequested,
			lifecycleType,
			notification,
		),
	}
}

func bookingNotificationPayload(
	booking domain.Booking,
	trigger string,
	extra map[string]any,
) map[string]any {
	payload := map[string]any{
		"aggregate_type": "booking",
		"aggregate_id":   booking.ID,
		"booking_id":     booking.ID,
		"trigger":        trigger,
		"recipient_e164": booking.CustomerPhone,
		"customer_name":  booking.CustomerName,
		"service_name":   booking.ServiceName,
		"start_at":       booking.StartAt,
		"end_at":         booking.EndAt,
		"timezone":       booking.Timezone,
	}
	for key, value := range extra {
		if value != nil && value != "" {
			payload[key] = value
		}
	}
	return payload
}

func newProjectionEvent(
	metadata domain.CommandMetadata,
	aggregateID, eventType, trigger string,
	payload map[string]any,
) domain.Event {
	projectionPayload := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		projectionPayload[key] = value
	}
	projectionPayload["trigger"] = trigger
	return newEvent(metadata, aggregateID, eventType, projectionPayload)
}

func newEvent(
	metadata domain.CommandMetadata,
	aggregateID, eventType string,
	value map[string]any,
) domain.Event {
	payload, _ := json.Marshal(value)
	digest := sha256.Sum256(payload)
	return domain.Event{
		ID:             uuid.New(),
		OrganizationID: metadata.OrganizationID,
		Type:           eventType,
		AggregateID:    aggregateID,
		Payload:        payload,
		PayloadHash:    hex.EncodeToString(digest[:]),
		IdempotencyKey: fmt.Sprintf("scheduling:%s:%s:%s:%d", eventType, aggregateID, metadata.SourceID, metadata.SourceVersion),
		RequestID:      metadata.RequestID,
		CorrelationID:  metadata.CorrelationID,
		ActorID:        metadata.ActorID,
		SourceVersion:  metadata.SourceVersion,
		AvailableAt:    time.Now().UTC(),
	}
}
