package dto

type SessionItem struct {
	ID              string         `json:"id"`
	OrgID           string         `json:"org_id"`
	BookingID       string         `json:"booking_id"`
	ProfileID       string         `json:"profile_id"`
	CustomerPartyID *string        `json:"customer_party_id,omitempty"`
	ServiceID       *string        `json:"service_id,omitempty"`
	Status          string         `json:"status"`
	StartedAt       *string        `json:"started_at,omitempty"`
	EndedAt         *string        `json:"ended_at,omitempty"`
	Summary         string         `json:"summary"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
}

type ListSessionsResponse struct {
	Items      []SessionItem `json:"items"`
	Total      int64         `json:"total"`
	HasMore    bool          `json:"has_more"`
	NextCursor string        `json:"next_cursor,omitempty"`
}

type CreateSessionRequest struct {
	BookingID       string         `json:"booking_id" binding:"required"`
	ProfileID       string         `json:"profile_id" binding:"required"`
	CustomerPartyID *string        `json:"customer_party_id"`
	ServiceID       *string        `json:"service_id"`
	StartedAt       *string        `json:"started_at"`
	Summary         string         `json:"summary"`
	Metadata        map[string]any `json:"metadata"`
}

type SessionNoteItem struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	NoteType  string `json:"note_type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

type CreateSessionNoteRequest struct {
	NoteType string `json:"note_type"`
	Title    string `json:"title"`
	Body     string `json:"body" binding:"required"`
}
