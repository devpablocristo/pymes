package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type SpecialtyModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"type:uuid;index;not null"`
	Code        string    `gorm:"not null"`
	Name        string    `gorm:"not null"`
	Description string
	IsActive    bool           `gorm:"not null;default:true"`
	IsFavorite  bool           `gorm:"column:is_favorite;not null"`
	Tags        pq.StringArray `gorm:"type:text[]"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (SpecialtyModel) TableName() string { return "professionals.specialties" }
