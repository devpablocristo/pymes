package domain

import (
	"time"

	"github.com/google/uuid"
)

type Sale struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	BranchID      *uuid.UUID `json:"branch_id,omitempty"`
	Number        string     `json:"number"`
	CustomerID    *uuid.UUID `json:"customer_id,omitempty"`
	CustomerName  string     `json:"customer_name"`
	QuoteID       *uuid.UUID `json:"quote_id,omitempty"`
	Status        string     `json:"status"`
	PaymentMethod string     `json:"payment_method"`
	Items         []SaleItem `json:"items"`
	Subtotal      float64    `json:"subtotal"`
	TaxTotal      float64    `json:"tax_total"`
	Total         float64    `json:"total"`
	Currency      string     `json:"currency"`
	Notes         string     `json:"notes"`
	CreatedBy     string     `json:"created_by"`
	CreatedAt     time.Time      `json:"created_at"`
	VoidedAt      *time.Time     `json:"voided_at,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type SaleItem struct {
	ID          uuid.UUID  `json:"id"`
	SaleID      uuid.UUID  `json:"sale_id"`
	ProductID   *uuid.UUID `json:"product_id,omitempty"`
	ServiceID   *uuid.UUID `json:"service_id,omitempty"`
	Description string     `json:"description"`
	Quantity    float64    `json:"quantity"`
	UnitPrice   float64    `json:"unit_price"`
	CostPrice   float64    `json:"cost_price"`
	TaxRate     float64    `json:"tax_rate"`
	Subtotal    float64    `json:"subtotal"`
	SortOrder   int        `json:"sort_order"`
}
