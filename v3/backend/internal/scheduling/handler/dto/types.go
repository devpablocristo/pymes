package dto

import (
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
)

type Allocation struct {
	ResourceID uuid.UUID             `json:"resource_id"`
	Mode       domain.AllocationMode `json:"mode"`
	Units      int                   `json:"units"`
}

func (a Allocation) Domain() domain.Allocation {
	return domain.Allocation{ResourceID: a.ResourceID, Mode: a.Mode, Units: a.Units}
}

func Allocations(values []Allocation) []domain.Allocation {
	result := make([]domain.Allocation, 0, len(values))
	for _, value := range values {
		result = append(result, value.Domain())
	}
	return result
}

func AllocationsFromDomain(values []domain.Allocation) []Allocation {
	result := make([]Allocation, 0, len(values))
	for _, value := range values {
		result = append(result, Allocation{
			ResourceID: value.ResourceID,
			Mode:       value.Mode,
			Units:      value.Units,
		})
	}
	return result
}

type CreateBranch struct {
	ID       uuid.UUID `json:"id"`
	Code     string    `json:"code"`
	Slug     string    `json:"slug"`
	Name     string    `json:"name"`
	Timezone string    `json:"timezone"`
	Address  string    `json:"address"`
	Active   *bool     `json:"active,omitempty"`
}

type CreateService struct {
	ID                   uuid.UUID              `json:"id"`
	Code                 string                 `json:"code"`
	Name                 string                 `json:"name"`
	Description          string                 `json:"description"`
	DurationMinutes      int                    `json:"duration_minutes"`
	BufferBeforeMinutes  int                    `json:"buffer_before_minutes"`
	BufferAfterMinutes   int                    `json:"buffer_after_minutes"`
	SlotMinutes          int                    `json:"slot_minutes"`
	Price                string                 `json:"price"`
	Currency             string                 `json:"currency"`
	FulfillmentMode      domain.FulfillmentMode `json:"fulfillment_mode"`
	MaxParticipants      int                    `json:"max_participants"`
	AllowGroup           bool                   `json:"allow_group"`
	AllowWaitlist        bool                   `json:"allow_waitlist"`
	ConfirmationRequired bool                   `json:"confirmation_required"`
	Active               *bool                  `json:"active,omitempty"`
	Requirements         []ResourceRequirement  `json:"resource_requirements"`
}

type ResourceRequirement struct {
	ID         uuid.UUID             `json:"id"`
	ResourceID *uuid.UUID            `json:"resource_id,omitempty"`
	Kind       domain.ResourceKind   `json:"resource_kind"`
	Mode       domain.AllocationMode `json:"allocation_mode"`
	Units      int                   `json:"units"`
	Optional   bool                  `json:"optional"`
}

type CreateResource struct {
	ID       uuid.UUID           `json:"id"`
	BranchID uuid.UUID           `json:"branch_id"`
	Code     string              `json:"code"`
	Name     string              `json:"name"`
	Kind     domain.ResourceKind `json:"kind"`
	Capacity int                 `json:"capacity"`
	Timezone string              `json:"timezone"`
	Active   *bool               `json:"active,omitempty"`
}

type CreateAvailabilityRule struct {
	ID          uuid.UUID               `json:"id"`
	BranchID    uuid.UUID               `json:"branch_id"`
	ResourceID  *uuid.UUID              `json:"resource_id,omitempty"`
	Kind        domain.AvailabilityKind `json:"kind"`
	Weekday     int                     `json:"weekday"`
	StartMinute int                     `json:"start_minute"`
	EndMinute   int                     `json:"end_minute"`
	ValidFrom   *time.Time              `json:"valid_from,omitempty"`
	ValidUntil  *time.Time              `json:"valid_until,omitempty"`
	Timezone    string                  `json:"timezone"`
	Active      *bool                   `json:"active,omitempty"`
}

