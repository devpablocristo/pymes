package domain

import (
	"time"

	"github.com/google/uuid"
)

type Preference struct {
	UserID           uuid.UUID `json:"user_id"`
	NotificationType string    `json:"notification_type"`
	Channel          string    `json:"channel"`
	Enabled          bool      `json:"enabled"`
}

type Log struct {
	ID                uuid.UUID `json:"id"`
	OrgID             uuid.UUID `json:"org_id"`
	UserID            uuid.UUID `json:"user_id"`
	NotificationType  string    `json:"notification_type"`
	Channel           string    `json:"channel"`
	Status            string    `json:"status"`
	ProviderMessageID string    `json:"provider_message_id,omitempty"`
	DedupKey          string    `json:"dedup_key"`
	CreatedAt         time.Time `json:"created_at"`
}
