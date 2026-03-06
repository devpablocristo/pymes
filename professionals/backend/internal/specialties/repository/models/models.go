package models

import (
	"time"

	"github.com/google/uuid"
)

type SpecialtyModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"type:uuid;index;not null"`
	Code        string    `gorm:"not null"`
	Name        string    `gorm:"not null"`
	Description string
	IsActive    bool `gorm:"not null;default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (SpecialtyModel) TableName() string { return "professionals.specialties" }
