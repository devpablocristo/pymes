package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	PermissionRead    = "scheduling:read"
	PermissionOperate = "scheduling:operate"
	PermissionManage  = "scheduling:manage"
)

type BookingStatus string

const (
	BookingHeld                BookingStatus = "held"
	BookingPendingConfirmation BookingStatus = "pending_confirmation"
	BookingConfirmed           BookingStatus = "confirmed"
	BookingCheckedIn           BookingStatus = "checked_in"
	BookingCompleted           BookingStatus = "completed"
	BookingCancelled           BookingStatus = "cancelled"
	BookingRescheduled         BookingStatus = "rescheduled"
	BookingNoShow              BookingStatus = "no_show"
)

func (s BookingStatus) Active() bool {
	switch s {
	case BookingHeld, BookingPendingConfirmation, BookingConfirmed, BookingCheckedIn:
		return true
	default:
		return false
	}
}

func (s BookingStatus) Valid() bool {
	switch s {
	case BookingHeld,
		BookingPendingConfirmation,
		BookingConfirmed,
		BookingCheckedIn,
		BookingCompleted,
		BookingCancelled,
		BookingRescheduled,
		BookingNoShow:
		return true
	default:
		return false
	}
}

func (s BookingStatus) CanTransition(to BookingStatus) bool {
	allowed := map[BookingStatus][]BookingStatus{
		BookingHeld:                {BookingPendingConfirmation, BookingConfirmed, BookingCancelled},
		BookingPendingConfirmation: {BookingConfirmed, BookingCancelled, BookingRescheduled},
		BookingConfirmed:           {BookingCheckedIn, BookingCancelled, BookingRescheduled, BookingNoShow},
		BookingCheckedIn:           {BookingCompleted, BookingCancelled},
	}
	return slices.Contains(allowed[s], to)
}

type ResourceKind string

const (
	ResourceProfessional ResourceKind = "professional"
	ResourceRoom         ResourceKind = "room"
	ResourceMachine      ResourceKind = "machine"
	ResourceVehicle      ResourceKind = "vehicle"
	ResourceEquipment    ResourceKind = "equipment"
	ResourceGeneric      ResourceKind = "generic"
)

type AllocationMode string

const (
	AllocationCapacity  AllocationMode = "capacity"
	AllocationExclusive AllocationMode = "exclusive"
)

type AvailabilityKind string

const (
	AvailabilityBranch   AvailabilityKind = "branch"
	AvailabilityResource AvailabilityKind = "resource"
)

type ExceptionKind string

const (
	ExceptionHoliday      ExceptionKind = "holiday"
	ExceptionVacation     ExceptionKind = "vacation"
	ExceptionAbsence      ExceptionKind = "absence"
	ExceptionManualBlock  ExceptionKind = "manual"
	ExceptionMaintenance  ExceptionKind = "maintenance"
	ExceptionAvailability ExceptionKind = "availability"
)

type FulfillmentMode string

const (
	FulfillmentInPerson FulfillmentMode = "in_person"
	FulfillmentVirtual  FulfillmentMode = "virtual"
	FulfillmentHybrid   FulfillmentMode = "hybrid"
)

type RecurrenceFrequency string

const (
	RecurrenceDaily  RecurrenceFrequency = "daily"
	RecurrenceWeekly RecurrenceFrequency = "weekly"
)

type WaitlistStatus string

const (
	WaitlistPending   WaitlistStatus = "pending"
	WaitlistOffered   WaitlistStatus = "offered"
	WaitlistAccepted  WaitlistStatus = "accepted"
	WaitlistCancelled WaitlistStatus = "cancelled"
	WaitlistExpired   WaitlistStatus = "expired"
)

type QueueStatus string

const (
	QueueWaiting   QueueStatus = "waiting"
	QueueCalled    QueueStatus = "called"
	QueueServing   QueueStatus = "serving"
	QueueCompleted QueueStatus = "completed"
	QueueNoShow    QueueStatus = "no_show"
	QueueCancelled QueueStatus = "cancelled"
)

