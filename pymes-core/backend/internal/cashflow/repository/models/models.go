package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type CashMovementModel struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID         uuid.UUID  `gorm:"type:uuid;index;not null"`
	BranchID      *uuid.UUID `gorm:"type:uuid;index"`
	Type          string     `gorm:"not null"`
	Amount        float64    `gorm:"type:numeric(15,2);not null"`
	Currency      string     `gorm:"not null"`
	Category      string
	Description   string
	PaymentMethod string
	ReferenceType string
	ReferenceID   *uuid.UUID     `gorm:"type:uuid"`
	IsFavorite    bool           `gorm:"column:is_favorite;not null"`
	Tags          pq.StringArray `gorm:"type:text[]"`
	DeletedAt     *time.Time     `gorm:"column:deleted_at;index"`
	CreatedBy     string
	CreatedAt     time.Time
}

func (CashMovementModel) TableName() string { return "cash_movements" }
