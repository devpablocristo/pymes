package models

import (
	"time"

	"github.com/google/uuid"
)

type TableSessionModel struct {
	ID         uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	OrgID      uuid.UUID  `gorm:"column:org_id;type:uuid;not null;index"`
	TableID    uuid.UUID  `gorm:"column:table_id;type:uuid;not null;index"`
	GuestCount int        `gorm:"column:guest_count;not null;default:1"`
	PartyLabel string     `gorm:"column:party_label;not null;default:''"`
	Notes      string     `gorm:"column:notes;not null;default:''"`
	OpenedAt   time.Time  `gorm:"column:opened_at;not null"`
	ClosedAt   *time.Time `gorm:"column:closed_at"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null"`
}

func (TableSessionModel) TableName() string { return "restaurant.table_sessions" }
