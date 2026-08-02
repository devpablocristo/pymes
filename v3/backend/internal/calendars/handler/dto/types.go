package dto

import "time"

type StartGoogleOAuthRequest struct {
	TimeZone        string `json:"time_zone"`
	FreeBusyEnabled bool   `json:"free_busy_enabled"`
	MeetEnabled     bool   `json:"meet_enabled"`
}

type OAuthStartResponse struct {
	ConnectionID     string    `json:"connection_id"`
	AuthorizationURL string    `json:"authorization_url"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type ConnectionResponse struct {
	ID                string    `json:"id"`
	Provider          string    `json:"provider"`
	Status            string    `json:"status"`
	CalendarConnected bool      `json:"calendar_connected"`
	TimeZone          string    `json:"time_zone"`
	FreeBusyEnabled   bool      `json:"free_busy_enabled"`
	MeetEnabled       bool      `json:"meet_enabled"`
	AccessTokenExpiry time.Time `json:"access_token_expires_at,omitempty"`
	Version           int       `json:"version"`
}

type ErrorResponse struct {
	Code string `json:"code"`
}
