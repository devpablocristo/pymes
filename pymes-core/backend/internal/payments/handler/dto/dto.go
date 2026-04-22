package dto

type CreatePaymentRequest struct {
	Method     string   `json:"method" binding:"required"`
	Amount     float64  `json:"amount" binding:"required"`
	Notes      string   `json:"notes,omitempty"`
	ReceivedAt string   `json:"received_at,omitempty"`
	IsFavorite *bool    `json:"is_favorite,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

type UpdatePaymentRequest struct {
	Notes      *string   `json:"notes,omitempty"`
	IsFavorite *bool     `json:"is_favorite,omitempty"`
	Tags       *[]string `json:"tags,omitempty"`
}

type PaymentItem struct {
	ID            string   `json:"id"`
	OrgID         string   `json:"org_id"`
	ReferenceType string   `json:"reference_type"`
	ReferenceID   string   `json:"reference_id"`
	Method        string   `json:"method"`
	Amount        float64  `json:"amount"`
	Notes         string   `json:"notes"`
	ReceivedAt    string   `json:"received_at"`
	IsFavorite    bool     `json:"is_favorite"`
	Tags          []string `json:"tags"`
	ArchivedAt    string   `json:"archived_at,omitempty"`
	CreatedBy     string   `json:"created_by,omitempty"`
	CreatedAt     string   `json:"created_at"`
}
