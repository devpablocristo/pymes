package audit

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/backend/go/hashutil"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/audit/repository/models"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/audit/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Add(in domain.LogInput) domain.Entry {
	var lastEntry models.AuditLogModel
	prevHash := ""
	if err := r.db.Where("org_id = ?", in.OrgID).Order("created_at DESC").First(&lastEntry).Error; err == nil {
		prevHash = lastEntry.Hash
	}

	canonicalPayload, _ := json.Marshal(in.Payload)
	hash := hashutil.SHA256Hex(prevHash + string(canonicalPayload))

	payloadJSON, _ := json.Marshal(in.Payload)
	actorType := normalizeActorType(in.Actor.Type)
	actorLabel := strings.TrimSpace(in.Actor.Label)
	if actorLabel == "" {
		actorLabel = strings.TrimSpace(in.Actor.Legacy)
	}

	m := models.AuditLogModel{
		ID:           uuid.New(),
		OrgID:        in.OrgID,
		Actor:        strings.TrimSpace(in.Actor.Legacy),
		ActorType:    actorType,
		ActorID:      in.Actor.ID,
		ActorLabel:   actorLabel,
		Action:       strings.TrimSpace(in.Action),
		ResourceType: strings.TrimSpace(in.ResourceType),
		ResourceID:   strings.TrimSpace(in.ResourceID),
		Payload:      payloadJSON,
		PrevHash:     prevHash,
		Hash:         hash,
		CreatedAt:    time.Now().UTC(),
	}
	r.db.Create(&m)

	return auditToDomain(m)
}

func (r *Repository) List(orgID uuid.UUID, limit int) []domain.Entry {
	if limit <= 0 {
		limit = 200
	}
	var rows []models.AuditLogModel
	r.db.Where("org_id = ?", orgID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows)

	result := make([]domain.Entry, 0, len(rows))
	for _, row := range rows {
		result = append(result, auditToDomain(row))
	}
	return result
}

func (r *Repository) ExportCSV(orgID uuid.UUID) (string, error) {
	entries := r.List(orgID, 0)

	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"id", "org_id", "actor", "actor_type", "actor_id", "actor_label", "action", "resource_type", "resource_id", "prev_hash", "hash", "created_at", "payload"}); err != nil {
		return "", err
	}
	for _, e := range entries {
		payload, _ := json.Marshal(e.Payload)
		actorID := ""
		if e.ActorID != nil {
			actorID = e.ActorID.String()
		}
		if err := w.Write([]string{
			e.ID.String(), e.OrgID.String(), e.Actor, e.ActorType, actorID, e.ActorLabel, e.Action,
			e.ResourceType, e.ResourceID, e.PrevHash, e.Hash,
			e.CreatedAt.Format(time.RFC3339), string(payload),
		}); err != nil {
			return "", err
		}
	}
	w.Flush()
	return b.String(), w.Error()
}

func auditToDomain(m models.AuditLogModel) domain.Entry {
	var payload map[string]any
	if len(m.Payload) > 0 {
		_ = json.Unmarshal(m.Payload, &payload)
	}
	return domain.Entry{
		ID:           m.ID,
		OrgID:        m.OrgID,
		Actor:        m.Actor,
		ActorType:    m.ActorType,
		ActorID:      m.ActorID,
		ActorLabel:   m.ActorLabel,
		Action:       m.Action,
		ResourceType: m.ResourceType,
		ResourceID:   m.ResourceID,
		Payload:      payload,
		PrevHash:     m.PrevHash,
		Hash:         m.Hash,
		CreatedAt:    m.CreatedAt,
	}
}

func normalizeActorType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "party", "service", "system":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "user"
	}
}
