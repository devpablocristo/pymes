package employees

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"
	archive "github.com/devpablocristo/modules/crud/archive/go/archive"
	"github.com/google/uuid"
	"gorm.io/gorm"

	empdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/employees/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]empdomain.Employee, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]empdomain.Employee, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (empdomain.Employee, error)
	Create(ctx context.Context, in empdomain.Employee) (empdomain.Employee, error)
	Update(ctx context.Context, in empdomain.Employee) (empdomain.Employee, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
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

func (u *Usecases) List(ctx context.Context, p ListParams) ([]empdomain.Employee, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]empdomain.Employee, error) {
	return u.repo.ListArchived(ctx, orgID, limit)
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (empdomain.Employee, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return empdomain.Employee{}, domainerr.NotFoundf("employee", id.String())
		}
		return empdomain.Employee{}, err
	}
	return out, nil
}

type CreateInput struct {
	OrgID      uuid.UUID
	FirstName  string
	LastName   string
	Email      string
	Phone      string
	Position   string
	Status     string
	HireDate   string
	EndDate    string
	Notes      string
	IsFavorite bool
	Tags       []string
	CreatedBy  string
}

func (u *Usecases) Create(ctx context.Context, in CreateInput) (empdomain.Employee, error) {
	status := normalizeStatus(in.Status)
	if !isValidStatus(status) {
		return empdomain.Employee{}, fmt.Errorf("invalid status: %w", httperrors.ErrBadInput)
	}
	if strings.TrimSpace(in.FirstName) == "" && strings.TrimSpace(in.LastName) == "" {
		return empdomain.Employee{}, fmt.Errorf("first_name or last_name is required: %w", httperrors.ErrBadInput)
	}
	hire, err := parseOptionalDate(in.HireDate)
	if err != nil {
		return empdomain.Employee{}, fmt.Errorf("invalid hire_date: %w", httperrors.ErrBadInput)
	}
	end, err := parseOptionalDate(in.EndDate)
	if err != nil {
		return empdomain.Employee{}, fmt.Errorf("invalid end_date: %w", httperrors.ErrBadInput)
	}
	out, err := u.repo.Create(ctx, empdomain.Employee{
		OrgID:      in.OrgID,
		FirstName:  in.FirstName,
		LastName:   in.LastName,
		Email:      in.Email,
		Phone:      in.Phone,
		Position:   in.Position,
		Status:     empdomain.EmployeeStatus(status),
		HireDate:   hire,
		EndDate:    end,
		Notes:      in.Notes,
		IsFavorite: in.IsFavorite,
		Tags:       in.Tags,
		CreatedBy:  in.CreatedBy,
	})
	if err != nil {
		return empdomain.Employee{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.CreatedBy, "employee.created", "employee", out.ID.String(), map[string]any{
			"name": strings.TrimSpace(out.FirstName + " " + out.LastName),
		})
	}
	return out, nil
}

type UpdateInput struct {
	OrgID      uuid.UUID
	ID         uuid.UUID
	FirstName  *string
	LastName   *string
	Email      *string
	Phone      *string
	Position   *string
	Status     *string
	HireDate   *string
	EndDate    *string
	Notes      *string
	IsFavorite *bool
	Tags       *[]string
	Actor      string
}

func (u *Usecases) Update(ctx context.Context, in UpdateInput) (empdomain.Employee, error) {
	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return empdomain.Employee{}, domainerr.NotFoundf("employee", in.ID.String())
		}
		return empdomain.Employee{}, err
	}
	if err := archive.IfArchived(current.ArchivedAt, "employee"); err != nil {
		return empdomain.Employee{}, err
	}
	if in.FirstName != nil {
		current.FirstName = *in.FirstName
	}
	if in.LastName != nil {
		current.LastName = *in.LastName
	}
	if in.Email != nil {
		current.Email = *in.Email
	}
	if in.Phone != nil {
		current.Phone = *in.Phone
	}
	if in.Position != nil {
		current.Position = *in.Position
	}
	if in.Status != nil {
		s := normalizeStatus(*in.Status)
		if !isValidStatus(s) {
			return empdomain.Employee{}, fmt.Errorf("invalid status: %w", httperrors.ErrBadInput)
		}
		current.Status = empdomain.EmployeeStatus(s)
	}
	if in.HireDate != nil {
		d, err := parseOptionalDate(*in.HireDate)
		if err != nil {
			return empdomain.Employee{}, fmt.Errorf("invalid hire_date: %w", httperrors.ErrBadInput)
		}
		current.HireDate = d
	}
	if in.EndDate != nil {
		d, err := parseOptionalDate(*in.EndDate)
		if err != nil {
			return empdomain.Employee{}, fmt.Errorf("invalid end_date: %w", httperrors.ErrBadInput)
		}
		current.EndDate = d
	}
	if in.Notes != nil {
		current.Notes = *in.Notes
	}
	if in.IsFavorite != nil {
		current.IsFavorite = *in.IsFavorite
	}
	if in.Tags != nil {
		current.Tags = *in.Tags
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return empdomain.Employee{}, domainerr.NotFoundf("employee", in.ID.String())
		}
		return empdomain.Employee{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.Actor, "employee.updated", "employee", out.ID.String(), nil)
	}
	return out, nil
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("employee", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "employee.archived", "employee", id.String(), nil)
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("employee", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "employee.restored", "employee", id.String(), nil)
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("employee", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "employee.hard_deleted", "employee", id.String(), nil)
	}
	return nil
}

func normalizeStatus(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	if s == "" {
		s = string(empdomain.EmployeeStatusActive)
	}
	return s
}

func isValidStatus(s string) bool {
	switch empdomain.EmployeeStatus(s) {
	case empdomain.EmployeeStatusActive, empdomain.EmployeeStatusInactive, empdomain.EmployeeStatusTerminated:
		return true
	}
	return false
}

func parseOptionalDate(raw string) (*time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}
	t = t.UTC()
	return &t, nil
}
