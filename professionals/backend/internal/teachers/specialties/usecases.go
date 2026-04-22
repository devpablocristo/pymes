package specialties

import (
	"errors"
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/specialties/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Specialty, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Specialty) (domain.Specialty, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Specialty, error)
	Update(ctx context.Context, in domain.Specialty) (domain.Specialty, error)
	CodeExists(ctx context.Context, orgID uuid.UUID, code string, excludeID *uuid.UUID) (bool, error)
	AssignProfessionals(ctx context.Context, orgID, specialtyID uuid.UUID, profileIDs []uuid.UUID) error
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

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.Specialty, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in domain.Specialty, actor string) (domain.Specialty, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)

	if len(in.Code) < 2 {
		return domain.Specialty{}, fmt.Errorf("code must be at least 2 characters: %w", httperrors.ErrBadInput)
	}
	if len(in.Name) < 2 {
		return domain.Specialty{}, fmt.Errorf("name must be at least 2 characters: %w", httperrors.ErrBadInput)
	}

	exists, err := u.repo.CodeExists(ctx, in.OrgID, in.Code, nil)
	if err != nil {
		return domain.Specialty{}, err
	}
	if exists {
		return domain.Specialty{}, fmt.Errorf("code '%s' already in use: %w", in.Code, httperrors.ErrConflict)
	}

	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.Specialty{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "specialty.created", "specialty", out.ID.String(), map[string]any{"code": out.Code, "name": out.Name})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Specialty, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Specialty{}, fmt.Errorf("specialty not found: %w", httperrors.ErrNotFound)
		}
		return domain.Specialty{}, err
	}
	return out, nil
}

type UpdateInput struct {
	Code        *string
	Name        *string
	Description *string
	IsActive    *bool
	IsFavorite  *bool
	Tags        *[]string
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Specialty, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Specialty{}, fmt.Errorf("specialty not found: %w", httperrors.ErrNotFound)
		}
		return domain.Specialty{}, err
	}

	if in.Code != nil {
		code := strings.TrimSpace(*in.Code)
		if code != current.Code {
			exists, err := u.repo.CodeExists(ctx, orgID, code, &id)
			if err != nil {
				return domain.Specialty{}, err
			}
			if exists {
				return domain.Specialty{}, fmt.Errorf("code '%s' already in use: %w", code, httperrors.ErrConflict)
			}
			current.Code = code
		}
	}
	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		current.Description = strings.TrimSpace(*in.Description)
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	if in.IsFavorite != nil {
		current.IsFavorite = *in.IsFavorite
	}
	if in.Tags != nil {
		current.Tags = *in.Tags
	}

	if len(current.Code) < 2 {
		return domain.Specialty{}, fmt.Errorf("code must be at least 2 characters: %w", httperrors.ErrBadInput)
	}
	if len(current.Name) < 2 {
		return domain.Specialty{}, fmt.Errorf("name must be at least 2 characters: %w", httperrors.ErrBadInput)
	}

	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Specialty{}, fmt.Errorf("specialty not found: %w", httperrors.ErrNotFound)
		}
		return domain.Specialty{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "specialty.updated", "specialty", out.ID.String(), map[string]any{"code": out.Code, "name": out.Name})
	}
	return out, nil
}

func (u *Usecases) AssignProfessionals(ctx context.Context, orgID, specialtyID uuid.UUID, profileIDs []uuid.UUID, actor string) error {
	if _, err := u.repo.GetByID(ctx, orgID, specialtyID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("specialty not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if err := u.repo.AssignProfessionals(ctx, orgID, specialtyID, profileIDs); err != nil {
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "specialty.professionals_assigned", "specialty", specialtyID.String(), map[string]any{"count": len(profileIDs)})
	}
	return nil
}
