package intakes

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/intakes/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, orgID uuid.UUID) ([]domain.Intake, error)
	Create(ctx context.Context, in domain.Intake) (domain.Intake, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Intake, error)
	Update(ctx context.Context, in domain.Intake) (domain.Intake, error)
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

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID) ([]domain.Intake, error) {
	return u.repo.List(ctx, orgID)
}

func (u *Usecases) Create(ctx context.Context, in domain.Intake, actor string) (domain.Intake, error) {
	if in.ProfileID == uuid.Nil {
		return domain.Intake{}, fmt.Errorf("profile_id is required: %w", httperrors.ErrBadInput)
	}
	if in.Status == "" {
		in.Status = domain.IntakeStatusDraft
	}
	if in.Status != domain.IntakeStatusDraft {
		return domain.Intake{}, fmt.Errorf("new intakes must start in draft status: %w", httperrors.ErrBadInput)
	}
	if in.Payload == nil {
		in.Payload = map[string]any{}
	}
	in.ServiceID = normalizeServiceID(in.ServiceID)

	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.Intake{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "intake.created", "intake", out.ID.String(), map[string]any{"status": out.Status})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Intake, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Intake{}, fmt.Errorf("intake not found: %w", httperrors.ErrNotFound)
		}
		return domain.Intake{}, err
	}
	return out, nil
}

type UpdateInput struct {
	BookingID       *uuid.UUID
	CustomerPartyID *uuid.UUID
	ServiceID       *uuid.UUID
	IsFavorite      *bool
	Tags            *[]string
	Payload         *map[string]any
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Intake, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Intake{}, fmt.Errorf("intake not found: %w", httperrors.ErrNotFound)
		}
		return domain.Intake{}, err
	}

	if current.Status != domain.IntakeStatusDraft {
		return domain.Intake{}, fmt.Errorf("only draft intakes can be updated: %w", httperrors.ErrNotDraft)
	}

	if in.BookingID != nil {
		current.BookingID = in.BookingID
	}
	if in.CustomerPartyID != nil {
		current.CustomerPartyID = in.CustomerPartyID
	}
	if in.ServiceID != nil {
		current.ServiceID = normalizeServiceID(in.ServiceID)
	}
	if in.IsFavorite != nil {
		current.IsFavorite = *in.IsFavorite
	}
	if in.Tags != nil {
		current.Tags = *in.Tags
	}
	if in.Payload != nil {
		current.Payload = *in.Payload
	}

	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Intake{}, fmt.Errorf("intake not found: %w", httperrors.ErrNotFound)
		}
		return domain.Intake{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "intake.updated", "intake", out.ID.String(), map[string]any{"status": out.Status})
	}
	return out, nil
}

func normalizeServiceID(serviceID *uuid.UUID) *uuid.UUID {
	if serviceID != nil && *serviceID != uuid.Nil {
		canonical := *serviceID
		return &canonical
	}
	return nil
}

func (u *Usecases) Submit(ctx context.Context, orgID, id uuid.UUID, actor string) (domain.Intake, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Intake{}, fmt.Errorf("intake not found: %w", httperrors.ErrNotFound)
		}
		return domain.Intake{}, err
	}
	if current.Status != domain.IntakeStatusDraft {
		return domain.Intake{}, fmt.Errorf("only draft intakes can be submitted: %w", httperrors.ErrNotDraft)
	}
	current.Status = domain.IntakeStatusSubmitted
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		return domain.Intake{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "intake.submitted", "intake", out.ID.String(), map[string]any{})
	}
	return out, nil
}