type ActionPurpose string

const (
	ActionConfirm        ActionPurpose = "confirm"
	ActionCancel         ActionPurpose = "cancel"
	ActionReschedule     ActionPurpose = "reschedule"
	ActionAcceptWaitlist ActionPurpose = "accept_waitlist"
)

type Branch struct {
	OrganizationID string
	ID             uuid.UUID
	Code           string
	Slug           string
	Name           string
	Timezone       string
	Address        string
	Active         bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (b Branch) Validate() error {
	if strings.TrimSpace(b.OrganizationID) == "" || b.ID == uuid.Nil ||
		strings.TrimSpace(b.Code) == "" || strings.TrimSpace(b.Name) == "" {
		return NewError(CodeValidation, "organization, id, code and name are required")
	}
	if _, err := time.LoadLocation(b.Timezone); err != nil {
		return NewError(CodeValidation, "timezone must be a valid IANA timezone")
	}
	return nil
}

type Service struct {
	OrganizationID       string
	ID                   uuid.UUID
	Code                 string
	Name                 string
	Description          string
	DurationMinutes      int
	BufferBeforeMinutes  int
	BufferAfterMinutes   int
	SlotMinutes          int
	Price                string
	Currency             string
	Mode                 FulfillmentMode
	MaxParticipants      int
	AllowGroup           bool
	AllowWaitlist        bool
	ConfirmationRequired bool
	Active               bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,6})?$`)

func (s Service) Validate() error {
	if strings.TrimSpace(s.OrganizationID) == "" || s.ID == uuid.Nil ||
		strings.TrimSpace(s.Code) == "" || strings.TrimSpace(s.Name) == "" {
		return NewError(CodeValidation, "organization, id, code and name are required")
	}
	if s.DurationMinutes <= 0 || s.DurationMinutes > 1440 || s.SlotMinutes <= 0 ||
		s.BufferBeforeMinutes < 0 || s.BufferAfterMinutes < 0 {
		return NewError(CodeValidation, "duration, slot and buffers are invalid")
	}
	if !decimalPattern.MatchString(s.Price) || len(s.Currency) != 3 {
		return NewError(CodeValidation, "price and ISO currency are required")
	}
	if s.MaxParticipants <= 0 || (!s.AllowGroup && s.MaxParticipants != 1) {
		return NewError(CodeValidation, "participant capacity is invalid")
	}
	switch s.Mode {
	case FulfillmentInPerson, FulfillmentVirtual, FulfillmentHybrid:
	default:
		return NewError(CodeValidation, "fulfillment mode is invalid")
	}
	return nil
}

type Resource struct {
	OrganizationID string
	ID             uuid.UUID
	BranchID       uuid.UUID
	Code           string
	Name           string
	Kind           ResourceKind
	Capacity       int
	Timezone       string
	Active         bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (r Resource) Validate() error {
	if strings.TrimSpace(r.OrganizationID) == "" || r.ID == uuid.Nil || r.BranchID == uuid.Nil ||
		strings.TrimSpace(r.Code) == "" || strings.TrimSpace(r.Name) == "" || r.Capacity <= 0 {
		return NewError(CodeValidation, "resource identity and positive capacity are required")
	}
	switch r.Kind {
	case ResourceProfessional, ResourceRoom, ResourceMachine, ResourceVehicle, ResourceEquipment, ResourceGeneric:
	default:
		return NewError(CodeValidation, "resource kind is invalid")
	}
	if _, err := time.LoadLocation(r.Timezone); err != nil {
		return NewError(CodeValidation, "resource timezone must be a valid IANA timezone")
	}
	return nil
}

type ResourceRequirement struct {
	OrganizationID string
	ID             uuid.UUID
	ServiceID      uuid.UUID
	ResourceID     *uuid.UUID
	Kind           ResourceKind
	Mode           AllocationMode
	Units          int
	Optional       bool
}

type AvailabilityRule struct {
	OrganizationID string
	ID             uuid.UUID
	BranchID       uuid.UUID
	ResourceID     *uuid.UUID
	Kind           AvailabilityKind
	Weekday        time.Weekday
	StartMinute    int
	EndMinute      int
	ValidFrom      *time.Time
	ValidUntil     *time.Time
	Timezone       string
	Active         bool
}

func (r AvailabilityRule) Validate() error {
	if r.OrganizationID == "" || r.ID == uuid.Nil || r.BranchID == uuid.Nil ||
		r.Weekday < time.Sunday || r.Weekday > time.Saturday ||
		r.StartMinute < 0 || r.StartMinute >= 1440 || r.EndMinute < 0 || r.EndMinute >= 2880 ||
		r.StartMinute == r.EndMinute {
		return NewError(CodeValidation, "availability rule is invalid")
	}
	if r.Kind == AvailabilityResource && (r.ResourceID == nil || *r.ResourceID == uuid.Nil) {
		return NewError(CodeValidation, "resource rule requires resource_id")
	}
	if _, err := time.LoadLocation(r.Timezone); err != nil {
		return NewError(CodeValidation, "timezone must be a valid IANA timezone")
	}
	return nil
}

type AvailabilityException struct {
	OrganizationID string
	ID             uuid.UUID
	BranchID       uuid.UUID
	ResourceID     *uuid.UUID
	Kind           ExceptionKind
	StartAt        time.Time
	EndAt          time.Time
	Reason         string
}

func (e AvailabilityException) Blocking() bool { return e.Kind != ExceptionAvailability }

type Allocation struct {
	ResourceID uuid.UUID
	Mode       AllocationMode
	Units      int
}

type Occupancy struct {
	ResourceID  uuid.UUID
	StartAt     time.Time
	EndAt       time.Time
	Units       int
	ServiceID   uuid.UUID
	BookingID   uuid.UUID
	BookingOpen bool
}

type Slot struct {
	StartAt          time.Time
	EndAt            time.Time
	OccupiesFrom     time.Time
	OccupiesUntil    time.Time
	Timezone         string
	Allocations      []Allocation
	Remaining        int
	ServiceRemaining int
}

type AvailabilitySnapshot struct {
	Branch      Branch
	Service     Service
	Resources   []Resource
	Rules       []AvailabilityRule
	Exceptions  []AvailabilityException
	Occupancies []Occupancy
}

type AvailabilityQuery struct {
	OrganizationID string
	BranchID       uuid.UUID
	ServiceID      uuid.UUID
	From           time.Time
	Until          time.Time
	Participants   int
	// DurationMinutes is an internal override used when an operator resizes an
	// existing booking. Public availability always uses the service duration.
	DurationMinutes  int
	Allocations      []Allocation
	ExcludeBookingID *uuid.UUID
}

type Booking struct {
	OrganizationID     string
	ID                 uuid.UUID
	SeriesID           *uuid.UUID
	SessionID          *uuid.UUID
	SupersedesID       *uuid.UUID
	Occurrence         int
	BranchID           uuid.UUID
	ServiceID          uuid.UUID
	PartyID            string
	Status             BookingStatus
	SubstateCode       string
	Participants       int
	StartAt            time.Time
	EndAt              time.Time
	OccupiesFrom       time.Time
	OccupiesUntil      time.Time
	HoldExpiresAt      *time.Time
	Version            int
	ServiceName        string
	Price              string
	Currency           string
	DurationMinutes    int
	Timezone           string
	CustomerName       string
	CustomerEmail      string
	CustomerPhone      string
	Notes              string
	CancellationReason string
	Allocations        []Allocation
	CreatedBy          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (b Booking) Validate() error {
	if b.OrganizationID == "" || b.ID == uuid.Nil || b.BranchID == uuid.Nil ||
		b.ServiceID == uuid.Nil || strings.TrimSpace(b.PartyID) == "" ||
		b.Participants <= 0 || !b.EndAt.After(b.StartAt) ||
		!b.OccupiesUntil.After(b.OccupiesFrom) || b.Version <= 0 {
		return NewError(CodeValidation, "booking identity, party, range and version are required")
	}
	if b.Status == BookingHeld && (b.HoldExpiresAt == nil || !b.HoldExpiresAt.After(b.CreatedAt)) {
		return NewError(CodeValidation, "held booking requires a future expiration")
	}
	if !b.Status.Valid() || (b.SubstateCode != "" && !bookingSubstateCodePattern.MatchString(b.SubstateCode)) {
		return NewError(CodeValidation, "booking state is invalid")
	}
	if !decimalPattern.MatchString(b.Price) || len(b.Currency) != 3 ||
		b.DurationMinutes <= 0 || strings.TrimSpace(b.Timezone) == "" {
		return NewError(CodeValidation, "booking snapshot is incomplete")
	}
	if len(b.Allocations) == 0 && b.SessionID == nil {
		return NewError(CodeValidation, "at least one allocation is required")
	}
	return nil
}

var bookingSubstateCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,39}$`)

