package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// InAppNotification aviso mostrado en la consola para un miembro de la org.
type InAppNotification struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	UserID      uuid.UUID
	Title       string
	Body        string
	Kind        string
	EntityType  string
	EntityID    string
	ChatContext json.RawMessage
	ReadAt      *time.Time
	CreatedAt   time.Time
}
