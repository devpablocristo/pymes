package models

import (
	"time"

	"github.com/google/uuid"
)

type VehicleModel struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID  `gorm:"type:uuid;index;not null"`
	CustomerID   *uuid.UUID `gorm:"type:uuid"`
	CustomerName string     `gorm:"not null;default:''"`
	LicensePlate string     `gorm:"not null"`
	VIN          string     `gorm:"not null;default:''"`
	Make         string     `gorm:"not null"`
	Model        string     `gorm:"not null"`
	Year         int        `gorm:"not null;default:0"`
	Kilometers   int        `gorm:"not null;default:0"`
	Color        string     `gorm:"not null;default:''"`
	Notes        string     `gorm:"not null;default:''"`
	ArchivedAt   *time.Time `gorm:"column:archived_at"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (VehicleModel) TableName() string { return "workshops.vehicles" }