func ValidBookingSubstateCode(code string) bool {
	return bookingSubstateCodePattern.MatchString(strings.TrimSpace(code))
}

type BookingSubstateDefinition struct {
	Code      string
	Label     string
	Active    bool
	SortOrder int
}

type BookingStatusConfiguration struct {
	OrganizationID string
	Status         BookingStatus
	Label          string
	Substates      []BookingSubstateDefinition
	UpdatedAt      time.Time
}

func (c BookingStatusConfiguration) Validate() error {
	if strings.TrimSpace(c.OrganizationID) == "" || !c.Status.Valid() {
		return NewError(CodeValidation, "organization and internal booking status are required")
	}
	label := strings.TrimSpace(c.Label)
	if label == "" || utf8.RuneCountInString(label) > 80 {
		return NewError(CodeValidation, "booking status label must contain at most 80 characters")
	}
	if len(c.Substates) > 50 {
		return NewError(CodeValidation, "a booking status supports at most 50 custom substates")
	}
	seen := make(map[string]struct{}, len(c.Substates))
	for _, substate := range c.Substates {
		code := strings.TrimSpace(substate.Code)
		substateLabel := strings.TrimSpace(substate.Label)
		if !ValidBookingSubstateCode(code) || substateLabel == "" ||
			utf8.RuneCountInString(substateLabel) > 80 ||
			substate.SortOrder < 0 || substate.SortOrder > 10000 {
			return NewError(CodeValidation, "booking substate definition is invalid")
		}
		if _, duplicate := seen[code]; duplicate {
			return NewError(CodeValidation, "booking substate codes must be unique")
		}
		seen[code] = struct{}{}
	}
	return nil
}

