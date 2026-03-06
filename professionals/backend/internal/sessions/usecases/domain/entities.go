package domain

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	AppointmentID   uuid.UUID
	ProfileID       uuid.UUID
	CustomerPartyID *uuid.UUID
	ProductID       *uuid.UUID
	Status          string
	StartedAt       *time.Time
	EndedAt         *time.Time
	Summary         string
	Metadata        map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SessionNote struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	SessionID uuid.UUID
	NoteType  string
	Title     string
	Body      string
	CreatedBy string
	CreatedAt time.Time
}

const (
	SessionStatusScheduled = "scheduled"
	SessionStatusActive    = "active"
	SessionStatusCompleted = "completed"
	SessionStatusCancelled = "cancelled"
)
