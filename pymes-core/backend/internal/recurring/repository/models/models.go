package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
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
	SupplierID    *uuid.UUID `gorm:"column:party_id;type:uuid"`
	IsActive      bool
	IsFavorite    bool           `gorm:"column:is_favorite;not null"`
	Tags          pq.StringArray `gorm:"type:text[]"`
	NextDueDate   time.Time
	LastPaidDate  *time.Time
	Notes         string
	DeletedAt     *time.Time `gorm:"column:deleted_at;index"`
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (RecurringExpenseModel) TableName() string { return "recurring_expenses" }
