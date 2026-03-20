package domain

import (
	"time"

	"github.com/google/uuid"
)

type Entry struct {
	ID          uuid.UUID      `json:"id"`
	OrgID       uuid.UUID      `json:"org_id"`
	EntityType  string         `json:"entity_type"`
	EntityID    uuid.UUID      `json:"entity_id"`
	EventType   string         `json:"event_type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Actor       string         `json:"actor"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}
