// Package dto define los request/response del handler HTTP de calendar_export.
package dto

import "time"

type IssueTokenRequest struct {
	Name string `json:"name"`
}

type TokenResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Scopes     string     `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// IssueTokenResponse expone el plaintext del token UNA SOLA VEZ. El cliente
// debe copiarlo en la respuesta inicial; el server no lo vuelve a producir.
// El path completo del feed (`feed_url`) viene pre-armado para que el cliente
// no tenga que componerlo.
type IssueTokenResponse struct {
	Token     TokenResponse `json:"token"`
	Plaintext string        `json:"plaintext"`
	FeedURL   string        `json:"feed_url"`
}

type ListTokensResponse struct {
	Items []TokenResponse `json:"items"`
}