type CreateBlock struct {
	ID         uuid.UUID            `json:"id"`
	BranchID   uuid.UUID            `json:"branch_id"`
	ResourceID *uuid.UUID           `json:"resource_id,omitempty"`
	Kind       domain.ExceptionKind `json:"kind"`
	StartAt    time.Time            `json:"start_at"`
	EndAt      time.Time            `json:"end_at"`
	Reason     string               `json:"reason"`
}

type Customer struct {
	PartyID string `json:"party_id,omitempty"`
	Name    string `json:"name"`
	Email   string `json:"email,omitempty"`
	Phone   string `json:"phone,omitempty"`
}

type PublicCatalog struct {
	Branches  []PublicBranch   `json:"branches"`
	Services  []PublicService  `json:"services"`
	Resources []PublicResource `json:"resources"`
}

type PublicBranch struct {
	ID       uuid.UUID `json:"id"`
	Slug     string    `json:"slug"`
	Name     string    `json:"name"`
	Timezone string    `json:"timezone"`
	Address  string    `json:"address"`
}

type PublicService struct {
	ID                   uuid.UUID              `json:"id"`
	Code                 string                 `json:"code"`
	Name                 string                 `json:"name"`
	Description          string                 `json:"description"`
	DurationMinutes      int                    `json:"duration_minutes"`
	BufferBeforeMinutes  int                    `json:"buffer_before_minutes"`
	BufferAfterMinutes   int                    `json:"buffer_after_minutes"`
	SlotMinutes          int                    `json:"slot_minutes"`
	Price                string                 `json:"price"`
	Currency             string                 `json:"currency"`
	FulfillmentMode      domain.FulfillmentMode `json:"fulfillment_mode"`
	MaxParticipants      int                    `json:"max_participants"`
	AllowGroup           bool                   `json:"allow_group"`
	AllowWaitlist        bool                   `json:"allow_waitlist"`
	ConfirmationRequired bool                   `json:"confirmation_required"`
}

type PublicResource struct {
	ID       uuid.UUID           `json:"id"`
	BranchID uuid.UUID           `json:"branch_id"`
	Name     string              `json:"name"`
	Kind     domain.ResourceKind `json:"kind"`
	Capacity int                 `json:"capacity"`
	Timezone string              `json:"timezone"`
}

func PublicCatalogFromDomain(
	branches []domain.Branch,
	services []domain.Service,
	resources []domain.Resource,
) PublicCatalog {
	result := PublicCatalog{
		Branches:  make([]PublicBranch, 0, len(branches)),
		Services:  make([]PublicService, 0, len(services)),
		Resources: make([]PublicResource, 0, len(resources)),
	}
	for _, branch := range branches {
		result.Branches = append(result.Branches, PublicBranch{
			ID: branch.ID, Slug: branch.Slug, Name: branch.Name,
			Timezone: branch.Timezone, Address: branch.Address,
		})
	}
	for _, service := range services {
		result.Services = append(result.Services, PublicService{
			ID: service.ID, Code: service.Code, Name: service.Name,
			Description: service.Description, DurationMinutes: service.DurationMinutes,
			BufferBeforeMinutes: service.BufferBeforeMinutes,
			BufferAfterMinutes:  service.BufferAfterMinutes,
			SlotMinutes:         service.SlotMinutes, Price: service.Price,
			Currency: service.Currency, FulfillmentMode: service.Mode,
			MaxParticipants: service.MaxParticipants, AllowGroup: service.AllowGroup,
			AllowWaitlist:        service.AllowWaitlist,
			ConfirmationRequired: service.ConfirmationRequired,
		})
	}
	for _, resource := range resources {
		result.Resources = append(result.Resources, PublicResource{
			ID: resource.ID, BranchID: resource.BranchID, Name: resource.Name,
			Kind: resource.Kind, Capacity: resource.Capacity, Timezone: resource.Timezone,
		})
	}
	return result
}

type Recurrence struct {
	Frequency  domain.RecurrenceFrequency `json:"frequency"`
	Interval   int                        `json:"interval"`
	Count      int                        `json:"count,omitempty"`
	Until      *time.Time                 `json:"until,omitempty"`
	ByWeekdays []int                      `json:"by_weekdays,omitempty"`
}

