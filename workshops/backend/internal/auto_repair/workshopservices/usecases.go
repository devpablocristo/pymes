package workshopservices

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/vertvalues"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workshopservices/usecases/domain"
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
	EstimatedHours  *float64
	BasePrice       *float64
	Currency        *string
	TaxRate         *float64
	LinkedServiceID *string
	IsActive        *bool
}

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Service, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID) ([]domain.Service, error)
	Create(ctx context.Context, in domain.Service) (domain.Service, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Service, error)
	Update(ctx context.Context, in domain.Service) (domain.Service, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
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

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.Service, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID) ([]domain.Service, error) {
	return u.repo.ListArchived(ctx, orgID)
}

func (u *Usecases) Create(ctx context.Context, in domain.Service, actor string) (domain.Service, error) {
	if err := u.enrichLinkedService(ctx, &in); err != nil {
		return domain.Service{}, err
	}
	if err := validateService(&in); err != nil {
		return domain.Service{}, err
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.Service{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "workshop_service.created", "workshop_service", out.ID.String(), map[string]any{"code": out.Code})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Service, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Service{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return domain.Service{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Service, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Service{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return domain.Service{}, err
	}
	if current.ArchivedAt != nil {
		return domain.Service{}, fmt.Errorf("service is archived: %w", httperrors.ErrBadInput)
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
	if in.EstimatedHours != nil {
		current.EstimatedHours = *in.EstimatedHours
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
		return domain.Service{}, err
	}
	if err := validateService(&current); err != nil {
		return domain.Service{}, err
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Service{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return domain.Service{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "workshop_service.updated", "workshop_service", out.ID.String(), map[string]any{"code": out.Code})
	}
	return out, nil
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "workshop_service.archived", "workshop_service", id.String(), nil)
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "workshop_service.restored", "workshop_service", id.String(), nil)
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		if errors.Is(err, ErrServiceReferencedInWorkOrders) {
			return fmt.Errorf("service referenced in work orders: %w", httperrors.ErrConflict)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "workshop_service.hard_deleted", "workshop_service", id.String(), nil)
	}
	return nil
}

func validateService(in *domain.Service) error {
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
	if in.EstimatedHours < 0 || in.BasePrice < 0 || in.TaxRate < 0 {
		return fmt.Errorf("numeric values must be positive: %w", httperrors.ErrBadInput)
	}
	return nil
}

func (u *Usecases) enrichLinkedService(ctx context.Context, in *domain.Service) error {
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
