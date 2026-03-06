package models

import (
	"time"

	"github.com/google/uuid"
)

type ReturnModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;index;not null"`
	Number       string    `gorm:"not null"`
	SaleID       uuid.UUID `gorm:"type:uuid;not null"`
	Reason       string    `gorm:"not null"`
	Subtotal     float64   `gorm:"type:numeric(15,2);not null"`
	TaxTotal     float64   `gorm:"type:numeric(15,2);not null"`
	Total        float64   `gorm:"type:numeric(15,2);not null"`
	RefundMethod string    `gorm:"not null"`
	Status       string    `gorm:"not null"`
	Notes        string    `gorm:"not null;default:''"`
	CreatedBy    string    `gorm:"default:''"`
	CreatedAt    time.Time `gorm:"not null"`
}

func (ReturnModel) TableName() string { return "returns" }

type ReturnItemModel struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ReturnID    uuid.UUID  `gorm:"type:uuid;index;not null"`
	SaleItemID  uuid.UUID  `gorm:"type:uuid;not null"`
	ProductID   *uuid.UUID `gorm:"type:uuid"`
	Description string     `gorm:"not null"`
	Quantity    float64    `gorm:"type:numeric(15,2);not null"`
	UnitPrice   float64    `gorm:"type:numeric(15,2);not null"`
	TaxRate     float64    `gorm:"type:numeric(5,2);not null"`
	Subtotal    float64    `gorm:"type:numeric(15,2);not null"`
}

func (ReturnItemModel) TableName() string { return "return_items" }

type CreditNoteModel struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID      uuid.UUID `gorm:"type:uuid;index;not null"`
	Number     string    `gorm:"not null"`
	PartyID    uuid.UUID `gorm:"column:party_id;type:uuid;not null"`
	ReturnID   uuid.UUID `gorm:"type:uuid;not null"`
	Amount     float64   `gorm:"type:numeric(15,2);not null"`
	UsedAmount float64   `gorm:"type:numeric(15,2);not null"`
	Balance    float64   `gorm:"type:numeric(15,2);not null"`
	ExpiresAt  *time.Time
	Status     string    `gorm:"not null"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (CreditNoteModel) TableName() string { return "credit_notes" }
