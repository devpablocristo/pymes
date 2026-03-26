package models

import (
	"time"

	"github.com/google/uuid"
)

type ServiceModel struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID           uuid.UUID  `gorm:"type:uuid;index;not null"`
	Segment         string     `gorm:"not null;default:'auto_repair'"`
	Code            string     `gorm:"not null"`
	Name            string     `gorm:"not null"`
	Description     string     `gorm:"not null;default:''"`
	Category        string     `gorm:"not null;default:''"`
	EstimatedHours  float64    `gorm:"not null;default:0"`
	BasePrice       float64    `gorm:"not null;default:0"`
	Currency        string     `gorm:"not null;default:'ARS'"`
	TaxRate         float64    `gorm:"not null;default:21"`
	LinkedProductID *uuid.UUID `gorm:"type:uuid"`
	IsActive        bool       `gorm:"not null;default:true"`
	ArchivedAt      *time.Time `gorm:"column:archived_at"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (ServiceModel) TableName() string { return "workshops.services" }
