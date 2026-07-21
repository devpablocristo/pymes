package dto

// CreateCreditNoteRequest alta manual de nota de crédito (sin devolución).
type CreateCreditNoteRequest struct {
	PartyID string  `json:"party_id" binding:"required"`
	Amount  float64 `json:"amount" binding:"required"`
}
