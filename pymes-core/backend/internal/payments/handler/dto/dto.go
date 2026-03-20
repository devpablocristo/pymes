package dto

type CreatePaymentRequest struct {
	Method     string  `json:"method" binding:"required"`
	Amount     float64 `json:"amount" binding:"required"`
	Notes      string  `json:"notes,omitempty"`
	ReceivedAt string  `json:"received_at,omitempty"`
}
