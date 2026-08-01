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
