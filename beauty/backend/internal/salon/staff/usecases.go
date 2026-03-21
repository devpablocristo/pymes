package staff

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/beauty/backend/internal/salon/staff/usecases/domain"
)

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Search string
}

type UpdateInput struct {
	DisplayName *string
	Role        *string
	Color       *string
	IsActive    *bool
	Notes       *string
}

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.StaffMember, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.StaffMember) (domain.StaffMember, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.StaffMember, error)
	Update(ctx context.Context, in domain.StaffMember) (domain.StaffMember, error)
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

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.StaffMember, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in domain.StaffMember, actor string) (domain.StaffMember, error) {
	if err := validateStaff(&in); err != nil {
		return domain.StaffMember{}, err
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.StaffMember{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "beauty.staff.created", "staff_member", out.ID.String(), map[string]any{"name": out.DisplayName})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.StaffMember, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.StaffMember{}, fmt.Errorf("staff member not found: %w", httperrors.ErrNotFound)
		}
		return domain.StaffMember{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.StaffMember, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.StaffMember{}, fmt.Errorf("staff member not found: %w", httperrors.ErrNotFound)
		}
		return domain.StaffMember{}, err
	}
	if in.DisplayName != nil {
		current.DisplayName = strings.TrimSpace(*in.DisplayName)
	}
	if in.Role != nil {
		current.Role = strings.TrimSpace(*in.Role)
	}
	if in.Color != nil {
		current.Color = strings.TrimSpace(*in.Color)
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	if in.Notes != nil {
		current.Notes = strings.TrimSpace(*in.Notes)
	}
	if err := validateStaff(&current); err != nil {
		return domain.StaffMember{}, err
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.StaffMember{}, fmt.Errorf("staff member not found: %w", httperrors.ErrNotFound)
		}
		return domain.StaffMember{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "beauty.staff.updated", "staff_member", out.ID.String(), map[string]any{"name": out.DisplayName})
	}
	return out, nil
}

func validateStaff(in *domain.StaffMember) error {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.Role = strings.TrimSpace(in.Role)
	in.Color = strings.TrimSpace(in.Color)
	in.Notes = strings.TrimSpace(in.Notes)
	if len(in.DisplayName) < 2 {
		return fmt.Errorf("display_name is required: %w", httperrors.ErrBadInput)
	}
	if in.Color == "" {
		in.Color = "#6366f1"
	}
	return nil
}
