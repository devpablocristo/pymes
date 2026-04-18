package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	servicedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/services/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]servicedomain.Service, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in servicedomain.Service) (servicedomain.Service, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (servicedomain.Service, error)
	Update(ctx context.Context, in servicedomain.Service) (servicedomain.Service, error)
	Archive(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	repo  RepositoryPort
	audit AuditPort
}

func NewUsecases(repo RepositoryPort, audit AuditPort) *Usecases {
	return &Usecases{repo: repo, audit: audit}
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]servicedomain.Service, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in servicedomain.Service, actor string) (servicedomain.Service, error) {
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 2 {
		return servicedomain.Service{}, fmt.Errorf("name must be at least 2 characters: %w", httperrors.ErrBadInput)
	}
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Currency == "" {
		in.Currency = "ARS"
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return servicedomain.Service{}, fmt.Errorf("service already exists: %w", httperrors.ErrConflict)
		}
		return servicedomain.Service{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "service.created", "service", out.ID.String(), map[string]any{"name": out.Name, "code": out.Code})
	}
	return out, nil
}

type UpdateInput struct {
	Code                   *string
	Name                   *string
	Description            *string
	CategoryCode           *string
	SalePrice              *float64
	CostPrice              *float64
	TaxRate                *float64
	Currency               *string
	DefaultDurationMinutes *int
	IsActive               *bool
	Tags                   *[]string
	Metadata               *map[string]any
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (servicedomain.Service, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return servicedomain.Service{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return servicedomain.Service{}, err
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
	if in.CategoryCode != nil {
		current.CategoryCode = strings.TrimSpace(*in.CategoryCode)
	}
	if in.SalePrice != nil {
		current.SalePrice = *in.SalePrice
	}
	if in.CostPrice != nil {
		current.CostPrice = *in.CostPrice
	}
	if in.TaxRate != nil {
		v := *in.TaxRate
		current.TaxRate = &v
	}
	if in.Currency != nil {
		current.Currency = strings.ToUpper(strings.TrimSpace(*in.Currency))
	}
	if in.DefaultDurationMinutes != nil {
		v := *in.DefaultDurationMinutes
		current.DefaultDurationMinutes = &v
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	if in.Tags != nil {
		current.Tags = append([]string(nil), (*in.Tags)...)
	}
	if in.Metadata != nil {
		current.Metadata = *in.Metadata
	}

	if len(current.Name) < 2 {
		return servicedomain.Service{}, fmt.Errorf("name must be at least 2 characters: %w", httperrors.ErrBadInput)
	}
	current.Currency = strings.ToUpper(strings.TrimSpace(current.Currency))
	if current.Currency == "" {
		current.Currency = "ARS"
	}

	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return servicedomain.Service{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		if errors.Is(err, ErrAlreadyExists) {
			return servicedomain.Service{}, fmt.Errorf("service already exists: %w", httperrors.ErrConflict)
		}
		return servicedomain.Service{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "service.updated", "service", out.ID.String(), map[string]any{"name": out.Name, "code": out.Code})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (servicedomain.Service, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return servicedomain.Service{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return servicedomain.Service{}, err
	}
	return out, nil
}

func (u *Usecases) Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Archive(ctx, orgID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "service.archived", "service", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "service.restored", "service", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Delete(ctx, orgID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "service.deleted", "service", id.String(), map[string]any{})
	}
	return nil
}
