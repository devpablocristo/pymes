package models

import (
	"time"

	"github.com/google/uuid"
)

type SalonServiceModel struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID           uuid.UUID  `gorm:"type:uuid;index;not null"`
	Code            string     `gorm:"not null"`
	Name            string     `gorm:"not null"`
	Description     string     `gorm:"not null;default:''"`
	Category        string     `gorm:"not null;default:''"`
	DurationMinutes int        `gorm:"not null;default:30"`
	BasePrice       float64    `gorm:"not null;default:0"`
	Currency        string     `gorm:"not null;default:'ARS'"`
	TaxRate         float64    `gorm:"not null;default:21"`
	LinkedServiceID *uuid.UUID `gorm:"type:uuid"`
	IsActive        bool       `gorm:"not null;default:true"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (SalonServiceModel) TableName() string { return "beauty.salon_services" }
