package areas

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/areas/usecases/domain"
)

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Search string
}

type UpdateInput struct {
	Name      *string
	SortOrder *int
}

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.DiningArea, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.DiningArea) (domain.DiningArea, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.DiningArea, error)
	Update(ctx context.Context, in domain.DiningArea) (domain.DiningArea, error)
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

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.DiningArea, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in domain.DiningArea, actor string) (domain.DiningArea, error) {
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 2 {
		return domain.DiningArea{}, fmt.Errorf("name too short: %w", httperrors.ErrBadInput)
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.DiningArea{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "restaurant.area.created", "dining_area", out.ID.String(), map[string]any{"name": out.Name})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.DiningArea, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.DiningArea{}, fmt.Errorf("dining area not found: %w", httperrors.ErrNotFound)
		}
		return domain.DiningArea{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.DiningArea, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.DiningArea{}, fmt.Errorf("dining area not found: %w", httperrors.ErrNotFound)
		}
		return domain.DiningArea{}, err
	}
	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.SortOrder != nil {
		current.SortOrder = *in.SortOrder
	}
	if len(current.Name) < 2 {
		return domain.DiningArea{}, fmt.Errorf("name too short: %w", httperrors.ErrBadInput)
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		return domain.DiningArea{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "restaurant.area.updated", "dining_area", out.ID.String(), nil)
	}
	return out, nil
}
