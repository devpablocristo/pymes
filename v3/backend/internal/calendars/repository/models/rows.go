package models

import "time"

type ConnectionRow struct {
	ID                string
	OrganizationID    string
	ActorID           string
	Provider          string
	Status            string
	CalendarID        string
	TimeZone          string
	Scopes            []string
	FreeBusyEnabled   bool
	MeetEnabled       bool
	TokenEnvelope     []byte
	AccessTokenExpiry time.Time
	Version           int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type OAuthStateRow struct {
	Hash            string
	OrganizationID  string
	ActorID         string
	ConnectionID    string
	SessionBinding  string
	TimeZone        string
	FreeBusyEnabled bool
	MeetEnabled     bool
	ExpiresAt       time.Time
	ConsumedAt      *time.Time
	CreatedAt       time.Time
}

type OAuthGrantPayload struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Scope        string    `json:"scope"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type ExternalEventRow struct {
	OrganizationID string
	ConnectionID   string
	BookingID      string
	GoogleEventID  string
	ETag           string
	MeetRequestID  string
	MeetStatus     string
	MeetURI        string
	SourceVersion  int
	SnapshotDigest string
	Status         string
	LastErrorCode  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
