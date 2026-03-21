package models

import (
	"time"

	"github.com/google/uuid"
)

type ProcurementRequest struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID          uuid.UUID  `gorm:"type:uuid;index;not null"`
	RequesterActor string     `gorm:"not null"`
	Title          string     `gorm:"not null"`
	Description    string     `gorm:"not null;default:''"`
	Category       string     `gorm:"not null;default:''"`
	Status         string     `gorm:"not null;default:'draft'"`
	EstimatedTotal float64    `gorm:"type:numeric(18,4);not null;default:0"`
	Currency       string     `gorm:"not null;default:'ARS'"`
	EvaluationJSON []byte     `gorm:"type:jsonb"`
	PurchaseID     *uuid.UUID `gorm:"type:uuid"`
	CreatedAt      time.Time  `gorm:"not null"`
	UpdatedAt      time.Time  `gorm:"not null"`
	ArchivedAt     *time.Time
}

func (ProcurementRequest) TableName() string { return "procurement_requests" }

type ProcurementRequestLine struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	RequestID         uuid.UUID `gorm:"type:uuid;index;not null"`
	Description       string    `gorm:"not null;default:''"`
	ProductID         *uuid.UUID `gorm:"type:uuid"`
	Quantity          float64   `gorm:"type:numeric(18,4);not null;default:1"`
	UnitPriceEstimate float64   `gorm:"type:numeric(18,4);not null;default:0"`
	SortOrder         int       `gorm:"not null;default:0"`
}

func (ProcurementRequestLine) TableName() string { return "procurement_request_lines" }

type ProcurementPolicy struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;index;not null"`
	Name         string    `gorm:"not null"`
	Expression   string    `gorm:"not null"`
	Effect       string    `gorm:"not null"`
	Priority     int       `gorm:"not null;default:100"`
	Mode         string    `gorm:"not null;default:'enforce'"`
	Enabled      bool      `gorm:"not null;default:true"`
	ActionFilter string    `gorm:"not null;default:'procurement.submit'"`
	SystemFilter string    `gorm:"not null;default:'pymes'"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (ProcurementPolicy) TableName() string { return "procurement_policies" }