type GroupSession struct {
	OrganizationID string
	ID             uuid.UUID
	BranchID       uuid.UUID
	ServiceID      uuid.UUID
	StartAt        time.Time
	EndAt          time.Time
	Capacity       int
	Booked         int
	Version        int
	Status         string
}

type GroupParticipant struct {
	OrganizationID string
	SessionID      uuid.UUID
	BookingID      uuid.UUID
	PartyID        string
	Seats          int
	Status         string
}

type RecurrenceRule struct {
	Frequency  RecurrenceFrequency
	Interval   int
	Count      int
	Until      *time.Time
	ByWeekdays []time.Weekday
}

func (r RecurrenceRule) Validate() error {
	if r.Frequency != RecurrenceDaily && r.Frequency != RecurrenceWeekly {
		return NewError(CodeValidation, "recurrence frequency is invalid")
	}
	if r.Interval <= 0 || r.Interval > 365 || r.Count < 0 || r.Count > 500 {
		return NewError(CodeValidation, "recurrence interval/count is invalid")
	}
	if r.Count == 0 && r.Until == nil {
		return NewError(CodeValidation, "recurrence requires count or until")
	}
	return nil
}

type RecurrenceSeries struct {
	OrganizationID string
	ID             uuid.UUID
	Rule           RecurrenceRule
	Timezone       string
	Status         string
	CreatedAt      time.Time
}

