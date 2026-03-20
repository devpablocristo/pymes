package models

import (
	"time"

	"github.com/google/uuid"
)

type PriceListModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"type:uuid;index;not null"`
	Name        string
	Description string
	IsDefault   bool
	Markup      float64
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (PriceListModel) TableName() string { return "price_lists" }

type PriceListItemModel struct {
	PriceListID uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProductID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Price       float64
}

func (PriceListItemModel) TableName() string { return "price_list_items" }
