package models

import (
	"time"

	"github.com/google/uuid"
)

type BranchModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID     uuid.UUID `gorm:"type:uuid;index;not null"`
	Code      string    `gorm:"not null"`
	Name      string    `gorm:"not null"`
	Timezone  string    `gorm:"not null"`
	Address   string
	Active    bool
	Metadata  []byte `gorm:"type:jsonb"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (BranchModel) TableName() string { return "scheduling_branches" }

type ServiceModel struct {
	ID                     uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID                  uuid.UUID `gorm:"type:uuid;index;not null"`
	CommercialServiceID    *uuid.UUID `gorm:"column:commercial_service_id;type:uuid"`
	Code                   string    `gorm:"not null"`
	Name                   string    `gorm:"not null"`
	Description            string
	FulfillmentMode        string `gorm:"not null"`
	DefaultDurationMinutes int
	BufferBeforeMinutes    int
	BufferAfterMinutes     int
	SlotGranularityMinutes int
	MaxConcurrentBookings  int
	MinCancelNoticeMinutes int
	AllowWaitlist          bool
	Active                 bool
	Metadata               []byte `gorm:"type:jsonb"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func (ServiceModel) TableName() string { return "scheduling_services" }

type ResourceModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID     uuid.UUID `gorm:"type:uuid;index;not null"`
	BranchID  uuid.UUID `gorm:"type:uuid;index;not null"`
	Code      string    `gorm:"not null"`
	Name      string    `gorm:"not null"`
	Kind      string    `gorm:"not null"`
	Capacity  int
	Timezone  string
	Active    bool
	Metadata  []byte `gorm:"type:jsonb"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ResourceModel) TableName() string { return "scheduling_resources" }

type ServiceResourceModel struct {
	ServiceID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	ResourceID uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt  time.Time
}

func (ServiceResourceModel) TableName() string { return "scheduling_service_resources" }

type AvailabilityRuleModel struct {
	ID                     uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID                  uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID               uuid.UUID  `gorm:"type:uuid;index;not null"`
	ResourceID             *uuid.UUID `gorm:"type:uuid;index"`
	Kind                   string     `gorm:"not null"`
	Weekday                int
	// Postgres column is `time without time zone`. The lib/pq driver returns it
	// as a string ("HH:MM:SS"), so we keep the model field as string and let
	// the mappers convert to/from time.Time using parseClock / formatClock.
	StartTime              string `gorm:"column:start_time;type:time"`
	EndTime                string `gorm:"column:end_time;type:time"`
	SlotGranularityMinutes *int
	ValidFrom              *time.Time
	ValidUntil             *time.Time
	Active                 bool
	Metadata               []byte `gorm:"type:jsonb"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func (AvailabilityRuleModel) TableName() string { return "scheduling_availability_rules" }

type BlockedRangeModel struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID      uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID   uuid.UUID  `gorm:"type:uuid;index;not null"`
	ResourceID *uuid.UUID `gorm:"type:uuid;index"`
	Kind       string     `gorm:"not null"`
	Reason     string
	StartAt    time.Time
	EndAt      time.Time
	AllDay     bool
	CreatedBy  string
	Metadata   []byte `gorm:"type:jsonb"`
	CreatedAt  time.Time
}

func (BlockedRangeModel) TableName() string { return "scheduling_blocked_ranges" }

