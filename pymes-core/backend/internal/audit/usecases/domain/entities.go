package domain

import (
	"time"

	"github.com/google/uuid"
)

type Entry struct {
	ID           uuid.UUID      `json:"id"`
	OrgID        uuid.UUID      `json:"org_id"`
	Actor        string         `json:"actor,omitempty"`
	ActorType    string         `json:"actor_type,omitempty"`
	ActorID      *uuid.UUID     `json:"actor_id,omitempty"`
	ActorLabel   string         `json:"actor_label,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
	PrevHash     string         `json:"prev_hash,omitempty"`
	Hash         string         `json:"hash"`
	CreatedAt    time.Time      `json:"created_at"`
}

type ActorRef struct {
	Legacy string
	Type   string
	ID     *uuid.UUID
	Label  string
}

type LogInput struct {
	OrgID        uuid.UUID
	Actor        ActorRef
	Action       string
	ResourceType string
	ResourceID   string
	Payload      map[string]any
}
