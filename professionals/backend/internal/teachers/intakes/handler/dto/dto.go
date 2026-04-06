package dto

type IntakeItem struct {
	ID              string         `json:"id"`
	OrgID           string         `json:"org_id"`
	AppointmentID   *string        `json:"appointment_id,omitempty"`
	ProfileID       string         `json:"profile_id"`
	CustomerPartyID *string        `json:"customer_party_id,omitempty"`
	ServiceID       *string        `json:"service_id,omitempty"`
	Status          string         `json:"status"`
	Payload         map[string]any `json:"payload"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
}

type CreateIntakeRequest struct {
	AppointmentID   *string        `json:"appointment_id"`
	ProfileID       string         `json:"profile_id" binding:"required"`
	CustomerPartyID *string        `json:"customer_party_id"`
	ServiceID       *string        `json:"service_id"`
	Payload         map[string]any `json:"payload"`
}

type UpdateIntakeRequest struct {
	AppointmentID   *string         `json:"appointment_id"`
	CustomerPartyID *string         `json:"customer_party_id"`
	ServiceID       *string         `json:"service_id"`
	Payload         *map[string]any `json:"payload"`
}
