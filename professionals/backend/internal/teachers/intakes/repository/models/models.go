package models

import (
	"time"

	"github.com/google/uuid"
)

type IntakeModel struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID           uuid.UUID  `gorm:"type:uuid;index;not null"`
	BookingID       *uuid.UUID `gorm:"type:uuid;column:booking_id"`
	ProfileID       uuid.UUID  `gorm:"type:uuid;not null"`
	CustomerPartyID *uuid.UUID `gorm:"type:uuid"`
	ServiceID       *uuid.UUID `gorm:"type:uuid"`
	Status          string     `gorm:"not null;default:draft"`
	Payload         []byte     `gorm:"type:jsonb"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (IntakeModel) TableName() string { return "professionals.intakes" }
