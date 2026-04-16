package domain

import (
	"time"

	"github.com/google/uuid"
)

type CashMovement struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	BranchID      *uuid.UUID `json:"branch_id,omitempty"`
	Type          string     `json:"type"`
	Amount        float64    `json:"amount"`
	Currency      string     `json:"currency"`
	Category      string     `json:"category"`
	Description   string     `json:"description"`
	PaymentMethod string     `json:"payment_method"`
	ReferenceType string     `json:"reference_type"`
	ReferenceID   *uuid.UUID `json:"reference_id,omitempty"`
	CreatedBy     string     `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
}

type CashSummary struct {
	OrgID        uuid.UUID `json:"org_id"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`
	TotalIncome  float64   `json:"total_income"`
	TotalExpense float64   `json:"total_expense"`
	Balance      float64   `json:"balance"`
	Currency     string    `json:"currency"`
}
