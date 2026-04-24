package agent

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type RiskLevel string

const (
	RiskRead     RiskLevel = "read"
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type Channel string

const (
	ChannelHumanUI       Channel = "human_ui"
	ChannelInternalAgent Channel = "internal_agent"
	ChannelExternalAgent Channel = "external_agent"
	ChannelMCP           Channel = "mcp"
)

type Capability struct {
	ID                     string         `json:"id"`
	Resource               string         `json:"resource"`
	Action                 string         `json:"action"`
	Description            string         `json:"description"`
	InputSchema            map[string]any `json:"input_schema"`
	OutputSchema           map[string]any `json:"output_schema"`
	RiskLevel              RiskLevel      `json:"risk_level"`
	RequiresConfirmation   bool           `json:"requires_confirmation"`
	RequiresReview         bool           `json:"requires_review"`
	RequiresIdempotencyKey bool           `json:"requires_idempotency_key"`
	AllowedChannels        []Channel      `json:"allowed_channels"`
	RBACResource           string         `json:"rbac_resource"`
	RBACAction             string         `json:"rbac_action"`
	AuditAction            string         `json:"audit_action"`
	OwnerModule            string         `json:"owner_module"`
	NexusActionType        string         `json:"nexus_action_type"`
	ExecutorStatus         string         `json:"executor_status"`
}

type Confirmation struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	Actor        string
	CapabilityID string
	PayloadHash  string
	HumanSummary string
	RiskLevel    string
	Status       string
	ExpiresAt    time.Time
	UsedAt       *time.Time
	CreatedAt    time.Time
}

type IdempotencyRecord struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	Actor          string
	CapabilityID   string
	IdempotencyKey string
	PayloadHash    string
	Response       json.RawMessage
	StatusCode     int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AgentEvent struct {
	ID              string         `json:"id"`
	OrgID           string         `json:"org_id"`
	ConversationID  *string        `json:"conversation_id,omitempty"`
	RequestID       *string        `json:"request_id,omitempty"`
	CapabilityID    *string        `json:"capability_id,omitempty"`
	ConfirmationID  *string        `json:"confirmation_id,omitempty"`
	ReviewRequestID *string        `json:"review_request_id,omitempty"`
	IdempotencyKey  *string        `json:"idempotency_key,omitempty"`
	PayloadHash     *string        `json:"payload_hash,omitempty"`
	ExternalRequest *string        `json:"external_request_id,omitempty"`
	AgentMode       string         `json:"agent_mode"`
	Channel         string         `json:"channel"`
	ActorID         string         `json:"actor_id"`
	ActorType       string         `json:"actor_type"`
	Action          string         `json:"action"`
	ToolName        string         `json:"tool_name"`
	EntityType      string         `json:"entity_type"`
	EntityID        string         `json:"entity_id"`
	Result          string         `json:"result"`
	Confirmed       bool           `json:"confirmed"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

type ActorContext struct {
	OrgID      string
	Actor      string
	Role       string
	Scopes     []string
	AuthMethod string
}
