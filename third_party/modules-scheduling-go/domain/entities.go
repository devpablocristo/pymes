package domain

import (
	"time"

	"github.com/google/uuid"
)

type FulfillmentMode string

const (
	FulfillmentModeSchedule FulfillmentMode = "schedule"
	FulfillmentModeQueue    FulfillmentMode = "queue"
	FulfillmentModeHybrid   FulfillmentMode = "hybrid"
)

type ResourceKind string

const (
	ResourceKindProfessional ResourceKind = "professional"
	ResourceKindDesk         ResourceKind = "desk"
	ResourceKindCounter      ResourceKind = "counter"
	ResourceKindBox          ResourceKind = "box"
	ResourceKindRoom         ResourceKind = "room"
	ResourceKindGeneric      ResourceKind = "generic"
)

type AvailabilityRuleKind string

const (
	AvailabilityRuleKindBranch   AvailabilityRuleKind = "branch"
	AvailabilityRuleKindResource AvailabilityRuleKind = "resource"
)

type BlockedRangeKind string

const (
	BlockedRangeKindHoliday     BlockedRangeKind = "holiday"
	BlockedRangeKindManual      BlockedRangeKind = "manual"
	BlockedRangeKindMaintenance BlockedRangeKind = "maintenance"
	BlockedRangeKindLeave       BlockedRangeKind = "leave"
)

type BookingStatus string

const (
	BookingStatusHold                BookingStatus = "hold"
	BookingStatusPendingConfirmation BookingStatus = "pending_confirmation"
	BookingStatusConfirmed           BookingStatus = "confirmed"
	BookingStatusCheckedIn           BookingStatus = "checked_in"
	BookingStatusInService           BookingStatus = "in_service"
	BookingStatusCompleted           BookingStatus = "completed"
	BookingStatusCancelled           BookingStatus = "cancelled"
	BookingStatusNoShow              BookingStatus = "no_show"
	BookingStatusExpired             BookingStatus = "expired"
)

type BookingSource string

const (
	BookingSourceAdmin     BookingSource = "admin"
	BookingSourcePublicWeb BookingSource = "public_web"
	BookingSourceWhatsApp  BookingSource = "whatsapp"
	BookingSourceAPI       BookingSource = "api"
)

type BookingActionType string

const (
	BookingActionConfirm BookingActionType = "confirm"
	BookingActionCancel  BookingActionType = "cancel"
)

type QueueStatus string

const (
	QueueStatusActive QueueStatus = "active"
	QueueStatusPaused QueueStatus = "paused"
	QueueStatusClosed QueueStatus = "closed"
)

type QueueStrategy string

const (
	QueueStrategyFIFO     QueueStrategy = "fifo"
	QueueStrategyPriority QueueStrategy = "priority"
)

type QueueTicketStatus string

const (
	QueueTicketStatusWaiting   QueueTicketStatus = "waiting"
	QueueTicketStatusCalled    QueueTicketStatus = "called"
	QueueTicketStatusServing   QueueTicketStatus = "serving"
	QueueTicketStatusCompleted QueueTicketStatus = "completed"
	QueueTicketStatusNoShow    QueueTicketStatus = "no_show"
	QueueTicketStatusCancelled QueueTicketStatus = "cancelled"
)

type QueueTicketSource string

const (
	QueueTicketSourceReception QueueTicketSource = "reception"
	QueueTicketSourceWeb       QueueTicketSource = "web"
	QueueTicketSourceWhatsApp  QueueTicketSource = "whatsapp"
	QueueTicketSourceAPI       QueueTicketSource = "api"
)

type WaitlistStatus string

const (
	WaitlistStatusPending   WaitlistStatus = "pending"
	WaitlistStatusNotified  WaitlistStatus = "notified"
	WaitlistStatusBooked    WaitlistStatus = "booked"
	WaitlistStatusCancelled WaitlistStatus = "cancelled"
	WaitlistStatusExpired   WaitlistStatus = "expired"
)

