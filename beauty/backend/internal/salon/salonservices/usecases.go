package salonservices

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "github.com/devpablocristo/pymes/beauty/backend/internal/salon/salonservices/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/vertvalues"
)

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Search string
}

type UpdateInput struct {
	Code            *string
	Name            *string
	Description     *string
	Category        *string
	DurationMinutes *int
	BasePrice       *float64
	Currency        *string
	TaxRate         *float64
	LinkedServiceID *string
	IsActive        *bool
}

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.SalonService, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.SalonService) (domain.SalonService, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.SalonService, error)
	Update(ctx context.Context, in domain.SalonService) (domain.SalonService, error)
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type controlPlanePort interface {
	GetService(ctx context.Context, orgID, serviceID string) (map[string]any, error)
}

type Usecases struct {
	repo  RepositoryPort
	audit AuditPort
	cp    controlPlanePort
}

func NewUsecases(repo RepositoryPort, audit AuditPort, cp controlPlanePort) *Usecases {
	return &Usecases{repo: repo, audit: audit, cp: cp}
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.SalonService, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in domain.SalonService, actor string) (domain.SalonService, error) {
	if err := u.enrichLinkedService(ctx, &in); err != nil {
		return domain.SalonService{}, err
	}
	if err := validateService(&in); err != nil {
		return domain.SalonService{}, err
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.SalonService{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "beauty.salon_service.created", "salon_service", out.ID.String(), map[string]any{"code": out.Code})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.SalonService, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.SalonService{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return domain.SalonService{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.SalonService, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.SalonService{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return domain.SalonService{}, err
	}
	if in.Code != nil {
		current.Code = strings.TrimSpace(*in.Code)
	}
	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		current.Description = strings.TrimSpace(*in.Description)
	}
	if in.Category != nil {
		current.Category = strings.TrimSpace(*in.Category)
	}
	if in.DurationMinutes != nil {
		current.DurationMinutes = *in.DurationMinutes
	}
	if in.BasePrice != nil {
		current.BasePrice = *in.BasePrice
	}
	if in.Currency != nil {
		current.Currency = strings.ToUpper(strings.TrimSpace(*in.Currency))
	}
	if in.TaxRate != nil {
		current.TaxRate = *in.TaxRate
	}
	if in.LinkedServiceID != nil {
		current.LinkedServiceID = vertvalues.ParseOptionalUUID(*in.LinkedServiceID)
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	if err := u.enrichLinkedService(ctx, &current); err != nil {
		return domain.SalonService{}, err
	}
	if err := validateService(&current); err != nil {
		return domain.SalonService{}, err
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.SalonService{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return domain.SalonService{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "beauty.salon_service.updated", "salon_service", out.ID.String(), map[string]any{"code": out.Code})
	}
	return out, nil
}

func validateService(in *domain.SalonService) error {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Category = strings.TrimSpace(in.Category)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Currency == "" {
		in.Currency = "ARS"
	}
	if len(in.Code) < 2 {
		return fmt.Errorf("code is required: %w", httperrors.ErrBadInput)
	}
	if len(in.Name) < 2 {
		return fmt.Errorf("name is required: %w", httperrors.ErrBadInput)
	}
	if in.DurationMinutes != 0 && (in.DurationMinutes < 5 || in.DurationMinutes > 480) {
		return fmt.Errorf("duration_minutes is invalid: %w", httperrors.ErrBadInput)
	}
	if in.DurationMinutes == 0 {
		in.DurationMinutes = 30
	}
	if in.BasePrice < 0 || in.TaxRate < 0 {
		return fmt.Errorf("numeric values must be positive: %w", httperrors.ErrBadInput)
	}
	return nil
}

func (u *Usecases) enrichLinkedService(ctx context.Context, in *domain.SalonService) error {
	if in.LinkedServiceID == nil || u.cp == nil {
		return nil
	}
	service, err := u.cp.GetService(ctx, in.OrgID.String(), in.LinkedServiceID.String())
	if err != nil {
		return fmt.Errorf("linked_service_id is invalid: %w", httperrors.ErrBadInput)
	}
	if strings.TrimSpace(in.Description) == "" {
		if description, ok := service["description"].(string); ok && strings.TrimSpace(description) != "" {
			in.Description = strings.TrimSpace(description)
		}
	}
	if in.BasePrice == 0 {
		in.BasePrice = vertvalues.ParseFloat(service["sale_price"])
	}
	if in.TaxRate == 0 {
		if value, ok := service["tax_rate"]; ok && value != nil {
			in.TaxRate = vertvalues.ParseFloat(value)
		}
	}
	return nil
}
