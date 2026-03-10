package vehicles

import (
	"time"

	"github.com/google/uuid"
)

type Vehicle struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	CustomerID   *uuid.UUID
	CustomerName string
	LicensePlate string
	VIN          string
	Make         string
	Model        string
	Year         int
	Kilometers   int
	Color        string
	Notes        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

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
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (VehicleModel) TableName() string { return "workshops.vehicles" }

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Search string
}

type UpdateInput struct {
	CustomerID   *string
	CustomerName *string
	LicensePlate *string
	VIN          *string
	Make         *string
	Model        *string
	Year         *int
	Kilometers   *int
	Color        *string
	Notes        *string
}