// CalendarEventModel persiste eventos internos de agenda. No se mezcla con
// BookingModel: tiene su propia tabla, su propio ciclo y nunca se expone en la
// surface pública.
type CalendarEventModel struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID    *uuid.UUID `gorm:"type:uuid;index"`
	ResourceID  *uuid.UUID `gorm:"type:uuid;index"`
	Title       string     `gorm:"not null"`
	Description string
	StartAt     time.Time
	EndAt       time.Time
	AllDay      bool
	Status      string `gorm:"not null"`
	Visibility  string `gorm:"not null"`
	CreatedBy   string
	Metadata    []byte `gorm:"type:jsonb"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (CalendarEventModel) TableName() string { return "scheduling_calendar_events" }

type BookingModel struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID          uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID       uuid.UUID  `gorm:"type:uuid;index;not null"`
	ServiceID      uuid.UUID  `gorm:"type:uuid;index;not null"`
	ResourceID     uuid.UUID  `gorm:"type:uuid;index;not null"`
	PartyID        *uuid.UUID `gorm:"column:party_id;type:uuid;index"`
	Reference      string     `gorm:"not null"`
	CustomerName   string     `gorm:"column:customer_name;not null"`
	CustomerPhone  string     `gorm:"column:customer_phone"`
	CustomerEmail  string     `gorm:"column:customer_email"`
	Status         string     `gorm:"not null"`
	Source         string     `gorm:"not null"`
	// Puntero para que el cero (`""`) se persista como NULL y no choque con el
	// índice único parcial `WHERE idempotency_key IS NOT NULL`.
	IdempotencyKey *string    `gorm:"column:idempotency_key"`
	StartAt        time.Time
	EndAt          time.Time
	OccupiesFrom   time.Time
	OccupiesUntil  time.Time
	HoldExpiresAt  *time.Time
	Notes          string
	Metadata       []byte `gorm:"type:jsonb"`
	CreatedBy      string
	ConfirmedAt    *time.Time
	CancelledAt    *time.Time
	ReminderSentAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (BookingModel) TableName() string { return "scheduling_bookings" }

type QueueModel struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID            uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID         uuid.UUID  `gorm:"type:uuid;index;not null"`
	ServiceID        *uuid.UUID `gorm:"type:uuid;index"`
	Code             string     `gorm:"not null"`
	Name             string     `gorm:"not null"`
	Status           string     `gorm:"not null"`
	Strategy         string     `gorm:"not null"`
	TicketPrefix     string     `gorm:"not null"`
	LastIssuedNumber int64
	AvgServiceSecond int `gorm:"column:avg_service_seconds"`
	AllowRemoteJoin  bool
	Metadata         []byte `gorm:"type:jsonb"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (QueueModel) TableName() string { return "scheduling_queues" }

type QueueTicketModel struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID             uuid.UUID  `gorm:"type:uuid;index;not null"`
	QueueID           uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID          uuid.UUID  `gorm:"type:uuid;index;not null"`
	ServiceID         *uuid.UUID `gorm:"type:uuid;index"`
	PartyID           *uuid.UUID `gorm:"column:party_id;type:uuid;index"`
	CustomerName      string     `gorm:"column:customer_name;not null"`
	CustomerPhone     string     `gorm:"column:customer_phone"`
	CustomerEmail     string     `gorm:"column:customer_email"`
	Number            int64      `gorm:"not null"`
	DisplayCode       string     `gorm:"not null"`
	Status            string     `gorm:"not null"`
	Priority          int
	Source            string     `gorm:"not null"`
	IdempotencyKey    string     `gorm:"column:idempotency_key"`
	ServingResourceID *uuid.UUID `gorm:"type:uuid;index"`
	OperatorUserID    *uuid.UUID `gorm:"type:uuid;index"`
	RequestedAt       time.Time
	CalledAt          *time.Time
	StartedAt         *time.Time
	CompletedAt       *time.Time
	CancelledAt       *time.Time
	Notes             string
	Metadata          []byte `gorm:"type:jsonb"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (QueueTicketModel) TableName() string { return "scheduling_queue_tickets" }

type BookingActionTokenModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID     uuid.UUID `gorm:"type:uuid;index;not null"`
	BookingID uuid.UUID `gorm:"type:uuid;index;not null"`
	Action    string    `gorm:"not null"`
	TokenHash string    `gorm:"column:token_hash;not null"`
	ExpiresAt time.Time
	UsedAt    *time.Time
	VoidedAt  *time.Time
	Metadata  []byte `gorm:"type:jsonb"`
	CreatedAt time.Time
}

func (BookingActionTokenModel) TableName() string { return "scheduling_booking_action_tokens" }

type WaitlistEntryModel struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID            uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID         uuid.UUID  `gorm:"type:uuid;index;not null"`
	ServiceID        uuid.UUID  `gorm:"type:uuid;index;not null"`
	ResourceID       *uuid.UUID `gorm:"type:uuid;index"`
	PartyID          *uuid.UUID `gorm:"column:party_id;type:uuid;index"`
	BookingID        *uuid.UUID `gorm:"column:booking_id;type:uuid;index"`
	CustomerName     string     `gorm:"column:customer_name;not null"`
	CustomerPhone    string     `gorm:"column:customer_phone"`
	CustomerEmail    string     `gorm:"column:customer_email"`
	RequestedStartAt time.Time
	Status           string `gorm:"not null"`
	Source           string `gorm:"not null"`
	IdempotencyKey   string `gorm:"column:idempotency_key"`
	ExpiresAt        *time.Time
	NotifiedAt       *time.Time
	Notes            string
	Metadata         []byte `gorm:"type:jsonb"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (WaitlistEntryModel) TableName() string { return "scheduling_waitlist_entries" }
