package models

import (
	"time"

	"github.com/google/uuid"
)

type ProfessionalProfileModel struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID             uuid.UUID `gorm:"type:uuid;index;not null"`
	PartyID           uuid.UUID `gorm:"type:uuid;not null"`
	PublicSlug        string    `gorm:"not null"`
	Bio               string
	Headline          string
	IsPublic          bool   `gorm:"not null;default:false"`
	IsBookable        bool   `gorm:"not null;default:false"`
	AcceptsNewClients bool   `gorm:"not null;default:true"`
	Metadata          []byte `gorm:"type:jsonb"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (ProfessionalProfileModel) TableName() string {
	return "professionals.professional_profiles"
}

type ProfessionalSpecialtyModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;not null"`
	ProfileID    uuid.UUID `gorm:"type:uuid;not null"`
	SpecialtyID  uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt    time.Time
}

func (ProfessionalSpecialtyModel) TableName() string {
	return "professionals.professional_specialties"
}
