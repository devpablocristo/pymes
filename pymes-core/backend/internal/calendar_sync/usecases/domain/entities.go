// Package domain contiene los tipos del dominio de calendar_sync.
//
// La sincronización con Google Calendar / Outlook se modela como una
// "conexión" del usuario interno con su cuenta externa: refresh_token
// encriptado, calendario seleccionado, sync_token para deltas. El módulo no
// modela el contenido del calendario — eso vive en modules/scheduling.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Provider identifica al servicio externo. Por ahora sólo Google; Outlook
// (microsoft) entra en Etapa 6.
type Provider string

const (
	ProviderGoogle    Provider = "google"
	ProviderMicrosoft Provider = "microsoft"
)

// Connection representa una conexión activa o histórica con un calendario
// externo. Los campos `*Encrypted` están encriptados en DB; el dominio sólo
// los expone como string opaco para que el usecase decida cuándo desencriptar.
type Connection struct {
	ID                    uuid.UUID
	OrgID                 uuid.UUID
	CreatedBy             string
	Provider              Provider
	ProviderAccountEmail  string
	ProviderCalendarID    string
	ProviderCalendarName  string
	Scopes                string
	RefreshTokenEncrypted string
	AccessTokenEncrypted  string
	AccessTokenExpiresAt  *time.Time
	SyncToken             string
	LastSyncAt            *time.Time
	LastSyncError         string
	RevokedAt             *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// OAuthState es el registro temporal de un flow OAuth en curso. Se borra al
// recibir el callback (consumido) o al expirar (TTL ~15 min).
type OAuthState struct {
	State     string
	OrgID     uuid.UUID
	CreatedBy string
	Provider  Provider
	ExpiresAt time.Time
	CreatedAt time.Time
}
