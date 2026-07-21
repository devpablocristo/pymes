package dto

type CreateAccountRequest struct {
	Type        string   `json:"type" binding:"required"`
	EntityType  string   `json:"entity_type" binding:"required"`
	EntityID    string   `json:"entity_id" binding:"required"`
	EntityName  string   `json:"entity_name" binding:"required"`
	Amount      float64  `json:"amount" binding:"required"`
	Currency    string   `json:"currency,omitempty"`
	CreditLimit *float64 `json:"credit_limit,omitempty"`
	Description string   `json:"description,omitempty"`
}
