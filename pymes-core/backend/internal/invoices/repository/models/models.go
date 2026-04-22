package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type InvoiceModel struct {
	ID              uuid.UUID      `gorm:"column:id;primaryKey"`
	OrgID           uuid.UUID      `gorm:"column:org_id"`
	Number          string         `gorm:"column:number"`
	PartyID         *uuid.UUID     `gorm:"column:party_id"`
	CustomerName    string         `gorm:"column:customer_name"`
	IssuedDate      time.Time      `gorm:"column:issued_date"`
	DueDate         time.Time      `gorm:"column:due_date"`
	Status          string         `gorm:"column:status"`
	Subtotal        float64        `gorm:"column:subtotal"`
	DiscountPercent float64        `gorm:"column:discount_percent"`
	TaxPercent      float64        `gorm:"column:tax_percent"`
	Total           float64        `gorm:"column:total"`
	Notes           string         `gorm:"column:notes"`
	IsFavorite      bool           `gorm:"column:is_favorite"`
	Tags            pq.StringArray `gorm:"column:tags;type:text[]"`
	CreatedBy       string         `gorm:"column:created_by"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at"`
	DeletedAt       *time.Time     `gorm:"column:deleted_at"`
}

func (InvoiceModel) TableName() string { return "invoices" }

type InvoiceLineItemModel struct {
	ID          uuid.UUID `gorm:"column:id;primaryKey"`
	InvoiceID   uuid.UUID `gorm:"column:invoice_id"`
	Description string    `gorm:"column:description"`
	Qty         float64   `gorm:"column:qty"`
	Unit        string    `gorm:"column:unit"`
	UnitPrice   float64   `gorm:"column:unit_price"`
	LineTotal   float64   `gorm:"column:line_total"`
	SortOrder   int       `gorm:"column:sort_order"`
}

func (InvoiceLineItemModel) TableName() string { return "invoice_line_items" }
