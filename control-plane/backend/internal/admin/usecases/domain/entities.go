package domain

import (
	"time"

	"github.com/google/uuid"
)

type TenantSettings struct {
	OrgID      uuid.UUID      `json:"org_id"`
	PlanCode   string         `json:"plan_code"`
	HardLimits map[string]any `json:"hard_limits"`
	UpdatedBy  *string        `json:"updated_by,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type ActivityEvent struct {
	ID           uuid.UUID      `json:"id"`
	OrgID        uuid.UUID      `json:"org_id"`
	Actor        string         `json:"actor,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}
