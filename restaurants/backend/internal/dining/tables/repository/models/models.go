package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type DiningTableModel struct {
	ID         uuid.UUID      `gorm:"column:id;type:uuid;primaryKey"`
	OrgID      uuid.UUID      `gorm:"column:org_id;type:uuid;not null;index"`
	AreaID     uuid.UUID      `gorm:"column:area_id;type:uuid;not null;index"`
	Code       string         `gorm:"column:code;not null"`
	Label      string         `gorm:"column:label;not null;default:''"`
	Capacity   int            `gorm:"column:capacity;not null;default:4"`
	Status     string         `gorm:"column:status;not null;default:'available'"`
	Notes      string         `gorm:"column:notes;not null;default:''"`
	IsFavorite bool           `gorm:"column:is_favorite;not null"`
	Tags       pq.StringArray `gorm:"column:tags;type:text[]"`
	CreatedAt  time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;not null"`
}

func (DiningTableModel) TableName() string { return "restaurant.dining_tables" }
