package domain

import (
	"time"

	"github.com/google/uuid"
)

type Quote struct {
	ID           uuid.UUID   `json:"id"`
	OrgID        uuid.UUID   `json:"org_id"`
	BranchID     *uuid.UUID  `json:"branch_id,omitempty"`
	Number       string      `json:"number"`
	CustomerID   *uuid.UUID  `json:"customer_id,omitempty"`
	CustomerName string      `json:"customer_name"`
	Status       string      `json:"status"`
	Items        []QuoteItem `json:"items"`
	Subtotal     float64     `json:"subtotal"`
	TaxTotal     float64     `json:"tax_total"`
	Total        float64     `json:"total"`
	Currency     string      `json:"currency"`
	IsFavorite   bool           `json:"is_favorite"`
	Tags         []string       `json:"tags,omitempty"`
	Notes        string         `json:"notes"`
	ValidUntil   *time.Time     `json:"valid_until,omitempty"`
	CreatedBy    string         `json:"created_by"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	ArchivedAt   *time.Time     `json:"archived_at,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type QuoteItem struct {
	ID          uuid.UUID  `json:"id"`
	QuoteID     uuid.UUID  `json:"quote_id"`
	ProductID   *uuid.UUID `json:"product_id,omitempty"`
	ServiceID   *uuid.UUID `json:"service_id,omitempty"`
	Description string     `json:"description"`
	Quantity    float64    `json:"quantity"`
	UnitPrice   float64    `json:"unit_price"`
	TaxRate     float64    `json:"tax_rate"`
	Subtotal    float64    `json:"subtotal"`
	SortOrder   int        `json:"sort_order"`
}
