package domain

import (
	"time"

	"github.com/google/uuid"
)

type StockLevel struct {
	ProductID   uuid.UUID  `json:"product_id"`
	OrgID       uuid.UUID  `json:"org_id"`
	BranchID    *uuid.UUID `json:"branch_id,omitempty"`
	ProductName string     `json:"product_name"`
	SKU         string     `json:"sku,omitempty"`
	Quantity    float64    `json:"quantity"`
	MinQuantity float64    `json:"min_quantity"`
	TrackStock  bool       `json:"track_stock"`
	IsLowStock  bool       `json:"is_low_stock"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type StockMovement struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	BranchID    *uuid.UUID `json:"branch_id,omitempty"`
	ProductID   uuid.UUID  `json:"product_id"`
	ProductName string     `json:"product_name"`
	Type        string     `json:"type"`
	Quantity    float64    `json:"quantity"`
	Reason      string     `json:"reason"`
	ReferenceID *uuid.UUID `json:"reference_id,omitempty"`
	Notes       string     `json:"notes"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
}