type WaitlistSource string

const (
	WaitlistSourceAdmin     WaitlistSource = "admin"
	WaitlistSourcePublicWeb WaitlistSource = "public_web"
	WaitlistSourceWhatsApp  WaitlistSource = "whatsapp"
	WaitlistSourceAPI       WaitlistSource = "api"
)

type Branch struct {
	ID        uuid.UUID      `json:"id"`
	OrgID     uuid.UUID      `json:"org_id"`
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Timezone  string         `json:"timezone"`
	Address   string         `json:"address"`
	Active    bool           `json:"active"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type Service struct {
	ID                     uuid.UUID       `json:"id"`
	OrgID                  uuid.UUID       `json:"org_id"`
	CommercialServiceID    *uuid.UUID      `json:"commercial_service_id,omitempty"`
	Code                   string          `json:"code"`
	Name                   string          `json:"name"`
	Description            string          `json:"description"`
	FulfillmentMode        FulfillmentMode `json:"fulfillment_mode"`
	DefaultDurationMinutes int             `json:"default_duration_minutes"`
	BufferBeforeMinutes    int             `json:"buffer_before_minutes"`
	BufferAfterMinutes     int             `json:"buffer_after_minutes"`
	SlotGranularityMinutes int             `json:"slot_granularity_minutes"`
	MaxConcurrentBookings  int             `json:"max_concurrent_bookings"`
	MinCancelNoticeMinutes int             `json:"min_cancel_notice_minutes"`
	AllowWaitlist          bool            `json:"allow_waitlist"`
	Active                 bool            `json:"active"`
	ResourceIDs            []uuid.UUID     `json:"resource_ids,omitempty"`
	Metadata               map[string]any  `json:"metadata,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

type Resource struct {
	ID        uuid.UUID      `json:"id"`
	OrgID     uuid.UUID      `json:"org_id"`
	BranchID  uuid.UUID      `json:"branch_id"`
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Kind      ResourceKind   `json:"kind"`
	Capacity  int            `json:"capacity"`
	Timezone  string         `json:"timezone,omitempty"`
	Active    bool           `json:"active"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type AvailabilityRule struct {
	ID                     uuid.UUID            `json:"id"`
	OrgID                  uuid.UUID            `json:"org_id"`
	BranchID               uuid.UUID            `json:"branch_id"`
	ResourceID             *uuid.UUID           `json:"resource_id,omitempty"`
	Kind                   AvailabilityRuleKind `json:"kind"`
	Weekday                int                  `json:"weekday"`
	StartTime              string               `json:"start_time"`
	EndTime                string               `json:"end_time"`
	SlotGranularityMinutes *int                 `json:"slot_granularity_minutes,omitempty"`
	ValidFrom              *time.Time           `json:"valid_from,omitempty"`
	ValidUntil             *time.Time           `json:"valid_until,omitempty"`
	Active                 bool                 `json:"active"`
	Metadata               map[string]any       `json:"metadata,omitempty"`
	CreatedAt              time.Time            `json:"created_at"`
	UpdatedAt              time.Time            `json:"updated_at"`
}

// CalendarEventStatus modela el ciclo simple de un evento interno de agenda.
// No se confunde con BookingStatus: los eventos no pasan por hold ni por
// in_service ni por no_show; sólo existen, se completan o se cancelan.
type CalendarEventStatus string

const (
	CalendarEventStatusScheduled CalendarEventStatus = "scheduled"
	CalendarEventStatusDone      CalendarEventStatus = "done"
	CalendarEventStatusCancelled CalendarEventStatus = "cancelled"
)

// CalendarEventVisibility decide quién puede ver el evento dentro de la org.
// `team` = todos los usuarios internos de la org. `private` = sólo el creador.
// La surface pública /v1/public/... NUNCA expone estos eventos.
type CalendarEventVisibility string

const (
	CalendarEventVisibilityTeam    CalendarEventVisibility = "team"
	CalendarEventVisibilityPrivate CalendarEventVisibility = "private"
)

// CalendarEvent es una entrada de agenda interna (reunión, capacitación,
// almuerzo, recordatorio del owner). Si tiene ResourceID, ocupa ese recurso y
// resta del slot picker externo (turnos clientes). Sin ResourceID, es tiempo
// personal y no afecta la disponibilidad pública.
type CalendarEvent struct {
	ID          uuid.UUID               `json:"id"`
	OrgID       uuid.UUID               `json:"org_id"`
	BranchID    *uuid.UUID              `json:"branch_id,omitempty"`
	ResourceID  *uuid.UUID              `json:"resource_id,omitempty"`
	Title       string                  `json:"title"`
	Description string                  `json:"description,omitempty"`
	StartAt     time.Time               `json:"start_at"`
	EndAt       time.Time               `json:"end_at"`
	AllDay      bool                    `json:"all_day"`
	Status      CalendarEventStatus     `json:"status"`
	Visibility  CalendarEventVisibility `json:"visibility"`
	CreatedBy   string                  `json:"created_by,omitempty"`
	Metadata    map[string]any          `json:"metadata,omitempty"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

// ListCalendarEventsFilter restringe el listado por rango temporal o recurso.
// Todos los campos son opcionales: si vienen vacíos, se devuelven los eventos
// de la org sin filtrar.
type ListCalendarEventsFilter struct {
	BranchID   *uuid.UUID
	ResourceID *uuid.UUID
	From       *time.Time
	To         *time.Time
	Status     *CalendarEventStatus
}

type BlockedRange struct {
	ID         uuid.UUID        `json:"id"`
	OrgID      uuid.UUID        `json:"org_id"`
	BranchID   uuid.UUID        `json:"branch_id"`
	ResourceID *uuid.UUID       `json:"resource_id,omitempty"`
	Kind       BlockedRangeKind `json:"kind"`
	Reason     string           `json:"reason"`
	StartAt    time.Time        `json:"start_at"`
	EndAt      time.Time        `json:"end_at"`
	AllDay     bool             `json:"all_day"`
	CreatedBy  string           `json:"created_by,omitempty"`
	Metadata   map[string]any   `json:"metadata,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}

type TimeSlot struct {
	ResourceID     uuid.UUID `json:"resource_id"`
	ResourceName   string    `json:"resource_name"`
	StartAt        time.Time `json:"start_at"`
	EndAt          time.Time `json:"end_at"`
	OccupiesFrom   time.Time `json:"occupies_from"`
	OccupiesUntil  time.Time `json:"occupies_until"`
	Timezone       string    `json:"timezone"`
	Remaining      int       `json:"remaining"`
	ConflictCount  int       `json:"conflict_count"`
	GranularityMin int       `json:"granularity_minutes"`
}

type Booking struct {
	ID             uuid.UUID      `json:"id"`
	OrgID          uuid.UUID      `json:"org_id"`
	BranchID       uuid.UUID      `json:"branch_id"`
	ServiceID      uuid.UUID      `json:"service_id"`
	ResourceID     uuid.UUID      `json:"resource_id"`
	PartyID        *uuid.UUID     `json:"party_id,omitempty"`
	Reference      string         `json:"reference"`
	CustomerName   string         `json:"customer_name"`
	CustomerPhone  string         `json:"customer_phone"`
	CustomerEmail  string         `json:"customer_email,omitempty"`
	Status         BookingStatus  `json:"status"`
	Source         BookingSource  `json:"source"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	StartAt        time.Time      `json:"start_at"`
	EndAt          time.Time      `json:"end_at"`
	OccupiesFrom   time.Time      `json:"occupies_from"`
	OccupiesUntil  time.Time      `json:"occupies_until"`
	HoldExpiresAt  *time.Time     `json:"hold_expires_at,omitempty"`
	Notes          string         `json:"notes"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedBy      string         `json:"created_by,omitempty"`
	ConfirmedAt    *time.Time     `json:"confirmed_at,omitempty"`
	CancelledAt    *time.Time     `json:"cancelled_at,omitempty"`
	ReminderSentAt *time.Time     `json:"reminder_sent_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type BookingActionToken struct {
	ID        uuid.UUID         `json:"id"`
	OrgID     uuid.UUID         `json:"org_id"`
	BookingID uuid.UUID         `json:"booking_id"`
	Action    BookingActionType `json:"action"`
	Token     string            `json:"token,omitempty"`
	TokenHash string            `json:"-"`
	ExpiresAt time.Time         `json:"expires_at"`
	UsedAt    *time.Time        `json:"used_at,omitempty"`
	VoidedAt  *time.Time        `json:"voided_at,omitempty"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

type Queue struct {
	ID               uuid.UUID      `json:"id"`
	OrgID            uuid.UUID      `json:"org_id"`
	BranchID         uuid.UUID      `json:"branch_id"`
	ServiceID        *uuid.UUID     `json:"service_id,omitempty"`
	Code             string         `json:"code"`
	Name             string         `json:"name"`
	Status           QueueStatus    `json:"status"`
	Strategy         QueueStrategy  `json:"strategy"`
	TicketPrefix     string         `json:"ticket_prefix"`
	LastIssuedNumber int64          `json:"last_issued_number"`
	AvgServiceSecond int            `json:"avg_service_seconds"`
	AllowRemoteJoin  bool           `json:"allow_remote_join"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type QueueTicket struct {
	ID                uuid.UUID         `json:"id"`
	OrgID             uuid.UUID         `json:"org_id"`
	QueueID           uuid.UUID         `json:"queue_id"`
	BranchID          uuid.UUID         `json:"branch_id"`
	ServiceID         *uuid.UUID        `json:"service_id,omitempty"`
	PartyID           *uuid.UUID        `json:"party_id,omitempty"`
	CustomerName      string            `json:"customer_name"`
	CustomerPhone     string            `json:"customer_phone"`
	CustomerEmail     string            `json:"customer_email,omitempty"`
	Number            int64             `json:"number"`
	DisplayCode       string            `json:"display_code"`
	Status            QueueTicketStatus `json:"status"`
	Priority          int               `json:"priority"`
	Source            QueueTicketSource `json:"source"`
	IdempotencyKey    string            `json:"idempotency_key,omitempty"`
	ServingResourceID *uuid.UUID        `json:"serving_resource_id,omitempty"`
	OperatorUserID    *uuid.UUID        `json:"operator_user_id,omitempty"`
	RequestedAt       time.Time         `json:"requested_at"`
	CalledAt          *time.Time        `json:"called_at,omitempty"`
	StartedAt         *time.Time        `json:"started_at,omitempty"`
	CompletedAt       *time.Time        `json:"completed_at,omitempty"`
	CancelledAt       *time.Time        `json:"cancelled_at,omitempty"`
	Notes             string            `json:"notes"`
	Metadata          map[string]any    `json:"metadata,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type WaitlistEntry struct {
	ID               uuid.UUID      `json:"id"`
	OrgID            uuid.UUID      `json:"org_id"`
	BranchID         uuid.UUID      `json:"branch_id"`
	ServiceID        uuid.UUID      `json:"service_id"`
	ResourceID       *uuid.UUID     `json:"resource_id,omitempty"`
	PartyID          *uuid.UUID     `json:"party_id,omitempty"`
	BookingID        *uuid.UUID     `json:"booking_id,omitempty"`
	CustomerName     string         `json:"customer_name"`
	CustomerPhone    string         `json:"customer_phone"`
	CustomerEmail    string         `json:"customer_email,omitempty"`
	RequestedStartAt time.Time      `json:"requested_start_at"`
	Status           WaitlistStatus `json:"status"`
	Source           WaitlistSource `json:"source"`
	IdempotencyKey   string         `json:"idempotency_key,omitempty"`
	ExpiresAt        *time.Time     `json:"expires_at,omitempty"`
	NotifiedAt       *time.Time     `json:"notified_at,omitempty"`
	Notes            string         `json:"notes,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type QueuePosition struct {
	TicketID         uuid.UUID         `json:"ticket_id"`
	QueueID          uuid.UUID         `json:"queue_id"`
	Status           QueueTicketStatus `json:"status"`
	Position         int               `json:"position"`
	EstimatedWaitSec int               `json:"estimated_wait_seconds"`
}

type DashboardStats struct {
	Date                   string `json:"date"`
	Timezone               string `json:"timezone"`
	BookingsToday          int64  `json:"bookings_today"`
	ConfirmedBookingsToday int64  `json:"confirmed_bookings_today"`
	ActiveQueues           int64  `json:"active_queues"`
	WaitingTickets         int64  `json:"waiting_tickets"`
	TicketsInService       int64  `json:"tickets_in_service"`
}

type DayAgendaItem struct {
	Type      string         `json:"type"`
	ID        uuid.UUID      `json:"id"`
	BranchID  uuid.UUID      `json:"branch_id"`
	ServiceID *uuid.UUID     `json:"service_id,omitempty"`
	StartAt   *time.Time     `json:"start_at,omitempty"`
	EndAt     *time.Time     `json:"end_at,omitempty"`
	Status    string         `json:"status"`
	Label     string         `json:"label"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type SlotQuery struct {
	BranchID   uuid.UUID
	ServiceID  uuid.UUID
	Date       time.Time
	ResourceID *uuid.UUID
}

type ListBookingsFilter struct {
	BranchID *uuid.UUID
	Date     *time.Time
	Status   string
	Limit    int
}

type BookingRecurrence struct {
	Freq      string     `json:"freq"`
	Interval  int        `json:"interval,omitempty"`
	Count     int        `json:"count,omitempty"`
	Until     *time.Time `json:"until,omitempty"`
	ByWeekday []int      `json:"by_weekday,omitempty"`
}

type CreateBookingInput struct {
	BranchID      uuid.UUID
	ServiceID     uuid.UUID
	ResourceID    *uuid.UUID
	PartyID       *uuid.UUID
	CustomerName  string
	CustomerPhone string
	CustomerEmail string
	StartAt       time.Time
	// EndAt, si viene informado (sin Recurrence), crea un booking con duración arbitraria
	// validada con la misma lógica que reschedule (reglas, bloqueos, solapes).
	EndAt          *time.Time
	Status         BookingStatus
	Source         BookingSource
	IdempotencyKey string
	Notes          string
	HoldUntil      *time.Time
	Metadata       map[string]any
	Recurrence     *BookingRecurrence
}

type RescheduleBookingInput struct {
	BookingID      uuid.UUID
	BranchID       uuid.UUID
	ResourceID     *uuid.UUID
	StartAt        time.Time
	EndAt          *time.Time
	IdempotencyKey string
}

type CreateQueueTicketInput struct {
	QueueID        uuid.UUID
	PartyID        *uuid.UUID
	CustomerName   string
	CustomerPhone  string
	CustomerEmail  string
	Priority       int
	Source         QueueTicketSource
	IdempotencyKey string
	Notes          string
	Metadata       map[string]any
}

type CreateWaitlistInput struct {
	BranchID         uuid.UUID
	ServiceID        uuid.UUID
	ResourceID       *uuid.UUID
	PartyID          *uuid.UUID
	CustomerName     string
	CustomerPhone    string
	CustomerEmail    string
	RequestedStartAt time.Time
	Source           WaitlistSource
	IdempotencyKey   string
	Notes            string
	Metadata         map[string]any
}

type ListWaitlistFilter struct {
	BranchID  *uuid.UUID
	ServiceID *uuid.UUID
	Status    string
	Limit     int
}
