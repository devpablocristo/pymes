package workshopservices

import (
	"time"

	"github.com/google/uuid"
)

type Service struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	Code            string
	Name            string
	Description     string
	Category        string
	EstimatedHours  float64
	BasePrice       float64
	Currency        string
	TaxRate         float64
	LinkedProductID *uuid.UUID
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ServiceModel struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID           uuid.UUID  `gorm:"type:uuid;index;not null"`
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
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (ServiceModel) TableName() string { return "workshops.services" }

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Search string
}

type UpdateInput struct {
	Code            *string
	Name            *string
	Description     *string
	Category        *string
	EstimatedHours  *float64
	BasePrice       *float64
	Currency        *string
	TaxRate         *float64
	LinkedProductID *string
	IsActive        *bool
}
