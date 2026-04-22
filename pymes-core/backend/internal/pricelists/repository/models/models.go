package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type PriceListModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"type:uuid;index;not null"`
	Name        string
	Description string
	IsDefault   bool
	Markup      float64
	IsActive    bool
	IsFavorite  bool           `gorm:"column:is_favorite;not null"`
	Tags        pq.StringArray `gorm:"type:text[]"`
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

type ServicePriceListItemModel struct {
	PriceListID uuid.UUID `gorm:"type:uuid;primaryKey"`
	ServiceID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Price       float64
}

func (ServicePriceListItemModel) TableName() string { return "service_price_list_items" }
