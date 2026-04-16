package models

import (
	"time"

	"github.com/google/uuid"
)

type StockLevelModel struct {
	ProductID   uuid.UUID  `gorm:"type:uuid;index;not null"`
	OrgID       uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID    *uuid.UUID `gorm:"type:uuid;index"`
	Quantity    float64    `gorm:"type:numeric(15,2);not null"`
	MinQuantity float64    `gorm:"type:numeric(15,2);not null"`
	UpdatedAt   time.Time
}

func (StockLevelModel) TableName() string { return "stock_levels" }

type StockMovementModel struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID    *uuid.UUID `gorm:"type:uuid;index"`
	ProductID   uuid.UUID  `gorm:"type:uuid;index;not null"`
	Type        string     `gorm:"not null"`
	Quantity    float64    `gorm:"type:numeric(15,2);not null"`
	Reason      string
	ReferenceID *uuid.UUID `gorm:"type:uuid"`
	Notes       string
	CreatedBy   string
	CreatedAt   time.Time
}

func (StockMovementModel) TableName() string { return "stock_movements" }
