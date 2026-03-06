package dto

type CreateRecurringExpenseRequest struct {
	Description   string  `json:"description" binding:"required"`
	Amount        float64 `json:"amount" binding:"required"`
	Currency      string  `json:"currency,omitempty"`
	Category      string  `json:"category,omitempty"`
	PaymentMethod string  `json:"payment_method,omitempty"`
	Frequency     string  `json:"frequency,omitempty"`
	DayOfMonth    int     `json:"day_of_month,omitempty"`
	SupplierID    *string `json:"supplier_id,omitempty"`
	NextDueDate   string  `json:"next_due_date,omitempty"`
	Notes         string  `json:"notes,omitempty"`
}

type UpdateRecurringExpenseRequest struct {
	Description   *string  `json:"description,omitempty"`
	Amount        *float64 `json:"amount,omitempty"`
	Currency      *string  `json:"currency,omitempty"`
	Category      *string  `json:"category,omitempty"`
	PaymentMethod *string  `json:"payment_method,omitempty"`
	Frequency     *string  `json:"frequency,omitempty"`
	DayOfMonth    *int     `json:"day_of_month,omitempty"`
	SupplierID    *string  `json:"supplier_id,omitempty"`
	IsActive      *bool    `json:"is_active,omitempty"`
	NextDueDate   *string  `json:"next_due_date,omitempty"`
	Notes         *string  `json:"notes,omitempty"`
}
