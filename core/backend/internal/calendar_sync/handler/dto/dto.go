// Package dto define los request/response del handler HTTP de calendar_sync.
package dto

import "time"

type StartConnectResponse struct {
	// AuthURL es la URL de consent de Google a la que el frontend tiene que
	// redirigir al browser del usuario.
	AuthURL string `json:"auth_url"`
}

type ConnectionResponse struct {
	ID                   string     `json:"id"`
	Provider             string     `json:"provider"`
	ProviderAccountEmail string     `json:"provider_account_email,omitempty"`
	ProviderCalendarID   string     `json:"provider_calendar_id,omitempty"`
	ProviderCalendarName string     `json:"provider_calendar_name,omitempty"`
	Scopes               string     `json:"scopes,omitempty"`
	LastSyncAt           *time.Time `json:"last_sync_at,omitempty"`
	LastSyncError        string     `json:"last_sync_error,omitempty"`
	RevokedAt            *time.Time `json:"revoked_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

type ListConnectionsResponse struct {
	Items []ConnectionResponse `json:"items"`
}
