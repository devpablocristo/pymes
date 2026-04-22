// Package models defines GORM models for sales persistence.
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type SaleModel struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID         uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID      *uuid.UUID `gorm:"type:uuid;index"`
	Number        string     `gorm:"not null"`
	CustomerID    *uuid.UUID `gorm:"column:party_id;type:uuid"`
	CustomerName  string     `gorm:"column:party_name"`
	QuoteID       *uuid.UUID `gorm:"type:uuid"`
	Status        string     `gorm:"not null"`
	PaymentMethod string     `gorm:"not null"`
	Subtotal      float64    `gorm:"type:numeric(15,2)"`
	TaxTotal      float64    `gorm:"type:numeric(15,2)"`
	Total         float64    `gorm:"type:numeric(15,2)"`
	Currency      string
	IsFavorite    bool           `gorm:"column:is_favorite;not null"`
	Tags          pq.StringArray `gorm:"type:text[]"`
	Notes         string
	CreatedBy     string
	CreatedAt     time.Time
	VoidedAt      *time.Time
}

func (SaleModel) TableName() string { return "sales" }

type SaleItemModel struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	SaleID      uuid.UUID  `gorm:"type:uuid;index;not null"`
	ProductID   *uuid.UUID `gorm:"type:uuid"`
	ServiceID   *uuid.UUID `gorm:"type:uuid"`
	Description string
	Quantity    float64 `gorm:"type:numeric(15,2)"`
	UnitPrice   float64 `gorm:"type:numeric(15,2)"`
	CostPrice   float64 `gorm:"type:numeric(15,2)"`
	TaxRate     float64 `gorm:"type:numeric(5,2)"`
	Subtotal    float64 `gorm:"type:numeric(15,2)"`
	SortOrder   int
}

func (SaleItemModel) TableName() string { return "sale_items" }
