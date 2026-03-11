package domain

import (
	"time"

	"github.com/google/uuid"
)

type ProfessionalProfile struct {
	ID                uuid.UUID
	OrgID             uuid.UUID
	PartyID           uuid.UUID
	PublicSlug        string
	Bio               string
	Headline          string
	IsPublic          bool
	IsBookable        bool
	AcceptsNewClients bool
	Metadata          map[string]any
	Specialties       []Specialty
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Specialty struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Code        string
	Name        string
	Description string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
