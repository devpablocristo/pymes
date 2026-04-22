package domain

import (
	"time"

	"github.com/google/uuid"
)

type Return struct {
	ID           uuid.UUID    `json:"id"`
	OrgID        uuid.UUID    `json:"org_id"`
	Number       string       `json:"number"`
	SaleID       uuid.UUID    `json:"sale_id"`
	PartyID      *uuid.UUID   `json:"party_id,omitempty"`
	PartyName    string       `json:"party_name"`
	Reason       string       `json:"reason"`
	Subtotal     float64      `json:"subtotal"`
	TaxTotal     float64      `json:"tax_total"`
	Total        float64      `json:"total"`
	RefundMethod string       `json:"refund_method"`
	Status       string       `json:"status"`
	Notes        string       `json:"notes"`
	IsFavorite   bool         `json:"is_favorite"`
	Tags         []string     `json:"tags"`
	ArchivedAt   *time.Time   `json:"archived_at,omitempty"`
	CreatedBy    string       `json:"created_by"`
	CreatedAt    time.Time    `json:"created_at"`
	Items        []ReturnItem `json:"items,omitempty"`
}

type ReturnItem struct {
	ID          uuid.UUID  `json:"id"`
	ReturnID    uuid.UUID  `json:"return_id"`
	SaleItemID  uuid.UUID  `json:"sale_item_id"`
	ProductID   *uuid.UUID `json:"product_id,omitempty"`
	Description string     `json:"description"`
	Quantity    float64    `json:"quantity"`
	UnitPrice   float64    `json:"unit_price"`
	TaxRate     float64    `json:"tax_rate"`
	Subtotal    float64    `json:"subtotal"`
}

type CreditNote struct {
	ID         uuid.UUID  `json:"id"`
	OrgID      uuid.UUID  `json:"org_id"`
	Number     string     `json:"number"`
	PartyID    uuid.UUID  `json:"party_id"`
	ReturnID   uuid.UUID  `json:"return_id"`
	Amount     float64    `json:"amount"`
	UsedAmount float64    `json:"used_amount"`
	Balance    float64    `json:"balance"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
}
