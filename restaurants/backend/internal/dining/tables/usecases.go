package tables

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/tables/usecases/domain"
)

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Search string
	AreaID *uuid.UUID
}

type UpdateInput struct {
	AreaID     *uuid.UUID
	Code       *string
	Label      *string
	Capacity   *int
	Status     *string
	Notes      *string
	IsFavorite *bool
	Tags       *[]string
}

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.DiningTable, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.DiningTable) (domain.DiningTable, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.DiningTable, error)
	Update(ctx context.Context, in domain.DiningTable) (domain.DiningTable, error)
}

type AreaLookup interface {
	ExistsForOrg(ctx context.Context, orgID, areaID uuid.UUID) (bool, error)
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	repo       RepositoryPort
	areaLookup AreaLookup
	audit      AuditPort
}

func NewUsecases(repo RepositoryPort, areaLookup AreaLookup, audit AuditPort) *Usecases {
	return &Usecases{repo: repo, areaLookup: areaLookup, audit: audit}
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.DiningTable, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in domain.DiningTable, actor string) (domain.DiningTable, error) {
	if err := u.normalizeAndValidate(&in); err != nil {
		return domain.DiningTable{}, err
	}
	ok, err := u.areaLookup.ExistsForOrg(ctx, in.OrgID, in.AreaID)
	if err != nil {
		return domain.DiningTable{}, err
	}
	if !ok {
		return domain.DiningTable{}, fmt.Errorf("dining area not found: %w", httperrors.ErrNotFound)
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		if errors.Is(err, ErrDuplicateTableCode) {
			return domain.DiningTable{}, fmt.Errorf("table code already exists: %w", httperrors.ErrConflict)
		}
		return domain.DiningTable{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "restaurant.table.created", "dining_table", out.ID.String(), map[string]any{"code": out.Code})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.DiningTable, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.DiningTable{}, fmt.Errorf("dining table not found: %w", httperrors.ErrNotFound)
		}
		return domain.DiningTable{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.DiningTable, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.DiningTable{}, fmt.Errorf("dining table not found: %w", httperrors.ErrNotFound)
		}
		return domain.DiningTable{}, err
	}
	if in.AreaID != nil {
		current.AreaID = *in.AreaID
	}
	if in.Code != nil {
		current.Code = strings.TrimSpace(*in.Code)
	}
	if in.Label != nil {
		current.Label = strings.TrimSpace(*in.Label)
	}
	if in.Capacity != nil {
		current.Capacity = *in.Capacity
	}
	if in.Status != nil {
		current.Status = strings.TrimSpace(*in.Status)
	}
	if in.Notes != nil {
		current.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.IsFavorite != nil {
		current.IsFavorite = *in.IsFavorite
	}
	if in.Tags != nil {
		current.Tags = *in.Tags
	}
	if err := u.normalizeAndValidate(&current); err != nil {
		return domain.DiningTable{}, err
	}
	ok, err := u.areaLookup.ExistsForOrg(ctx, current.OrgID, current.AreaID)
	if err != nil {
		return domain.DiningTable{}, err
	}
	if !ok {
		return domain.DiningTable{}, fmt.Errorf("dining area not found: %w", httperrors.ErrNotFound)
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, ErrDuplicateTableCode) {
			return domain.DiningTable{}, fmt.Errorf("table code already exists: %w", httperrors.ErrConflict)
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.DiningTable{}, fmt.Errorf("dining table not found: %w", httperrors.ErrNotFound)
		}
		return domain.DiningTable{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "restaurant.table.updated", "dining_table", out.ID.String(), nil)
	}
	return out, nil
}

func (u *Usecases) normalizeAndValidate(in *domain.DiningTable) error {
	in.Code = strings.TrimSpace(in.Code)
	in.Label = strings.TrimSpace(in.Label)
	in.Status = strings.TrimSpace(in.Status)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Code == "" {
		return fmt.Errorf("code required: %w", httperrors.ErrBadInput)
	}
	if in.Capacity < 1 || in.Capacity > 99 {
		return fmt.Errorf("invalid capacity: %w", httperrors.ErrBadInput)
	}
	if in.Status == "" {
		in.Status = "available"
	}
	if !isValidTableStatus(in.Status) {
		return fmt.Errorf("invalid status: %w", httperrors.ErrBadInput)
	}
	return nil
}

func isValidTableStatus(s string) bool {
	switch s {
	case "available", "occupied", "reserved", "cleaning":
		return true
	default:
		return false
	}
}
