package models

import (
	"time"

	"github.com/google/uuid"
)

type WorkOrderModel struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID            uuid.UUID  `gorm:"type:uuid;index;not null"`
	Number           string     `gorm:"not null"`
	VehicleID        uuid.UUID  `gorm:"type:uuid;not null"`
	VehiclePlate     string     `gorm:"not null;default:''"`
	CustomerID       *uuid.UUID `gorm:"type:uuid"`
	CustomerName     string     `gorm:"not null;default:''"`
	AppointmentID    *uuid.UUID `gorm:"type:uuid"`
	QuoteID          *uuid.UUID `gorm:"type:uuid"`
	SaleID           *uuid.UUID `gorm:"type:uuid"`
	Status           string     `gorm:"not null"`
	RequestedWork    string     `gorm:"not null;default:''"`
	Diagnosis        string     `gorm:"not null;default:''"`
	Notes            string     `gorm:"not null;default:''"`
	InternalNotes    string     `gorm:"not null;default:''"`
	Currency         string     `gorm:"not null;default:'ARS'"`
	SubtotalServices float64    `gorm:"not null;default:0"`
	SubtotalParts    float64    `gorm:"not null;default:0"`
	TaxTotal         float64    `gorm:"not null;default:0"`
	Total            float64    `gorm:"not null;default:0"`
	OpenedAt         time.Time  `gorm:"not null"`
	PromisedAt       *time.Time
	ReadyAt          *time.Time
	DeliveredAt      *time.Time
	CreatedBy        string `gorm:"not null;default:''"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (WorkOrderModel) TableName() string { return "workshops.work_orders" }

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
