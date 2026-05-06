package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type CustomerAssetModel struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID      `gorm:"type:uuid;index;not null"`
	AssetType    string         `gorm:"not null"`
	CustomerID   *uuid.UUID     `gorm:"type:uuid"`
	CustomerName string         `gorm:"not null;default:''"`
	Label        string         `gorm:"not null;default:''"`
	Brand        string         `gorm:"not null;default:''"`
	Model        string         `gorm:"not null;default:''"`
	SerialNumber string         `gorm:"not null;default:''"`
	Year         int            `gorm:"not null;default:0"`
	Color        string         `gorm:"not null;default:''"`
	Notes        string         `gorm:"not null;default:''"`
	Metadata     []byte         `gorm:"type:jsonb;not null;default:'{}'"`
	IsFavorite   bool           `gorm:"column:is_favorite;not null"`
	Tags         pq.StringArray `gorm:"type:text[]"`
	ArchivedAt   *time.Time     `gorm:"column:archived_at"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (CustomerAssetModel) TableName() string { return "workshops.customer_assets" }
