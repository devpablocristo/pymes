package domain

import (
	"time"

	"github.com/google/uuid"
)

type Entry struct {
	ID           uuid.UUID      `json:"id"`
	OrgID        uuid.UUID      `json:"org_id"`
	Actor        string         `json:"actor,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
	PrevHash     string         `json:"prev_hash,omitempty"`
	Hash         string         `json:"hash"`
	CreatedAt    time.Time      `json:"created_at"`
}
