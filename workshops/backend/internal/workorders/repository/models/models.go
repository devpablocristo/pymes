package models

import (
	"time"

	"github.com/google/uuid"
)

// WorkOrderModel mapea workshops.work_orders (tabla unificada con polimorfismo target_type/target_id).
type WorkOrderModel struct {
	ID       uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID    uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID *uuid.UUID `gorm:"type:uuid;index"`
	Number   string     `gorm:"not null"`

	TargetType  string    `gorm:"not null"`
	TargetID    uuid.UUID `gorm:"type:uuid;not null"`
	TargetLabel string    `gorm:"not null;default:''"`

	CustomerID   *uuid.UUID `gorm:"type:uuid"`
	CustomerName string     `gorm:"not null;default:''"`
	BookingID    *uuid.UUID `gorm:"type:uuid"`
	QuoteID      *uuid.UUID `gorm:"type:uuid"`
	SaleID       *uuid.UUID `gorm:"type:uuid"`

	Status        string `gorm:"not null"`
	RequestedWork string `gorm:"not null;default:''"`
	Diagnosis     string `gorm:"not null;default:''"`
	Notes         string `gorm:"not null;default:''"`
	InternalNotes string `gorm:"not null;default:''"`

	Currency         string  `gorm:"not null;default:'ARS'"`
	SubtotalServices float64 `gorm:"not null;default:0"`
	SubtotalParts    float64 `gorm:"not null;default:0"`
	TaxTotal         float64 `gorm:"not null;default:0"`
	Total            float64 `gorm:"not null;default:0"`

	OpenedAt    time.Time `gorm:"not null"`
	PromisedAt  *time.Time
	ReadyAt     *time.Time
	DeliveredAt *time.Time

	Metadata []byte `gorm:"type:jsonb;not null;default:'{}'"`

	CreatedBy  string     `gorm:"not null;default:''"`
	ArchivedAt *time.Time `gorm:"column:archived_at"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (WorkOrderModel) TableName() string { return "workshops.work_orders" }

// WorkOrderItemModel mapea workshops.work_order_items.
type WorkOrderItemModel struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID  `gorm:"type:uuid;index;not null"`
	WorkOrderID uuid.UUID  `gorm:"type:uuid;index;not null"`
	ItemType    string     `gorm:"not null"`
	ServiceID   *uuid.UUID `gorm:"type:uuid"`
	ProductID   *uuid.UUID `gorm:"type:uuid"`
	Description string     `gorm:"not null"`
	Quantity    float64    `gorm:"not null;default:1"`
	UnitPrice   float64    `gorm:"not null;default:0"`
	TaxRate     float64    `gorm:"not null;default:21"`
	SortOrder   int        `gorm:"not null;default:0"`
	Metadata    []byte     `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (WorkOrderItemModel) TableName() string { return "workshops.work_order_items" }
