package models

import (
	"time"

	"github.com/google/uuid"
)

type TimelineEntryModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"type:uuid;index;not null"`
	EntityType  string    `gorm:"not null;index:idx_timeline_entity,priority:2"`
	EntityID    uuid.UUID `gorm:"type:uuid;not null;index:idx_timeline_entity,priority:3"`
	EventType   string    `gorm:"not null"`
	Title       string    `gorm:"not null"`
	Description string    `gorm:"not null;default:''"`
	Actor       string    `gorm:"default:''"`
	Metadata    []byte    `gorm:"type:jsonb;not null"`
	CreatedAt   time.Time `gorm:"not null;index:idx_timeline_entity,priority:4,sort:desc"`
}

func (TimelineEntryModel) TableName() string { return "timeline_entries" }
