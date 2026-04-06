package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type ProductModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"type:uuid;index;not null"`
	Type        string    `gorm:"not null"`
	SKU         string
	Name        string `gorm:"not null"`
	Description string
	Unit        string
	Price       float64  `gorm:"type:numeric(15,2)"`
	Currency    string   `gorm:"column:price_currency;not null"`
	CostPrice   float64  `gorm:"type:numeric(15,2)"`
	TaxRate     *float64 `gorm:"type:numeric(5,2)"`
	TrackStock  bool
	IsActive    bool           `gorm:"column:is_active;not null"`
	Tags        pq.StringArray `gorm:"type:text[]"`
	Metadata    []byte         `gorm:"type:jsonb"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func (ProductModel) TableName() string { return "products" }
