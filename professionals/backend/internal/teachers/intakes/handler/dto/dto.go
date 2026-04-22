package dto

type IntakeItem struct {
	ID              string         `json:"id"`
	OrgID           string         `json:"org_id"`
	BookingID       *string        `json:"booking_id,omitempty"`
	ProfileID       string         `json:"profile_id"`
	CustomerPartyID *string        `json:"customer_party_id,omitempty"`
	ServiceID       *string        `json:"service_id,omitempty"`
	Status          string         `json:"status"`
	IsFavorite      bool           `json:"is_favorite"`
	Tags            []string       `json:"tags"`
	Payload         map[string]any `json:"payload"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
}

type CreateIntakeRequest struct {
	BookingID       *string        `json:"booking_id"`
	ProfileID       string         `json:"profile_id" binding:"required"`
	CustomerPartyID *string        `json:"customer_party_id"`
	ServiceID       *string        `json:"service_id"`
	IsFavorite      *bool          `json:"is_favorite,omitempty"`
	Tags            []string       `json:"tags,omitempty"`
	Payload         map[string]any `json:"payload"`
}

type UpdateIntakeRequest struct {
	BookingID       *string         `json:"booking_id"`
	CustomerPartyID *string         `json:"customer_party_id"`
	ServiceID       *string         `json:"service_id"`
	IsFavorite      *bool           `json:"is_favorite,omitempty"`
	Tags            *[]string       `json:"tags,omitempty"`
	Payload         *map[string]any `json:"payload"`
}
