package models

import (
	"time"

	"github.com/google/uuid"
)

type QuoteModel struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID  `gorm:"type:uuid;index;not null"`
	Number       string     `gorm:"not null"`
	CustomerID   *uuid.UUID `gorm:"type:uuid"`
	CustomerName string
	Status       string  `gorm:"not null"`
	Subtotal     float64 `gorm:"type:numeric(15,2)"`
	TaxTotal     float64 `gorm:"type:numeric(15,2)"`
	Total        float64 `gorm:"type:numeric(15,2)"`
	Currency     string
	Notes        string
	ValidUntil   *time.Time
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (QuoteModel) TableName() string { return "quotes" }

type QuoteItemModel struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	QuoteID     uuid.UUID  `gorm:"type:uuid;index;not null"`
	ProductID   *uuid.UUID `gorm:"type:uuid"`
	Description string
	Quantity    float64 `gorm:"type:numeric(15,2)"`
	UnitPrice   float64 `gorm:"type:numeric(15,2)"`
	TaxRate     float64 `gorm:"type:numeric(5,2)"`
	Subtotal    float64 `gorm:"type:numeric(15,2)"`
	SortOrder   int
}

func (QuoteItemModel) TableName() string { return "quote_items" }
