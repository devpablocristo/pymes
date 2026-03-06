package models

import (
	"time"

	"github.com/google/uuid"
)

type RecurringExpenseModel struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID         uuid.UUID `gorm:"type:uuid;index;not null"`
	Description   string
	Amount        float64
	Currency      string
	Category      string
	PaymentMethod string
	Frequency     string
	DayOfMonth    int
	SupplierID    *uuid.UUID
	IsActive      bool
	NextDueDate   time.Time
	LastPaidDate  *time.Time
	Notes         string
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (RecurringExpenseModel) TableName() string { return "recurring_expenses" }
