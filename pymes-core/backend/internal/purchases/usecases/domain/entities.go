package domain

import (
	"time"

	"github.com/google/uuid"
)

type Purchase struct {
	ID            uuid.UUID      `json:"id"`
	OrgID         uuid.UUID      `json:"org_id"`
	BranchID      *uuid.UUID     `json:"branch_id,omitempty"`
	Number        string         `json:"number"`
	SupplierID    *uuid.UUID     `json:"supplier_id,omitempty"`
	SupplierName  string         `json:"supplier_name"`
	Status        string         `json:"status"`
	PaymentStatus string         `json:"payment_status"`
	Subtotal      float64        `json:"subtotal"`
	TaxTotal      float64        `json:"tax_total"`
	Total         float64        `json:"total"`
	Currency      string         `json:"currency"`
	Notes         string         `json:"notes"`
	ReceivedAt    *time.Time     `json:"received_at,omitempty"`
	CreatedBy     string         `json:"created_by,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	Tags          []string       `json:"tags,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Items         []PurchaseItem `json:"items,omitempty"`
}

type PurchaseItem struct {
	ID          uuid.UUID  `json:"id"`
	PurchaseID  uuid.UUID  `json:"purchase_id"`
	ProductID   *uuid.UUID `json:"product_id,omitempty"`
	ServiceID   *uuid.UUID `json:"service_id,omitempty"`
	Description string     `json:"description"`
	Quantity    float64    `json:"quantity"`
	UnitCost    float64    `json:"unit_cost"`
	TaxRate     float64    `json:"tax_rate"`
	Subtotal    float64    `json:"subtotal"`
	SortOrder   int        `json:"sort_order"`
}