func (r *Recurrence) Domain() *domain.RecurrenceRule {
	if r == nil {
		return nil
	}
	weekdays := make([]time.Weekday, 0, len(r.ByWeekdays))
	for _, value := range r.ByWeekdays {
		weekdays = append(weekdays, time.Weekday(value))
	}
	return &domain.RecurrenceRule{
		Frequency: r.Frequency, Interval: r.Interval, Count: r.Count,
		Until: r.Until, ByWeekdays: weekdays,
	}
}

type CreateBooking struct {
	ID            uuid.UUID            `json:"id,omitempty"`
	BranchID      uuid.UUID            `json:"branch_id"`
	ServiceID     uuid.UUID            `json:"service_id"`
	SessionID     *uuid.UUID           `json:"session_id,omitempty"`
	Customer      Customer             `json:"customer"`
	StartAt       time.Time            `json:"start_at"`
	Participants  int                  `json:"participants"`
	Status        domain.BookingStatus `json:"status,omitempty"`
	HoldMinutes   int                  `json:"hold_minutes,omitempty"`
	Allocations   []Allocation         `json:"allocations,omitempty"`
	MeetRequested bool                 `json:"meet_requested,omitempty"`
	Notes         string               `json:"notes,omitempty"`
	Recurrence    *Recurrence          `json:"recurrence,omitempty"`
}

type CreateGroupSession struct {
	ID          uuid.UUID    `json:"id,omitempty"`
	BranchID    uuid.UUID    `json:"branch_id"`
	ServiceID   uuid.UUID    `json:"service_id"`
	StartAt     time.Time    `json:"start_at"`
	Capacity    int          `json:"capacity"`
	Allocations []Allocation `json:"allocations"`
}

type AvailabilityQuery struct {
	BranchID     uuid.UUID    `json:"branch_id"`
	ServiceID    uuid.UUID    `json:"service_id"`
	From         time.Time    `json:"from"`
	Until        time.Time    `json:"until"`
	Participants int          `json:"participants"`
	Allocations  []Allocation `json:"allocations"`
}

type Reschedule struct {
	ExpectedVersion int          `json:"expected_version"`
	StartAt         time.Time    `json:"start_at"`
	DurationMinutes int          `json:"duration_minutes,omitempty"`
	Allocations     []Allocation `json:"allocations,omitempty"`
}

type UpdateBooking struct {
	ExpectedVersion int       `json:"expected_version"`
	Customer        *Customer `json:"customer,omitempty"`
	Participants    *int      `json:"participants,omitempty"`
	Notes           *string   `json:"notes,omitempty"`
	SubstateCode    *string   `json:"substate_code,omitempty"`
}

// UpdateBookingIdempotencyScope is the canonical command payload hashed by the
// HTTP adapter. The route resource is part of the scope so one key can never
// replay an update completed for another booking.
type UpdateBookingIdempotencyScope struct {
	BookingID uuid.UUID     `json:"booking_id"`
	Body      UpdateBooking `json:"body"`
}

type Transition struct {
	ExpectedVersion int    `json:"expected_version"`
	Reason          string `json:"reason,omitempty"`
}

type BookingSubstateDefinition struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	Active    bool   `json:"active"`
	SortOrder int    `json:"sort_order"`
}

type ConfigureBookingStatus struct {
	Label     string                      `json:"label"`
	Substates []BookingSubstateDefinition `json:"substates"`
}

type BookingStatusConfiguration struct {
	Status    domain.BookingStatus        `json:"status"`
	Label     string                      `json:"label"`
	Substates []BookingSubstateDefinition `json:"substates"`
	UpdatedAt time.Time                   `json:"updated_at"`
}

