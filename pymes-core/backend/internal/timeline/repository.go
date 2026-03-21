package timeline

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/backend/go/pagination"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/timeline/repository/models"
	timelinedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/timeline/usecases/domain"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, limit int) ([]timelinedomain.Entry, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 20, MaxLimit: 100})
	var rows []models.TimelineEntryModel
	if err := r.db.WithContext(ctx).
		Where("org_id = ? AND entity_type = ? AND entity_id = ?", orgID, entityType, entityID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]timelinedomain.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) Create(ctx context.Context, in timelinedomain.Entry) (timelinedomain.Entry, error) {
	if in.ID == uuid.Nil {
		in.ID = uuid.New()
	}
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now().UTC()
	}
	payload, _ := json.Marshal(defaultMetadata(in.Metadata))
	row := models.TimelineEntryModel{
		ID:          in.ID,
		OrgID:       in.OrgID,
		EntityType:  in.EntityType,
		EntityID:    in.EntityID,
		EventType:   in.EventType,
		Title:       in.Title,
		Description: in.Description,
		Actor:       in.Actor,
		Metadata:    payload,
		CreatedAt:   in.CreatedAt.UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return timelinedomain.Entry{}, err
	}
	return toDomain(row), nil
}

func toDomain(row models.TimelineEntryModel) timelinedomain.Entry {
	meta := map[string]any{}
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	return timelinedomain.Entry{
		ID:          row.ID,
		OrgID:       row.OrgID,
		EntityType:  row.EntityType,
		EntityID:    row.EntityID,
		EventType:   row.EventType,
		Title:       row.Title,
		Description: row.Description,
		Actor:       row.Actor,
		Metadata:    meta,
		CreatedAt:   row.CreatedAt,
	}
}

func defaultMetadata(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}
