package models

import (
	"time"

	"github.com/google/uuid"
)

type PurchaseModel struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID         uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID      *uuid.UUID `gorm:"type:uuid;index"`
	Number        string
	SupplierID    *uuid.UUID `gorm:"column:party_id;type:uuid"`
	SupplierName  string     `gorm:"column:party_name"`
	Status        string
	PaymentStatus string
	Subtotal      float64
	TaxTotal      float64
	Total         float64
	Currency      string
	Notes         string
	ReceivedAt    *time.Time
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (PurchaseModel) TableName() string { return "purchases" }

type PurchaseItemModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	PurchaseID  uuid.UUID `gorm:"type:uuid;index;not null"`
	ProductID   *uuid.UUID
	ServiceID   *uuid.UUID
	Description string
	Quantity    float64
	UnitCost    float64
	TaxRate     float64
	Subtotal    float64
	SortOrder   int
}

func (PurchaseItemModel) TableName() string { return "purchase_items" }
