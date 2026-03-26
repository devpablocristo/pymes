package suppliers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	supplierdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/suppliers/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]supplierdomain.Supplier, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in supplierdomain.Supplier) (supplierdomain.Supplier, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (supplierdomain.Supplier, error)
	Update(ctx context.Context, in supplierdomain.Supplier) (supplierdomain.Supplier, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	ListArchived(ctx context.Context, orgID uuid.UUID) ([]supplierdomain.Supplier, error)
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
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

func (u *Usecases) List(ctx context.Context, p ListParams) ([]supplierdomain.Supplier, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in supplierdomain.Supplier, actor string) (supplierdomain.Supplier, error) {
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 2 {
		return supplierdomain.Supplier{}, fmt.Errorf("name must be at least 2 characters: %w", httperrors.ErrBadInput)
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return supplierdomain.Supplier{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "supplier.created", "supplier", out.ID.String(), map[string]any{"name": out.Name})
	}
	return out, nil
}

type UpdateInput struct {
	Name        *string
	TaxID       *string
	Email       *string
	Phone       *string
	Address     *supplierdomain.Address
	ContactName *string
	Notes       *string
	Tags        *[]string
	Metadata    *map[string]any
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (supplierdomain.Supplier, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return supplierdomain.Supplier{}, fmt.Errorf("supplier not found: %w", httperrors.ErrNotFound)
		}
		return supplierdomain.Supplier{}, err
	}
	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.TaxID != nil {
		current.TaxID = strings.TrimSpace(*in.TaxID)
	}
	if in.Email != nil {
		current.Email = strings.TrimSpace(*in.Email)
	}
	if in.Phone != nil {
		current.Phone = strings.TrimSpace(*in.Phone)
	}
	if in.Address != nil {
		current.Address = *in.Address
	}
	if in.ContactName != nil {
		current.ContactName = strings.TrimSpace(*in.ContactName)
	}
	if in.Notes != nil {
		current.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.Tags != nil {
		current.Tags = append([]string(nil), (*in.Tags)...)
	}
	if in.Metadata != nil {
		current.Metadata = *in.Metadata
	}
	if len(current.Name) < 2 {
		return supplierdomain.Supplier{}, fmt.Errorf("name must be at least 2 characters: %w", httperrors.ErrBadInput)
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return supplierdomain.Supplier{}, fmt.Errorf("supplier not found: %w", httperrors.ErrNotFound)
		}
		return supplierdomain.Supplier{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "supplier.updated", "supplier", out.ID.String(), map[string]any{"name": out.Name})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (supplierdomain.Supplier, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return supplierdomain.Supplier{}, fmt.Errorf("supplier not found: %w", httperrors.ErrNotFound)
		}
		return supplierdomain.Supplier{}, err
	}
	return out, nil
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("supplier not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "supplier.deleted", "supplier", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID) ([]supplierdomain.Supplier, error) {
	return u.repo.ListArchived(ctx, orgID)
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("supplier not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "supplier.restored", "supplier", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("supplier not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "supplier.hard_deleted", "supplier", id.String(), map[string]any{})
	}
	return nil
}
