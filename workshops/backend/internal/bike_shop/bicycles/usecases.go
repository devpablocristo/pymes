package bicycles

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/bicycles/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/vertvalues"
)

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Search string
}

type UpdateInput struct {
	CustomerID      *string
	CustomerName    *string
	FrameNumber     *string
	Make            *string
	Model           *string
	BikeType        *string
	Size            *string
	WheelSizeInches *int
	Color           *string
	EbikeNotes      *string
	Notes           *string
}

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Bicycle, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Bicycle) (domain.Bicycle, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Bicycle, error)
	Update(ctx context.Context, in domain.Bicycle) (domain.Bicycle, error)
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type controlPlanePort interface {
	GetCustomer(ctx context.Context, orgID, customerID string) (map[string]any, error)
	GetParty(ctx context.Context, orgID, partyID string) (map[string]any, error)
}

type Usecases struct {
	repo  RepositoryPort
	audit AuditPort
	cp    controlPlanePort
}

func NewUsecases(repo RepositoryPort, audit AuditPort, cp controlPlanePort) *Usecases {
	return &Usecases{repo: repo, audit: audit, cp: cp}
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.Bicycle, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in domain.Bicycle, actor string) (domain.Bicycle, error) {
	if err := u.enrichCustomer(ctx, &in); err != nil {
		return domain.Bicycle{}, err
	}
	if err := validateBicycle(&in); err != nil {
		return domain.Bicycle{}, err
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.Bicycle{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "bicycle.created", "bicycle", out.ID.String(), map[string]any{"frame_number": out.FrameNumber})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Bicycle, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Bicycle{}, fmt.Errorf("bicycle not found: %w", httperrors.ErrNotFound)
		}
		return domain.Bicycle{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Bicycle, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Bicycle{}, fmt.Errorf("bicycle not found: %w", httperrors.ErrNotFound)
		}
		return domain.Bicycle{}, err
	}
	if in.CustomerID != nil {
		current.CustomerID = vertvalues.ParseOptionalUUID(*in.CustomerID)
	}
	if in.CustomerName != nil {
		current.CustomerName = strings.TrimSpace(*in.CustomerName)
	}
	if in.FrameNumber != nil {
		current.FrameNumber = normalizeFrame(*in.FrameNumber)
	}
	if in.Make != nil {
		current.Make = strings.TrimSpace(*in.Make)
	}
	if in.Model != nil {
		current.Model = strings.TrimSpace(*in.Model)
	}
	if in.BikeType != nil {
		current.BikeType = strings.TrimSpace(*in.BikeType)
	}
	if in.Size != nil {
		current.Size = strings.TrimSpace(*in.Size)
	}
	if in.WheelSizeInches != nil {
		current.WheelSizeInches = *in.WheelSizeInches
	}
	if in.Color != nil {
		current.Color = strings.TrimSpace(*in.Color)
	}
	if in.EbikeNotes != nil {
		current.EbikeNotes = strings.TrimSpace(*in.EbikeNotes)
	}
	if in.Notes != nil {
		current.Notes = strings.TrimSpace(*in.Notes)
	}
	if err := u.enrichCustomer(ctx, &current); err != nil {
		return domain.Bicycle{}, err
	}
	if err := validateBicycle(&current); err != nil {
		return domain.Bicycle{}, err
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Bicycle{}, fmt.Errorf("bicycle not found: %w", httperrors.ErrNotFound)
		}
		return domain.Bicycle{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "bicycle.updated", "bicycle", out.ID.String(), map[string]any{"frame_number": out.FrameNumber})
	}
	return out, nil
}

func (u *Usecases) enrichCustomer(ctx context.Context, in *domain.Bicycle) error {
	if in.CustomerID == nil {
		return nil
	}
	if u.cp == nil {
		return nil
	}
	customer, err := u.cp.GetCustomer(ctx, in.OrgID.String(), in.CustomerID.String())
	if err == nil {
		if strings.TrimSpace(in.CustomerName) == "" {
			if name, ok := customer["name"].(string); ok {
				in.CustomerName = strings.TrimSpace(name)
			}
		}
		return nil
	}
	party, err := u.cp.GetParty(ctx, in.OrgID.String(), in.CustomerID.String())
	if err != nil {
		return fmt.Errorf("customer_id is invalid: %w", httperrors.ErrBadInput)
	}
	if strings.TrimSpace(in.CustomerName) == "" {
		if displayName, ok := party["display_name"].(string); ok {
			in.CustomerName = strings.TrimSpace(displayName)
		}
	}
	return nil
}

func validateBicycle(in *domain.Bicycle) error {
	in.CustomerName = strings.TrimSpace(in.CustomerName)
	in.FrameNumber = normalizeFrame(in.FrameNumber)
	in.Make = strings.TrimSpace(in.Make)
	in.Model = strings.TrimSpace(in.Model)
	in.BikeType = strings.TrimSpace(in.BikeType)
	in.Size = strings.TrimSpace(in.Size)
	in.Color = strings.TrimSpace(in.Color)
	in.EbikeNotes = strings.TrimSpace(in.EbikeNotes)
	in.Notes = strings.TrimSpace(in.Notes)

	if in.FrameNumber == "" {
		return fmt.Errorf("frame_number is required: %w", httperrors.ErrBadInput)
	}
	if len(in.Make) < 2 {
		return fmt.Errorf("make is required: %w", httperrors.ErrBadInput)
	}
	if len(in.Model) < 1 {
		return fmt.Errorf("model is required: %w", httperrors.ErrBadInput)
	}
	if in.WheelSizeInches < 0 || in.WheelSizeInches > 99 {
		return fmt.Errorf("wheel_size_inches is invalid: %w", httperrors.ErrBadInput)
	}
	return nil
}

func normalizeFrame(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}
