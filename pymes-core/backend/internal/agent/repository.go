package agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type confirmationModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;index;not null"`
	Actor        string    `gorm:"not null;default:''"`
	CapabilityID string    `gorm:"not null;index"`
	PayloadHash  string    `gorm:"not null"`
	HumanSummary string    `gorm:"not null;default:''"`
	RiskLevel    string    `gorm:"not null"`
	Status       string    `gorm:"not null;default:pending"`
	ExpiresAt    time.Time `gorm:"not null"`
	UsedAt       *time.Time
	CreatedAt    time.Time
}

func (confirmationModel) TableName() string { return "agent_confirmations" }

type idempotencyModel struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID          uuid.UUID `gorm:"type:uuid;index;not null"`
	Actor          string    `gorm:"not null;default:''"`
	CapabilityID   string    `gorm:"not null;index"`
	IdempotencyKey string    `gorm:"column:idempotency_key;not null;index"`
	PayloadHash    string    `gorm:"not null"`
	Response       []byte    `gorm:"type:jsonb;not null"`
	StatusCode     int       `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (idempotencyModel) TableName() string { return "agent_idempotency_records" }

type agentEventModel struct {
	ID                string          `gorm:"column:id"`
	OrgID             string          `gorm:"column:org_id"`
	ConversationID    *string         `gorm:"column:conversation_id"`
	RequestID         *string         `gorm:"column:request_id"`
	CapabilityID      *string         `gorm:"column:capability_id"`
	ConfirmationID    *string         `gorm:"column:confirmation_id"`
	ReviewRequestID   *string         `gorm:"column:review_request_id"`
	IdempotencyKey    *string         `gorm:"column:idempotency_key"`
	PayloadHash       *string         `gorm:"column:payload_hash"`
	ExternalRequestID *string         `gorm:"column:external_request_id"`
	AgentMode         string          `gorm:"column:agent_mode"`
	Channel           string          `gorm:"column:channel"`
	ActorID           string          `gorm:"column:actor_id"`
	ActorType         string          `gorm:"column:actor_type"`
	Action            string          `gorm:"column:action"`
	ToolName          string          `gorm:"column:tool_name"`
	EntityType        string          `gorm:"column:entity_type"`
	EntityID          string          `gorm:"column:entity_id"`
	Result            string          `gorm:"column:result"`
	Confirmed         bool            `gorm:"column:confirmed"`
	Metadata          json.RawMessage `gorm:"column:metadata"`
	CreatedAt         time.Time       `gorm:"column:created_at"`
}

func (agentEventModel) TableName() string { return "ai_agent_events" }

func (r *Repository) CreateConfirmation(ctx context.Context, in Confirmation) (Confirmation, error) {
	now := time.Now().UTC()
	row := confirmationModel{
		ID:           in.ID,
		OrgID:        in.OrgID,
		Actor:        in.Actor,
		CapabilityID: in.CapabilityID,
		PayloadHash:  in.PayloadHash,
		HumanSummary: in.HumanSummary,
		RiskLevel:    in.RiskLevel,
		Status:       "pending",
		ExpiresAt:    in.ExpiresAt.UTC(),
		CreatedAt:    now,
	}
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return Confirmation{}, err
	}
	return confirmationToDomain(row), nil
}

func (r *Repository) GetConfirmation(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (Confirmation, error) {
	var row confirmationModel
	if err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).First(&row).Error; err != nil {
		return Confirmation{}, err
	}
	return confirmationToDomain(row), nil
}

func (r *Repository) MarkConfirmationUsed(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&confirmationModel{}).
		Where("org_id = ? AND id = ? AND status = 'pending'", orgID, id).
		Updates(map[string]any{"status": "used", "used_at": now}).Error
}

func (r *Repository) GetIdempotencyRecord(ctx context.Context, orgID uuid.UUID, actor, capabilityID, key string) (IdempotencyRecord, bool, error) {
	var row idempotencyModel
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND actor = ? AND capability_id = ? AND idempotency_key = ?", orgID, actor, capabilityID, key).
		First(&row).Error
	if err == nil {
		return idempotencyToDomain(row), true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return IdempotencyRecord{}, false, nil
	}
	return IdempotencyRecord{}, false, err
}

func (r *Repository) SaveIdempotencyRecord(ctx context.Context, in IdempotencyRecord) error {
	now := time.Now().UTC()
	row := idempotencyModel{
		ID:             in.ID,
		OrgID:          in.OrgID,
		Actor:          in.Actor,
		CapabilityID:   in.CapabilityID,
		IdempotencyKey: in.IdempotencyKey,
		PayloadHash:    in.PayloadHash,
		Response:       in.Response,
		StatusCode:     in.StatusCode,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) ListAgentEvents(ctx context.Context, orgID uuid.UUID, limit int, capabilityID, requestID string) ([]AgentEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := r.db.WithContext(ctx).Table("ai_agent_events").Where("org_id = ?", orgID)
	if capabilityID != "" {
		q = q.Where("capability_id = ?", capabilityID)
	}
	if requestID != "" {
		q = q.Where("request_id = ? OR external_request_id = ?", requestID, requestID)
	}
	var rows []agentEventModel
	if err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AgentEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, eventToDomain(row))
	}
	return out, nil
}

func confirmationToDomain(row confirmationModel) Confirmation {
	return Confirmation{
		ID:           row.ID,
		OrgID:        row.OrgID,
		Actor:        row.Actor,
		CapabilityID: row.CapabilityID,
		PayloadHash:  row.PayloadHash,
		HumanSummary: row.HumanSummary,
		RiskLevel:    row.RiskLevel,
		Status:       row.Status,
		ExpiresAt:    row.ExpiresAt,
		UsedAt:       row.UsedAt,
		CreatedAt:    row.CreatedAt,
	}
}

func idempotencyToDomain(row idempotencyModel) IdempotencyRecord {
	return IdempotencyRecord{
		ID:             row.ID,
		OrgID:          row.OrgID,
		Actor:          row.Actor,
		CapabilityID:   row.CapabilityID,
		IdempotencyKey: row.IdempotencyKey,
		PayloadHash:    row.PayloadHash,
		Response:       append(json.RawMessage(nil), row.Response...),
		StatusCode:     row.StatusCode,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func eventToDomain(row agentEventModel) AgentEvent {
	var metadata map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &metadata)
	}
	return AgentEvent{
		ID:              row.ID,
		OrgID:           row.OrgID,
		ConversationID:  row.ConversationID,
		RequestID:       row.RequestID,
		CapabilityID:    row.CapabilityID,
		ConfirmationID:  row.ConfirmationID,
		ReviewRequestID: row.ReviewRequestID,
		IdempotencyKey:  row.IdempotencyKey,
		PayloadHash:     row.PayloadHash,
		ExternalRequest: row.ExternalRequestID,
		AgentMode:       row.AgentMode,
		Channel:         row.Channel,
		ActorID:         row.ActorID,
		ActorType:       row.ActorType,
		Action:          row.Action,
		ToolName:        row.ToolName,
		EntityType:      row.EntityType,
		EntityID:        row.EntityID,
		Result:          row.Result,
		Confirmed:       row.Confirmed,
		Metadata:        metadata,
		CreatedAt:       row.CreatedAt,
	}
}
