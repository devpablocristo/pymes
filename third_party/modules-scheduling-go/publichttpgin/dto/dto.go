package dto

type CancelBookingActionRequest struct {
	Reason string `json:"reason"`
}

type CreateQueueTicketRequest struct {
	PartyID        *string        `json:"party_id,omitempty"`
	CustomerName   string         `json:"customer_name"`
	CustomerPhone  string         `json:"customer_phone"`
	CustomerEmail  string         `json:"customer_email,omitempty"`
	Priority       int            `json:"priority"`
	Source         string         `json:"source,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Notes          string         `json:"notes,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type CreateWaitlistRequest struct {
	BranchID         string         `json:"branch_id"`
	ServiceID        string         `json:"service_id"`
	ResourceID       *string        `json:"resource_id,omitempty"`
	PartyID          *string        `json:"party_id,omitempty"`
	CustomerName     string         `json:"customer_name"`
	CustomerPhone    string         `json:"customer_phone"`
	CustomerEmail    string         `json:"customer_email,omitempty"`
	RequestedStartAt string         `json:"requested_start_at"`
	Source           string         `json:"source,omitempty"`
	IdempotencyKey   string         `json:"idempotency_key,omitempty"`
	Notes            string         `json:"notes,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}
