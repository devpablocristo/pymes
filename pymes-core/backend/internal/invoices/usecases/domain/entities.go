package domain

import (
	"time"

	"github.com/google/uuid"
)

type InvoiceStatus string

const (
	InvoiceStatusPaid    InvoiceStatus = "paid"
	InvoiceStatusPending InvoiceStatus = "pending"
	InvoiceStatusOverdue InvoiceStatus = "overdue"
)

type Invoice struct {
	ID              uuid.UUID         `json:"id"`
	OrgID           uuid.UUID         `json:"org_id"`
	Number          string            `json:"number"`
	PartyID         *uuid.UUID        `json:"party_id,omitempty"`
	CustomerName    string            `json:"customer_name"`
	IssuedDate      time.Time         `json:"issued_date"`
	DueDate         time.Time         `json:"due_date"`
	Status          InvoiceStatus     `json:"status"`
	Subtotal        float64           `json:"subtotal"`
	DiscountPercent float64           `json:"discount_percent"`
	TaxPercent      float64           `json:"tax_percent"`
	Total           float64           `json:"total"`
	Notes           string            `json:"notes"`
	IsFavorite      bool              `json:"is_favorite"`
	Tags            []string          `json:"tags"`
	CreatedBy       string            `json:"created_by"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	ArchivedAt      *time.Time        `json:"archived_at,omitempty"`
	Items           []InvoiceLineItem `json:"items"`
}

type InvoiceLineItem struct {
	ID          uuid.UUID `json:"id"`
	InvoiceID   uuid.UUID `json:"invoice_id"`
	Description string    `json:"description"`
	Qty         float64   `json:"qty"`
	Unit        string    `json:"unit"`
	UnitPrice   float64   `json:"unit_price"`
	LineTotal   float64   `json:"line_total"`
	SortOrder   int       `json:"sort_order"`
}
