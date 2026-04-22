package domain

import (
	"time"

	"github.com/google/uuid"
)

type RecurringExpense struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	Description   string     `json:"description"`
	Amount        float64    `json:"amount"`
	Currency      string     `json:"currency"`
	Category      string     `json:"category"`
	PaymentMethod string     `json:"payment_method"`
	Frequency     string     `json:"frequency"`
	DayOfMonth    int        `json:"day_of_month"`
	SupplierID    *uuid.UUID `json:"supplier_id,omitempty"`
	IsActive      bool       `json:"is_active"`
	IsFavorite    bool       `json:"is_favorite"`
	Tags          []string   `json:"tags"`
	NextDueDate   time.Time  `json:"next_due_date"`
	LastPaidDate  *time.Time `json:"last_paid_date,omitempty"`
	Notes         string     `json:"notes"`
	CreatedBy     string     `json:"created_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
