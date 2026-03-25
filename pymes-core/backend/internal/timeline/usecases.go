package timeline

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/errors/go/domainerr"
	timelinedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/timeline/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, limit int) ([]timelinedomain.Entry, error)
	Create(ctx context.Context, in timelinedomain.Entry) (timelinedomain.Entry, error)
}

type Usecases struct{ repo RepositoryPort }

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, limit int) ([]timelinedomain.Entry, error) {
	if orgID == uuid.Nil || entityID == uuid.Nil || strings.TrimSpace(entityType) == "" {
		return nil, domainerr.Validation("org_id, entity_type and entity_id are required")
	}
	return u.repo.List(ctx, orgID, strings.TrimSpace(entityType), entityID, limit)
}

func (u *Usecases) Record(ctx context.Context, in timelinedomain.Entry) (timelinedomain.Entry, error) {
	if in.OrgID == uuid.Nil || in.EntityID == uuid.Nil || strings.TrimSpace(in.EntityType) == "" {
		return timelinedomain.Entry{}, domainerr.Validation("org_id, entity_type and entity_id are required")
	}
	if strings.TrimSpace(in.EventType) == "" {
		return timelinedomain.Entry{}, domainerr.Validation("event_type is required")
	}
	if strings.TrimSpace(in.Title) == "" {
		return timelinedomain.Entry{}, domainerr.Validation("title is required")
	}
	in.EntityType = strings.TrimSpace(in.EntityType)
	in.EventType = strings.TrimSpace(in.EventType)
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.Actor = strings.TrimSpace(in.Actor)
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now().UTC()
	}
	return u.repo.Create(ctx, in)
}

func (u *Usecases) RecordEvent(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, eventType, title, description, actor string, metadata map[string]any) error {
	_, err := u.Record(ctx, timelinedomain.Entry{
		OrgID:       orgID,
		EntityType:  entityType,
		EntityID:    entityID,
		EventType:   eventType,
		Title:       title,
		Description: description,
		Actor:       actor,
		Metadata:    metadata,
	})
	return err
}
