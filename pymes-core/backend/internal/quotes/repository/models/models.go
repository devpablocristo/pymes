// Package models defines GORM models for quotes persistence.
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type QuoteModel struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID     *uuid.UUID `gorm:"type:uuid;index"`
	Number       string     `gorm:"not null"`
	CustomerID   *uuid.UUID `gorm:"column:party_id;type:uuid"`
	CustomerName string     `gorm:"column:party_name"`
	Status       string     `gorm:"not null"`
	Subtotal     float64    `gorm:"type:numeric(15,2)"`
	TaxTotal     float64    `gorm:"type:numeric(15,2)"`
	Total        float64    `gorm:"type:numeric(15,2)"`
	Currency     string
	IsFavorite   bool           `gorm:"column:is_favorite;not null"`
	Tags         pq.StringArray `gorm:"type:text[]"`
	Notes        string
	ValidUntil   *time.Time
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ArchivedAt   *time.Time
	Tags         pq.StringArray `gorm:"type:text[];not null;default:'{}'"`
	Metadata     []byte         `gorm:"type:jsonb;not null;default:'{}'"`
}

func (QuoteModel) TableName() string { return "quotes" }

type QuoteItemModel struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	QuoteID     uuid.UUID  `gorm:"type:uuid;index;not null"`
	ProductID   *uuid.UUID `gorm:"type:uuid"`
	ServiceID   *uuid.UUID `gorm:"type:uuid"`
	Description string
	Quantity    float64 `gorm:"type:numeric(15,2)"`
	UnitPrice   float64 `gorm:"type:numeric(15,2)"`
	TaxRate     float64 `gorm:"type:numeric(5,2)"`
	Subtotal    float64 `gorm:"type:numeric(15,2)"`
	SortOrder   int
}

func (QuoteItemModel) TableName() string { return "quote_items" }
