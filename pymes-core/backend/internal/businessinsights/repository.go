package businessinsights

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	candidatesdomain "github.com/devpablocristo/core/notifications/go/candidates/usecases/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/businessinsights/repository/models"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Upsert(ctx context.Context, in CandidateUpsert) (CandidateRecord, bool, error) {
	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	orgID, err := uuid.Parse(in.TenantID)
	if err != nil {
		return CandidateRecord{}, false, fmt.Errorf("parse tenant_id: %w", err)
	}

	var row models.CandidateModel
	err = r.db.WithContext(ctx).
		Where("org_id = ? AND fingerprint = ?", orgID, in.Fingerprint).
		First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return CandidateRecord{}, false, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.CandidateModel{
			ID:              uuid.New(),
			OrgID:           orgID,
			Kind:            in.Kind,
			EventType:       in.EventType,
			EntityType:      in.EntityType,
			EntityID:        in.EntityID,
			Fingerprint:     in.Fingerprint,
			Severity:        in.Severity,
			Status:          candidatesdomain.StatusNew,
			Title:           in.Title,
			Body:            in.Body,
			EvidenceJSON:    marshalEvidence(in.Evidence),
			OccurrenceCount: 1,
			FirstSeenAt:     now,
			LastSeenAt:      now,
			LastActor:       in.Actor,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return CandidateRecord{}, false, err
		}
		return toCandidateRecord(row), true, nil
	}

	shouldNotify := row.Status != candidatesdomain.StatusNotified
	row.Kind = in.Kind
	row.EventType = in.EventType
	row.EntityType = in.EntityType
	row.EntityID = in.EntityID
	row.Severity = in.Severity
	row.Title = in.Title
	row.Body = in.Body
	row.EvidenceJSON = marshalEvidence(in.Evidence)
	row.LastSeenAt = now
	row.LastActor = in.Actor
	row.UpdatedAt = now
	row.OccurrenceCount++
	if row.Status == candidatesdomain.StatusResolved {
		row.Status = candidatesdomain.StatusNew
		row.ResolvedAt = nil
		shouldNotify = true
	}
	if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
		return CandidateRecord{}, false, err
	}
	return toCandidateRecord(row), shouldNotify, nil
}

func (r *Repository) ListByTenant(ctx context.Context, tenantID string, limit int) ([]CandidateRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	orgID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("parse tenant_id: %w", err)
	}
	var rows []models.CandidateModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("last_seen_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]CandidateRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCandidateRecord(row))
	}
	return out, nil
}

func (r *Repository) MarkNotified(ctx context.Context, tenantID, candidateID string, notifiedAt time.Time) error {
	if notifiedAt.IsZero() {
		notifiedAt = time.Now().UTC()
	}
	orgID, err := uuid.Parse(tenantID)
	if err != nil {
		return fmt.Errorf("parse tenant_id: %w", err)
	}
	id, err := uuid.Parse(candidateID)
	if err != nil {
		return fmt.Errorf("parse candidate_id: %w", err)
	}
	updates := map[string]any{
		"status":           candidatesdomain.StatusNotified,
		"last_notified_at": notifiedAt.UTC(),
		"updated_at":       notifiedAt.UTC(),
	}
	var row models.CandidateModel
	if err := r.db.WithContext(ctx).First(&row, "id = ? AND org_id = ?", id, orgID).Error; err != nil {
		return err
	}
	if row.FirstNotifiedAt == nil {
		updates["first_notified_at"] = notifiedAt.UTC()
	}
	return r.db.WithContext(ctx).Model(&models.CandidateModel{}).
		Where("id = ? AND org_id = ?", id, orgID).
		Updates(updates).Error
}

func marshalEvidence(in map[string]any) []byte {
	if len(in) == 0 {
		return []byte("{}")
	}
	raw, err := json.Marshal(in)
	if err != nil || len(raw) == 0 {
		return []byte("{}")
	}
	return raw
}

func toCandidateRecord(row models.CandidateModel) CandidateRecord {
	evidence := map[string]any{}
	if len(row.EvidenceJSON) > 0 {
		_ = json.Unmarshal(row.EvidenceJSON, &evidence)
	}
	return CandidateRecord{
		ID:              row.ID.String(),
		TenantID:        row.OrgID.String(),
		Kind:            row.Kind,
		EventType:       row.EventType,
		EntityType:      row.EntityType,
		EntityID:        row.EntityID,
		Fingerprint:     row.Fingerprint,
		Severity:        row.Severity,
		Status:          row.Status,
		Title:           row.Title,
		Body:            row.Body,
		Evidence:        evidence,
		OccurrenceCount: row.OccurrenceCount,
		FirstSeenAt:     row.FirstSeenAt,
		LastSeenAt:      row.LastSeenAt,
		FirstNotifiedAt: row.FirstNotifiedAt,
		LastNotifiedAt:  row.LastNotifiedAt,
		ResolvedAt:      row.ResolvedAt,
		LastActor:       row.LastActor,
	}
}