func BookingStatusConfigurationFromDomain(
	value domain.BookingStatusConfiguration,
) BookingStatusConfiguration {
	substates := make([]BookingSubstateDefinition, 0, len(value.Substates))
	for _, substate := range value.Substates {
		substates = append(substates, BookingSubstateDefinition{
			Code:      substate.Code,
			Label:     substate.Label,
			Active:    substate.Active,
			SortOrder: substate.SortOrder,
		})
	}
	return BookingStatusConfiguration{
		Status: value.Status, Label: value.Label, Substates: substates, UpdatedAt: value.UpdatedAt,
	}
}

func BookingStatusConfigurationsFromDomain(
	values []domain.BookingStatusConfiguration,
) []BookingStatusConfiguration {
	result := make([]BookingStatusConfiguration, 0, len(values))
	for _, value := range values {
		result = append(result, BookingStatusConfigurationFromDomain(value))
	}
	return result
}

type SetBookingSubstate struct {
	ExpectedVersion int    `json:"expected_version"`
	SubstateCode    string `json:"substate_code"`
}

type Action struct {
	Purpose         domain.ActionPurpose `json:"purpose"`
	ExpectedVersion int                  `json:"expected_version"`
	StartAt         *time.Time           `json:"start_at,omitempty"`
	DurationMinutes int                  `json:"duration_minutes,omitempty"`
	Reason          string               `json:"reason,omitempty"`
}

type CreateWaitlist struct {
	ID             uuid.UUID `json:"id"`
	BranchID       uuid.UUID `json:"branch_id"`
	ServiceID      uuid.UUID `json:"service_id"`
	Customer       Customer  `json:"customer"`
	PreferredFrom  time.Time `json:"preferred_from"`
	PreferredUntil time.Time `json:"preferred_until"`
	Participants   int       `json:"participants"`
	MeetRequested  bool      `json:"meet_requested,omitempty"`
}

type CreateQueueTicket struct {
	ID        uuid.UUID `json:"id"`
	BranchID  uuid.UUID `json:"branch_id"`
	ServiceID uuid.UUID `json:"service_id"`
	PartyID   string    `json:"party_id"`
	Priority  int       `json:"priority"`
}

type AdvanceQueueTicket struct {
	ExpectedVersion int                `json:"expected_version"`
	Status          domain.QueueStatus `json:"status"`
}

type Booking struct {
	ID                 uuid.UUID            `json:"id"`
	SeriesID           *uuid.UUID           `json:"series_id,omitempty"`
	SessionID          *uuid.UUID           `json:"session_id,omitempty"`
	SupersedesID       *uuid.UUID           `json:"supersedes_id,omitempty"`
	BranchID           uuid.UUID            `json:"branch_id"`
	ServiceID          uuid.UUID            `json:"service_id"`
	PartyID            string               `json:"party_id"`
	Status             domain.BookingStatus `json:"status"`
	SubstateCode       string               `json:"substate_code,omitempty"`
	Participants       int                  `json:"participants"`
	StartAt            time.Time            `json:"start_at"`
	EndAt              time.Time            `json:"end_at"`
	Version            int                  `json:"version"`
	ServiceName        string               `json:"service_name"`
	Price              string               `json:"price"`
	Currency           string               `json:"currency"`
	DurationMinutes    int                  `json:"duration_minutes"`
	Timezone           string               `json:"timezone"`
	CustomerName       string               `json:"customer_name"`
	CustomerEmail      string               `json:"customer_email,omitempty"`
	CustomerPhone      string               `json:"customer_phone,omitempty"`
	MeetRequested      bool                 `json:"meet_requested"`
	Notes              string               `json:"notes,omitempty"`
	CancellationReason string               `json:"cancellation_reason,omitempty"`
	Allocations        []domain.Allocation  `json:"allocations"`
}

