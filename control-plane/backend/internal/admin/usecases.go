package admin

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/admin/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/pkg/http/errors"
)

type RepositoryPort interface {
	GetTenantSettings(orgID uuid.UUID) domain.TenantSettings
	UpdateTenantSettings(orgID uuid.UUID, plan string, hardLimits map[string]any, actor *string) domain.TenantSettings
	ListActivity(orgID uuid.UUID, limit int) []domain.ActivityEvent
}

type Usecases struct {
	repo RepositoryPort
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) GetBootstrap(ctx context.Context, orgID string, role string, scopes []string, actor string, authMethod string) (map[string]any, error) {
	_ = ctx
	settings, err := u.GetTenantSettings(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"auth": map[string]any{
			"org_id":      orgID,
			"role":        role,
			"scopes":      scopes,
			"actor":       actor,
			"auth_method": authMethod,
		},
		"settings": settings,
	}, nil
}

func (u *Usecases) GetTenantSettings(ctx context.Context, orgID string) (domain.TenantSettings, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return domain.TenantSettings{}, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	return u.repo.GetTenantSettings(id), nil
}

func (u *Usecases) UpdateTenantSettings(ctx context.Context, orgID, plan string, hardLimits map[string]any, actor *string) (domain.TenantSettings, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return domain.TenantSettings{}, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	return u.repo.UpdateTenantSettings(id, plan, hardLimits, actor), nil
}

func (u *Usecases) ListActivity(ctx context.Context, orgID string, limit int) ([]domain.ActivityEvent, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("invalid org_id: %w", httperrors.ErrBadInput)
	}
	return u.repo.ListActivity(id, limit), nil
}
