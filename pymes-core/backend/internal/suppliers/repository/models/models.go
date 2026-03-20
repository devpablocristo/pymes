package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type SupplierModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID       uuid.UUID `gorm:"type:uuid;index;not null"`
	Name        string    `gorm:"not null"`
	TaxID       string
	Email       string
	Phone       string
	Address     []byte `gorm:"type:jsonb"`
	ContactName string
	Notes       string
	Tags        pq.StringArray `gorm:"type:text[]"`
	Metadata    []byte         `gorm:"type:jsonb"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func (SupplierModel) TableName() string { return "suppliers" }