type WaitlistEntry struct {
	OrganizationID     string
	ID                 uuid.UUID
	BranchID           uuid.UUID
	ServiceID          uuid.UUID
	PartyID            string
	CustomerName       string
	CustomerEmail      string
	CustomerPhone      string
	PreferredFrom      time.Time
	PreferredUntil     time.Time
	Participants       int
	Status             WaitlistStatus
	OfferExpiresAt     *time.Time
	OfferedStartAt     *time.Time
	OfferedEndAt       *time.Time
	OfferedAllocations []Allocation
	AcceptedBookingID  *uuid.UUID
	Version            int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type QueueTicket struct {
	OrganizationID string
	ID             uuid.UUID
	BranchID       uuid.UUID
	ServiceID      uuid.UUID
	PartyID        string
	Number         int64
	Priority       int
	Status         QueueStatus
	CalledAt       *time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	Version        int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ActionToken struct {
	OrganizationID  string
	ID              uuid.UUID
	BookingID       *uuid.UUID
	WaitlistID      *uuid.UUID
	ResultBookingID *uuid.UUID
	Purpose         ActionPurpose
	TokenHash       string
	ExpiresAt       time.Time
	ConsumedAt      *time.Time
	CreatedAt       time.Time
}

type Event struct {
	ID             uuid.UUID
	OrganizationID string
	Type           string
	AggregateID    string
	Payload        []byte
	PayloadHash    string
	IdempotencyKey string
	RequestID      string
	CorrelationID  string
	ActorID        string
	SourceVersion  int
	AvailableAt    time.Time
}

const (
	EventBookingCreated        = "BookingCreated"
	EventBookingUpdated        = "BookingUpdated"
	EventBookingConfirmed      = "BookingConfirmed"
	EventBookingRescheduled    = "BookingRescheduled"
	EventBookingCancelled      = "BookingCancelled"
	EventBookingCompleted      = "BookingCompleted"
	EventBookingNoShow         = "BookingNoShow"
	EventWaitlistOffered       = "WaitlistOffered"
	EventReminderDue           = "ReminderDue"
	EventCalendarSyncRequested = "CalendarSyncRequested"
	EventNotificationRequested = "NotificationRequested"
)

type CommandMetadata struct {
	OrganizationID string
	IdempotencyKey string
	SourceID       string
	SourceVersion  int
	PayloadHash    string
	RequestID      string
	CorrelationID  string
	ActorID        string
}

type MaintenanceResult struct {
	ExpiredHolds   int
	ReminderEvents int
	WaitlistOffers int
}

func (m CommandMetadata) Validate() error {
	if strings.TrimSpace(m.OrganizationID) == "" || strings.TrimSpace(m.IdempotencyKey) == "" ||
		strings.TrimSpace(m.SourceID) == "" || m.SourceVersion <= 0 ||
		!regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(m.PayloadHash) ||
		strings.TrimSpace(m.RequestID) == "" || strings.TrimSpace(m.CorrelationID) == "" ||
		strings.TrimSpace(m.ActorID) == "" {
		return NewError(CodeValidation, "command metadata is incomplete")
	}
	return nil
}

func ValidateAllocation(a Allocation) error {
	if a.ResourceID == uuid.Nil || a.Units <= 0 {
		return NewError(CodeValidation, "allocation resource and positive units are required")
	}
	if a.Mode != AllocationCapacity && a.Mode != AllocationExclusive {
		return NewError(CodeValidation, "allocation mode is invalid")
	}
	return nil
}

func NormalizeCode(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", "-"))
}

func ValidateRange(startAt, endAt time.Time) error {
	if startAt.IsZero() || !endAt.After(startAt) {
		return NewError(CodeValidation, "time range is invalid")
	}
	if endAt.Sub(startAt) > 31*24*time.Hour {
		return NewError(CodeValidation, "time range is too long")
	}
	return nil
}

func EnsureTimezone(value string) (*time.Location, error) {
	location, err := time.LoadLocation(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("invalid IANA timezone: %w", err)
	}
	return location, nil
}
