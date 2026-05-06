package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type DiningAreaModel struct {
	ID         uuid.UUID      `gorm:"column:id;type:uuid;primaryKey"`
	OrgID      uuid.UUID      `gorm:"column:org_id;type:uuid;not null;index"`
	Name       string         `gorm:"column:name;not null"`
	SortOrder  int            `gorm:"column:sort_order;not null;default:0"`
	IsFavorite bool           `gorm:"column:is_favorite;not null"`
	Tags       pq.StringArray `gorm:"column:tags;type:text[]"`
	Metadata   []byte         `gorm:"column:metadata;type:jsonb"`
	CreatedAt  time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt  *time.Time     `gorm:"column:deleted_at"`
}

func (DiningAreaModel) TableName() string { return "restaurant.dining_areas" }
