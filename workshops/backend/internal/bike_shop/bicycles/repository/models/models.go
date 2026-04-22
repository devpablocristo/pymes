package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type BicycleModel struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID           uuid.UUID  `gorm:"type:uuid;not null"`
	CustomerID      *uuid.UUID `gorm:"type:uuid"`
	CustomerName    string     `gorm:"not null;default:''"`
	FrameNumber     string     `gorm:"not null;default:''"`
	Brand           string     `gorm:"not null;default:''"`
	Model           string     `gorm:"not null;default:''"`
	BikeType        string     `gorm:"not null;default:''"`
	Size            string     `gorm:"not null;default:''"`
	WheelSizeInches int        `gorm:"not null;default:0"`
	Color           string     `gorm:"not null;default:''"`
	EbikeNotes      string     `gorm:"not null;default:''"`
	Notes           string     `gorm:"not null;default:''"`
	IsFavorite      bool           `gorm:"column:is_favorite;not null"`
	Tags            pq.StringArray `gorm:"type:text[]"`
	ArchivedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (BicycleModel) TableName() string { return "workshops.bicycles" }
