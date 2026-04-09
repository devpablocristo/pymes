// Package domain contiene los tipos del dominio de calendar_export.
//
// Este módulo NO modela calendarios — eso vive en modules/scheduling. Lo único
// que modela es la noción de "token público que un usuario interno emite para
// suscribir su agenda desde una app externa". El feed que el endpoint público
// devuelve se compone consultando scheduling y serializando con la librería
// core/calendar/ics/go.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Token es un token de export persistido. El campo plaintext NUNCA se guarda;
// sólo el hash. La única vez que el plaintext aparece es en la respuesta de
// IssueResult, después de la creación.
type Token struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	CreatedBy   string
	Name        string
	TokenHash   string
	Scopes      string
	LastUsedAt  *time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}

// IssueResult es lo único que devuelve el caso de uso de creación: la entidad
// persistida más el plaintext del token, este último visible una sola vez.
// El handler debe asegurarse de exponer Plaintext sólo en la respuesta inicial
// y nunca volver a leerlo.
type IssueResult struct {
	Token     Token
	Plaintext string
}
