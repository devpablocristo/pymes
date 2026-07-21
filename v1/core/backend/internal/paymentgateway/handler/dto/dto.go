package dto

type ConnectionStatusResponse struct {
	Connected      bool    `json:"connected"`
	Provider       string  `json:"provider,omitempty"`
	ExternalUserID string  `json:"external_user_id,omitempty"`
	TokenExpiresAt *string `json:"token_expires_at,omitempty"`
	ConnectedAt    *string `json:"connected_at,omitempty"`
}

type PaymentLinkResponse struct {
	ID            string  `json:"id"`
	Provider      string  `json:"provider"`
	ReferenceType string  `json:"reference_type"`
	ReferenceID   string  `json:"reference_id"`
	Status        string  `json:"status"`
	Amount        float64 `json:"amount"`
	PaymentURL    string  `json:"payment_url"`
	QRData        string  `json:"qr_data"`
	ExpiresAt     string  `json:"expires_at"`
	CreatedAt     string  `json:"created_at"`
}

type WhatsAppResponse struct {
	URL     string `json:"url"`
	Message string `json:"message"`
}
