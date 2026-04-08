package admin

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/admin/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/shared/authz"
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
			"org_id":       orgID,
			"role":         role,
			"product_role": authz.ProductRole(role, scopes),
			"scopes":       scopes,
			"actor":        actor,
			"auth_method":  authMethod,
		},
		"settings": settings,
	}, nil
}

func (u *Usecases) GetTenantSettings(ctx context.Context, orgID string) (domain.TenantSettings, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return domain.TenantSettings{}, domainerr.Validation("invalid org_id")
	}
	return u.repo.GetTenantSettings(id), nil
}

func (u *Usecases) UpdateTenantSettings(ctx context.Context, orgID string, patch domain.TenantSettingsPatch, actor *string) (domain.TenantSettings, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return domain.TenantSettings{}, domainerr.Validation("invalid org_id")
	}
	if patch.SchedulingReminderHours != nil && *patch.SchedulingReminderHours < 0 {
		return domain.TenantSettings{}, domainerr.Validation("scheduling_reminder_hours must be >= 0")
	}
	if patch.TaxRate != nil && *patch.TaxRate < 0 {
		return domain.TenantSettings{}, domainerr.Validation("tax_rate must be >= 0")
	}
	if patch.SupportedCurrencies != nil {
		norm, err := domain.NormalizeSupportedCurrencies(*patch.SupportedCurrencies)
		if err != nil {
			return domain.TenantSettings{}, domainerr.Validation(err.Error())
		}
		patch.SupportedCurrencies = &norm
	}
	if patch.Vertical != nil {
		norm, err := domain.NormalizeVertical(*patch.Vertical)
		if err != nil {
			return domain.TenantSettings{}, domainerr.Validation(err.Error())
		}
		patch.Vertical = &norm
	}
	if patch.TeamSize != nil {
		norm, err := domain.NormalizeTeamSize(*patch.TeamSize)
		if err != nil {
			return domain.TenantSettings{}, domainerr.Validation(err.Error())
		}
		patch.TeamSize = &norm
	}
	if patch.Sells != nil {
		norm, err := domain.NormalizeSells(*patch.Sells)
		if err != nil {
			return domain.TenantSettings{}, domainerr.Validation(err.Error())
		}
		patch.Sells = &norm
	}
	if patch.PaymentMethod != nil {
		norm, err := domain.NormalizePaymentMethod(*patch.PaymentMethod)
		if err != nil {
			return domain.TenantSettings{}, domainerr.Validation(err.Error())
		}
		patch.PaymentMethod = &norm
	}
	if patch.ClientLabel != nil {
		norm, err := domain.NormalizeClientLabel(*patch.ClientLabel)
		if err != nil {
			return domain.TenantSettings{}, domainerr.Validation(err.Error())
		}
		patch.ClientLabel = &norm
	}
	return u.repo.UpdateTenantSettings(id, patch, actor), nil
}

func (u *Usecases) ListActivity(ctx context.Context, orgID string, limit int) ([]domain.ActivityEvent, error) {
	_ = ctx
	id, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("invalid org_id: %w", domainerr.Validation("invalid org_id"))
	}
	return u.repo.ListActivity(id, limit), nil
}
