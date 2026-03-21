package models

import (
	"time"

	"github.com/google/uuid"
)

type BicycleModel struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID           uuid.UUID  `gorm:"type:uuid;index;not null"`
	CustomerID      *uuid.UUID `gorm:"type:uuid"`
	CustomerName    string     `gorm:"not null;default:''"`
	FrameNumber     string     `gorm:"not null"`
	Make            string     `gorm:"not null"`
	Model           string     `gorm:"not null"`
	BikeType        string     `gorm:"not null;default:''"`
	Size            string     `gorm:"not null;default:''"`
	WheelSizeInches int        `gorm:"not null;default:0"`
	Color           string     `gorm:"not null;default:''"`
	EbikeNotes      string     `gorm:"not null;default:''"`
	Notes           string     `gorm:"not null;default:''"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (BicycleModel) TableName() string { return "workshops.bicycles" }
