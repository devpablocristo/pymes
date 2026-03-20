package models

import (
	"time"

	"github.com/google/uuid"
)

type AccountModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"type:uuid;index;not null"`
	Type        string
	EntityType  string
	EntityID    uuid.UUID
	EntityName  string
	Balance     float64
	Currency    string
	CreditLimit float64
	UpdatedAt   time.Time
}

func (AccountModel) TableName() string { return "accounts" }

type MovementModel struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	AccountID     uuid.UUID `gorm:"type:uuid;index;not null"`
	OrgID         uuid.UUID `gorm:"type:uuid;index;not null"`
	Type          string
	Amount        float64
	Balance       float64
	Description   string
	ReferenceType string
	ReferenceID   *uuid.UUID
	CreatedBy     string
	CreatedAt     time.Time
}

func (MovementModel) TableName() string { return "account_movements" }
