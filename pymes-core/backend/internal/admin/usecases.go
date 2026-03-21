package admin

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/backend/go/apperror"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/admin/usecases/domain"
)

type RepositoryPort interface {
	GetTenantSettings(orgID uuid.UUID) domain.TenantSettings
	UpdateTenantSettings(orgID uuid.UUID, patch domain.TenantSettingsPatch, actor *string) domain.TenantSettings
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
		return domain.TenantSettings{}, apperror.NewBadInput("invalid org_id")
	}
	return u.repo.GetTenantSettings(id), nil
}

func (u *Usecases) UpdateTenantSettings(ctx context.Context, orgID string, patch domain.TenantSettingsPatch, actor *string) (domain.TenantSettings, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return domain.TenantSettings{}, apperror.NewBadInput("invalid org_id")
	}
	if patch.AppointmentReminderHours != nil && *patch.AppointmentReminderHours < 0 {
		return domain.TenantSettings{}, apperror.NewBadInput("appointment_reminder_hours must be >= 0")
	}
	if patch.TaxRate != nil && *patch.TaxRate < 0 {
		return domain.TenantSettings{}, apperror.NewBadInput("tax_rate must be >= 0")
	}
	return u.repo.UpdateTenantSettings(id, patch, actor), nil
}

func (u *Usecases) ListActivity(ctx context.Context, orgID string, limit int) ([]domain.ActivityEvent, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("invalid org_id: %w", apperror.NewBadInput("invalid org_id"))
	}
	return u.repo.ListActivity(id, limit), nil
}
