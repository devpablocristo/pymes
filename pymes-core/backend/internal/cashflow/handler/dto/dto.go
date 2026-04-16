package dto

type CashMovementItem struct {
	ID            string  `json:"id"`
	OrgID         string  `json:"org_id"`
	BranchID      string  `json:"branch_id,omitempty"`
	Type          string  `json:"type"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Category      string  `json:"category"`
	Description   string  `json:"description"`
	PaymentMethod string  `json:"payment_method"`
	ReferenceType string  `json:"reference_type"`
	ReferenceID   string  `json:"reference_id,omitempty"`
	CreatedBy     string  `json:"created_by"`
	CreatedAt     string  `json:"created_at"`
}

type ListCashMovementsResponse struct {
	Items      []CashMovementItem `json:"items"`
	Total      int64              `json:"total"`
	HasMore    bool               `json:"has_more"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

type CreateCashMovementRequest struct {
	Type          string  `json:"type" binding:"required"`
	Amount        float64 `json:"amount" binding:"required"`
	BranchID      *string `json:"branch_id"`
	Category      string  `json:"category"`
	Description   string  `json:"description"`
	PaymentMethod string  `json:"payment_method"`
	ReferenceType string  `json:"reference_type"`
	ReferenceID   *string `json:"reference_id"`
	Currency      *string `json:"currency"`
}

type CashSummaryResponse struct {
	OrgID        string  `json:"org_id"`
	PeriodStart  string  `json:"period_start"`
	PeriodEnd    string  `json:"period_end"`
	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
	Balance      float64 `json:"balance"`
	Currency     string  `json:"currency"`
}

type DailySummaryItem struct {
	Date    string  `json:"date"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Balance float64 `json:"balance"`
}