func BookingFromDomain(value domain.Booking) Booking {
	return Booking{
		ID: value.ID, SeriesID: value.SeriesID, SessionID: value.SessionID,
		SupersedesID: value.SupersedesID, BranchID: value.BranchID,
		ServiceID: value.ServiceID, PartyID: value.PartyID, Status: value.Status,
		SubstateCode: value.SubstateCode,
		Participants: value.Participants, StartAt: value.StartAt, EndAt: value.EndAt,
		Version: value.Version, ServiceName: value.ServiceName, Price: value.Price,
		Currency: value.Currency, DurationMinutes: value.DurationMinutes,
		Timezone: value.Timezone, CustomerName: value.CustomerName,
		CustomerEmail: value.CustomerEmail, CustomerPhone: value.CustomerPhone,
		MeetRequested: value.MeetRequested,
		Notes:         value.Notes, CancellationReason: value.CancellationReason,
		Allocations: value.Allocations,
	}
}

type PublicBooking struct {
	ID              uuid.UUID            `json:"id"`
	SeriesID        *uuid.UUID           `json:"series_id,omitempty"`
	SessionID       *uuid.UUID           `json:"session_id,omitempty"`
	SupersedesID    *uuid.UUID           `json:"supersedes_id,omitempty"`
	BranchID        uuid.UUID            `json:"branch_id"`
	ServiceID       uuid.UUID            `json:"service_id"`
	Status          domain.BookingStatus `json:"status"`
	Participants    int                  `json:"participants"`
	StartAt         time.Time            `json:"start_at"`
	EndAt           time.Time            `json:"end_at"`
	Version         int                  `json:"version"`
	ServiceName     string               `json:"service_name"`
	Price           string               `json:"price"`
	Currency        string               `json:"currency"`
	DurationMinutes int                  `json:"duration_minutes"`
	Timezone        string               `json:"timezone"`
	MeetRequested   bool                 `json:"meet_requested"`
}

func PublicBookingFromDomain(value domain.Booking) PublicBooking {
	return PublicBooking{
		ID: value.ID, SeriesID: value.SeriesID, SessionID: value.SessionID,
		SupersedesID: value.SupersedesID, BranchID: value.BranchID,
		ServiceID: value.ServiceID, Status: value.Status,
		Participants: value.Participants, StartAt: value.StartAt, EndAt: value.EndAt,
		Version: value.Version, ServiceName: value.ServiceName, Price: value.Price,
		Currency: value.Currency, DurationMinutes: value.DurationMinutes,
		Timezone: value.Timezone, MeetRequested: value.MeetRequested,
	}
}

func PublicBookingsFromDomain(values []domain.Booking) []PublicBooking {
	result := make([]PublicBooking, 0, len(values))
	for _, value := range values {
		result = append(result, PublicBookingFromDomain(value))
	}
	return result
}

func BookingsFromDomain(values []domain.Booking) []Booking {
	result := make([]Booking, 0, len(values))
	for _, value := range values {
		result = append(result, BookingFromDomain(value))
	}
	return result
}

