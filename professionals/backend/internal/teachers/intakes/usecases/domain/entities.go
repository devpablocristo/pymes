package domain

import (
	"time"

	"github.com/google/uuid"
)

type Intake struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	BookingID       *uuid.UUID
	ProfileID       uuid.UUID
	CustomerPartyID *uuid.UUID
	ServiceID       *uuid.UUID
	Status          string
	IsFavorite      bool
	Tags            []string
	Payload         map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

const (
	IntakeStatusDraft     = "draft"
	IntakeStatusSubmitted = "submitted"
	IntakeStatusReviewed  = "reviewed"
)
