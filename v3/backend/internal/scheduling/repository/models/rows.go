package models

import (
	"time"

	"github.com/google/uuid"
)

type BookingRow struct {
	OrganizationID  string
	ID              uuid.UUID
	SeriesID        *uuid.UUID
	SessionID       *uuid.UUID
	SupersedesID    *uuid.UUID
	Occurrence      int
	BranchID        uuid.UUID
	ServiceID       uuid.UUID
	PartyID         string
	Status          string
	Participants    int
	StartAt         time.Time
	EndAt           time.Time
	OccupiesFrom    time.Time
	OccupiesUntil   time.Time
	HoldExpiresAt   *time.Time
	Version         int
	ServiceName     string
	Price           string
	Currency        string
	DurationMinutes int
	Timezone        string
	CustomerName    string
	CustomerEmail   string
	CustomerPhone   string
	Notes           string
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AllocationRow struct {
	ResourceID uuid.UUID
	Mode       string
	Units      int
}

type IdempotencyRow struct {
	PayloadHash string
	Response    []byte
}

type BookingIDsResponse struct {
	BookingIDs []uuid.UUID `json:"booking_ids"`
}

type BookingIDResponse struct {
	BookingID uuid.UUID `json:"booking_id"`
}

type SessionIDResponse struct {
	SessionID uuid.UUID `json:"session_id"`
}

type WaitlistIDResponse struct {
	WaitlistID uuid.UUID `json:"waitlist_id"`
}

type PublicWaitlistActionResponse struct {
	WaitlistID uuid.UUID `json:"waitlist_id"`
	BookingID  uuid.UUID `json:"booking_id"`
}

type TicketIDResponse struct {
	TicketID uuid.UUID `json:"ticket_id"`
}

type ReminderDueRow struct {
	ID            uuid.UUID
	At            time.Time
	EndsAt        time.Time
	ServiceName   string
	Timezone      string
	CustomerName  string
	CustomerPhone string
}

// BookingResponse is the stable adapter-owned snapshot persisted for exact
// idempotency replay. It must not drift with Go field names in domain.Booking.
type BookingResponse struct {
	OrganizationID     string               `json:"organization_id"`
	ID                 uuid.UUID            `json:"id"`
	SeriesID           *uuid.UUID           `json:"series_id,omitempty"`
	SessionID          *uuid.UUID           `json:"session_id,omitempty"`
	SupersedesID       *uuid.UUID           `json:"supersedes_id,omitempty"`
	Occurrence         int                  `json:"occurrence"`
	BranchID           uuid.UUID            `json:"branch_id"`
	ServiceID          uuid.UUID            `json:"service_id"`
	PartyID            string               `json:"party_id"`
	Status             string               `json:"status"`
	SubstateCode       string               `json:"substate_code,omitempty"`
	Participants       int                  `json:"participants"`
	StartAt            time.Time            `json:"start_at"`
	EndAt              time.Time            `json:"end_at"`
	OccupiesFrom       time.Time            `json:"occupies_from"`
	OccupiesUntil      time.Time            `json:"occupies_until"`
	HoldExpiresAt      *time.Time           `json:"hold_expires_at,omitempty"`
	Version            int                  `json:"version"`
	ServiceName        string               `json:"service_name"`
	Price              string               `json:"price"`
	Currency           string               `json:"currency"`
	DurationMinutes    int                  `json:"duration_minutes"`
	Timezone           string               `json:"timezone"`
	CustomerName       string               `json:"customer_name"`
	CustomerEmail      string               `json:"customer_email,omitempty"`
	CustomerPhone      string               `json:"customer_phone,omitempty"`
	Notes              string               `json:"notes,omitempty"`
	CancellationReason string               `json:"cancellation_reason,omitempty"`
	Allocations        []AllocationResponse `json:"allocations"`
	CreatedBy          string               `json:"created_by"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
}

type AllocationResponse struct {
	ResourceID uuid.UUID `json:"resource_id"`
	Mode       string    `json:"mode"`
	Units      int       `json:"units"`
}