type Branch struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Code           string    `json:"code"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	Timezone       string    `json:"timezone"`
	Address        string    `json:"address"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func BranchFromDomain(value domain.Branch) Branch {
	return Branch{
		ID: value.ID, OrganizationID: value.OrganizationID, Code: value.Code,
		Slug: value.Slug, Name: value.Name, Timezone: value.Timezone,
		Address: value.Address, Active: value.Active,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func BranchesFromDomain(values []domain.Branch) []Branch {
	result := make([]Branch, 0, len(values))
	for _, value := range values {
		result = append(result, BranchFromDomain(value))
	}
	return result
}

type Service struct {
	ID                   uuid.UUID              `json:"id"`
	OrganizationID       string                 `json:"organization_id"`
	Code                 string                 `json:"code"`
	Name                 string                 `json:"name"`
	Description          string                 `json:"description"`
	DurationMinutes      int                    `json:"duration_minutes"`
	BufferBeforeMinutes  int                    `json:"buffer_before_minutes"`
	BufferAfterMinutes   int                    `json:"buffer_after_minutes"`
	SlotMinutes          int                    `json:"slot_minutes"`
	Price                string                 `json:"price"`
	Currency             string                 `json:"currency"`
	FulfillmentMode      domain.FulfillmentMode `json:"fulfillment_mode"`
	MaxParticipants      int                    `json:"max_participants"`
	AllowGroup           bool                   `json:"allow_group"`
	AllowWaitlist        bool                   `json:"allow_waitlist"`
	ConfirmationRequired bool                   `json:"confirmation_required"`
	Active               bool                   `json:"active"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

func ServiceFromDomain(value domain.Service) Service {
	return Service{
		ID: value.ID, OrganizationID: value.OrganizationID, Code: value.Code,
		Name: value.Name, Description: value.Description,
		DurationMinutes:     value.DurationMinutes,
		BufferBeforeMinutes: value.BufferBeforeMinutes,
		BufferAfterMinutes:  value.BufferAfterMinutes, SlotMinutes: value.SlotMinutes,
		Price: value.Price, Currency: value.Currency, FulfillmentMode: value.Mode,
		MaxParticipants: value.MaxParticipants, AllowGroup: value.AllowGroup,
		AllowWaitlist:        value.AllowWaitlist,
		ConfirmationRequired: value.ConfirmationRequired, Active: value.Active,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func ServicesFromDomain(values []domain.Service) []Service {
	result := make([]Service, 0, len(values))
	for _, value := range values {
		result = append(result, ServiceFromDomain(value))
	}
	return result
}

type Resource struct {
	ID             uuid.UUID           `json:"id"`
	OrganizationID string              `json:"organization_id"`
	BranchID       uuid.UUID           `json:"branch_id"`
	Code           string              `json:"code"`
	Name           string              `json:"name"`
	Kind           domain.ResourceKind `json:"kind"`
	Capacity       int                 `json:"capacity"`
	Timezone       string              `json:"timezone"`
	Active         bool                `json:"active"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

func ResourceFromDomain(value domain.Resource) Resource {
	return Resource{
		ID: value.ID, OrganizationID: value.OrganizationID, BranchID: value.BranchID,
		Code: value.Code, Name: value.Name, Kind: value.Kind, Capacity: value.Capacity,
		Timezone: value.Timezone, Active: value.Active,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func ResourcesFromDomain(values []domain.Resource) []Resource {
	result := make([]Resource, 0, len(values))
	for _, value := range values {
		result = append(result, ResourceFromDomain(value))
	}
	return result
}

type AvailabilityRule struct {
	ID             uuid.UUID               `json:"id"`
	OrganizationID string                  `json:"organization_id"`
	BranchID       uuid.UUID               `json:"branch_id"`
	ResourceID     *uuid.UUID              `json:"resource_id,omitempty"`
	Kind           domain.AvailabilityKind `json:"kind"`
	Weekday        int                     `json:"weekday"`
	StartMinute    int                     `json:"start_minute"`
	EndMinute      int                     `json:"end_minute"`
	ValidFrom      *time.Time              `json:"valid_from,omitempty"`
	ValidUntil     *time.Time              `json:"valid_until,omitempty"`
	Timezone       string                  `json:"timezone"`
	Active         bool                    `json:"active"`
}

func AvailabilityRuleFromDomain(value domain.AvailabilityRule) AvailabilityRule {
	return AvailabilityRule{
		ID: value.ID, OrganizationID: value.OrganizationID, BranchID: value.BranchID,
		ResourceID: value.ResourceID, Kind: value.Kind, Weekday: int(value.Weekday),
		StartMinute: value.StartMinute, EndMinute: value.EndMinute,
		ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil,
		Timezone: value.Timezone, Active: value.Active,
	}
}

func AvailabilityRulesFromDomain(values []domain.AvailabilityRule) []AvailabilityRule {
	result := make([]AvailabilityRule, 0, len(values))
	for _, value := range values {
		result = append(result, AvailabilityRuleFromDomain(value))
	}
	return result
}

type AvailabilityException struct {
	ID             uuid.UUID            `json:"id"`
	OrganizationID string               `json:"organization_id"`
	BranchID       uuid.UUID            `json:"branch_id"`
	ResourceID     *uuid.UUID           `json:"resource_id,omitempty"`
	Kind           domain.ExceptionKind `json:"kind"`
	StartAt        time.Time            `json:"start_at"`
	EndAt          time.Time            `json:"end_at"`
	Reason         string               `json:"reason"`
}

func AvailabilityExceptionFromDomain(value domain.AvailabilityException) AvailabilityException {
	return AvailabilityException{
		ID: value.ID, OrganizationID: value.OrganizationID, BranchID: value.BranchID,
		ResourceID: value.ResourceID, Kind: value.Kind, StartAt: value.StartAt,
		EndAt: value.EndAt, Reason: value.Reason,
	}
}

func AvailabilityExceptionsFromDomain(values []domain.AvailabilityException) []AvailabilityException {
	result := make([]AvailabilityException, 0, len(values))
	for _, value := range values {
		result = append(result, AvailabilityExceptionFromDomain(value))
	}
	return result
}

type Slot struct {
	StartAt          time.Time    `json:"start_at"`
	EndAt            time.Time    `json:"end_at"`
	OccupiesFrom     time.Time    `json:"occupies_from"`
	OccupiesUntil    time.Time    `json:"occupies_until"`
	Timezone         string       `json:"timezone"`
	Allocations      []Allocation `json:"allocations"`
	Remaining        int          `json:"remaining"`
	ServiceRemaining int          `json:"service_remaining"`
}

func SlotsFromDomain(values []domain.Slot) []Slot {
	result := make([]Slot, 0, len(values))
	for _, value := range values {
		allocations := make([]Allocation, 0, len(value.Allocations))
		for _, allocation := range value.Allocations {
			allocations = append(allocations, Allocation{
				ResourceID: allocation.ResourceID, Mode: allocation.Mode, Units: allocation.Units,
			})
		}
		result = append(result, Slot{
			StartAt: value.StartAt, EndAt: value.EndAt,
			OccupiesFrom: value.OccupiesFrom, OccupiesUntil: value.OccupiesUntil,
			Timezone: value.Timezone, Allocations: allocations,
			Remaining: value.Remaining, ServiceRemaining: value.ServiceRemaining,
		})
	}
	return result
}

type GroupSession struct {
	ID        uuid.UUID `json:"id"`
	BranchID  uuid.UUID `json:"branch_id"`
	ServiceID uuid.UUID `json:"service_id"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	Capacity  int       `json:"capacity"`
	Booked    int       `json:"booked"`
	Version   int       `json:"version"`
	Status    string    `json:"status"`
}

func GroupSessionFromDomain(value domain.GroupSession) GroupSession {
	return GroupSession{
		ID: value.ID, BranchID: value.BranchID, ServiceID: value.ServiceID,
		StartAt: value.StartAt, EndAt: value.EndAt, Capacity: value.Capacity,
		Booked: value.Booked, Version: value.Version, Status: value.Status,
	}
}

type WaitlistEntry struct {
	ID                 uuid.UUID             `json:"id"`
	BranchID           uuid.UUID             `json:"branch_id"`
	ServiceID          uuid.UUID             `json:"service_id"`
	PartyID            string                `json:"party_id"`
	CustomerName       string                `json:"customer_name"`
	CustomerEmail      string                `json:"customer_email,omitempty"`
	CustomerPhone      string                `json:"customer_phone,omitempty"`
	PreferredFrom      time.Time             `json:"preferred_from"`
	PreferredUntil     time.Time             `json:"preferred_until"`
	Participants       int                   `json:"participants"`
	MeetRequested      bool                  `json:"meet_requested"`
	Status             domain.WaitlistStatus `json:"status"`
	OfferExpiresAt     *time.Time            `json:"offer_expires_at,omitempty"`
	OfferedStartAt     *time.Time            `json:"offered_start_at,omitempty"`
	OfferedEndAt       *time.Time            `json:"offered_end_at,omitempty"`
	OfferedAllocations []Allocation          `json:"offered_allocations,omitempty"`
	AcceptedBookingID  *uuid.UUID            `json:"accepted_booking_id,omitempty"`
	Version            int                   `json:"version"`
}

func WaitlistEntryFromDomain(value domain.WaitlistEntry) WaitlistEntry {
	return WaitlistEntry{
		ID: value.ID, BranchID: value.BranchID, ServiceID: value.ServiceID,
		PartyID: value.PartyID, CustomerName: value.CustomerName,
		CustomerEmail: value.CustomerEmail, CustomerPhone: value.CustomerPhone,
		PreferredFrom:  value.PreferredFrom,
		PreferredUntil: value.PreferredUntil, Participants: value.Participants,
		MeetRequested: value.MeetRequested,
		Status:        value.Status, OfferExpiresAt: value.OfferExpiresAt,
		OfferedStartAt: value.OfferedStartAt, OfferedEndAt: value.OfferedEndAt,
		OfferedAllocations: AllocationsFromDomain(value.OfferedAllocations),
		AcceptedBookingID:  value.AcceptedBookingID, Version: value.Version,
	}
}

type PublicWaitlistEntry struct {
	ID                 uuid.UUID             `json:"id"`
	BranchID           uuid.UUID             `json:"branch_id"`
	ServiceID          uuid.UUID             `json:"service_id"`
	PreferredFrom      time.Time             `json:"preferred_from"`
	PreferredUntil     time.Time             `json:"preferred_until"`
	Participants       int                   `json:"participants"`
	MeetRequested      bool                  `json:"meet_requested"`
	Status             domain.WaitlistStatus `json:"status"`
	OfferExpiresAt     *time.Time            `json:"offer_expires_at,omitempty"`
	OfferedStartAt     *time.Time            `json:"offered_start_at,omitempty"`
	OfferedEndAt       *time.Time            `json:"offered_end_at,omitempty"`
	OfferedAllocations []Allocation          `json:"offered_allocations,omitempty"`
	AcceptedBookingID  *uuid.UUID            `json:"accepted_booking_id,omitempty"`
	Version            int                   `json:"version"`
}

func PublicWaitlistEntryFromDomain(value domain.WaitlistEntry) PublicWaitlistEntry {
	return PublicWaitlistEntry{
		ID: value.ID, BranchID: value.BranchID, ServiceID: value.ServiceID,
		PreferredFrom: value.PreferredFrom, PreferredUntil: value.PreferredUntil,
		Participants: value.Participants, Status: value.Status,
		MeetRequested:  value.MeetRequested,
		OfferExpiresAt: value.OfferExpiresAt, OfferedStartAt: value.OfferedStartAt,
		OfferedEndAt:       value.OfferedEndAt,
		OfferedAllocations: AllocationsFromDomain(value.OfferedAllocations),
		AcceptedBookingID:  value.AcceptedBookingID, Version: value.Version,
	}
}

func WaitlistFromDomain(values []domain.WaitlistEntry) []WaitlistEntry {
	result := make([]WaitlistEntry, 0, len(values))
	for _, value := range values {
		result = append(result, WaitlistEntryFromDomain(value))
	}
	return result
}

type QueueTicket struct {
	ID        uuid.UUID          `json:"id"`
	BranchID  uuid.UUID          `json:"branch_id"`
	ServiceID uuid.UUID          `json:"service_id"`
	PartyID   string             `json:"party_id"`
	Number    int64              `json:"number"`
	Priority  int                `json:"priority"`
	Status    domain.QueueStatus `json:"status"`
	Version   int                `json:"version"`
}

func QueueTicketFromDomain(value domain.QueueTicket) QueueTicket {
	return QueueTicket{
		ID: value.ID, BranchID: value.BranchID, ServiceID: value.ServiceID,
		PartyID: value.PartyID, Number: value.Number, Priority: value.Priority,
		Status: value.Status, Version: value.Version,
	}
}

func QueueFromDomain(values []domain.QueueTicket) []QueueTicket {
	result := make([]QueueTicket, 0, len(values))
	for _, value := range values {
		result = append(result, QueueTicketFromDomain(value))
	}
	return result
}
