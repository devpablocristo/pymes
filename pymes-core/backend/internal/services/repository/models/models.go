package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type ServiceModel struct {
	ID                     uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID                  uuid.UUID `gorm:"type:uuid;index;not null"`
	Code                   string
	Name                   string `gorm:"not null"`
	Description            string
	CategoryCode           string
	SalePrice              float64        `gorm:"column:sale_price;type:numeric(15,2)"`
	CostPrice              float64        `gorm:"type:numeric(15,2)"`
	TaxRate                *float64       `gorm:"type:numeric(5,2)"`
	Currency               string         `gorm:"not null"`
	DefaultDurationMinutes *int           `gorm:"column:default_duration_minutes"`
	IsActive               bool           `gorm:"column:is_active;not null"`
	IsFavorite             bool           `gorm:"column:is_favorite;not null"`
	Tags                   pq.StringArray `gorm:"type:text[]"`
	Metadata               []byte         `gorm:"type:jsonb"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
	DeletedAt              *time.Time
}

func (ServiceModel) TableName() string { return "services" }
