package vehicles

import (
	"errors"
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles/usecases/domain"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/values"
)

type ListParams struct {
	OrgID  uuid.UUID
	Limit  int
	After  *uuid.UUID
	Search string
}

type UpdateInput struct {
	CustomerID   *string
	CustomerName *string
	LicensePlate *string
	VIN          *string
	Make         *string
	Model        *string
	Year         *int
	Kilometers   *int
	Color        *string
	Notes        *string
}

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.Vehicle, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in domain.Vehicle) (domain.Vehicle, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Vehicle, error)
	Update(ctx context.Context, in domain.Vehicle) (domain.Vehicle, error)
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

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.Vehicle, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in domain.Vehicle, actor string) (domain.Vehicle, error) {
	if err := u.enrichCustomer(ctx, &in); err != nil {
		return domain.Vehicle{}, err
	}
	if err := validateVehicle(&in); err != nil {
		return domain.Vehicle{}, err
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "vehicle.created", "vehicle", out.ID.String(), map[string]any{"license_plate": out.LicensePlate})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Vehicle, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Vehicle{}, fmt.Errorf("vehicle not found: %w", httperrors.ErrNotFound)
		}
		return domain.Vehicle{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.Vehicle, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Vehicle{}, fmt.Errorf("vehicle not found: %w", httperrors.ErrNotFound)
		}
		return domain.Vehicle{}, err
	}
	if in.CustomerID != nil {
		current.CustomerID = values.ParseOptionalUUID(*in.CustomerID)
	}
	if in.CustomerName != nil {
		current.CustomerName = strings.TrimSpace(*in.CustomerName)
	}
	if in.LicensePlate != nil {
		current.LicensePlate = normalizePlate(*in.LicensePlate)
	}
	if in.VIN != nil {
		current.VIN = strings.ToUpper(strings.TrimSpace(*in.VIN))
	}
	if in.Make != nil {
		current.Make = strings.TrimSpace(*in.Make)
	}
	if in.Model != nil {
		current.Model = strings.TrimSpace(*in.Model)
	}
	if in.Year != nil {
		current.Year = *in.Year
	}
	if in.Kilometers != nil {
		current.Kilometers = *in.Kilometers
	}
	if in.Color != nil {
		current.Color = strings.TrimSpace(*in.Color)
	}
	if in.Notes != nil {
		current.Notes = strings.TrimSpace(*in.Notes)
	}
	if err := u.enrichCustomer(ctx, &current); err != nil {
		return domain.Vehicle{}, err
	}
	if err := validateVehicle(&current); err != nil {
		return domain.Vehicle{}, err
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Vehicle{}, fmt.Errorf("vehicle not found: %w", httperrors.ErrNotFound)
		}
		return domain.Vehicle{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "vehicle.updated", "vehicle", out.ID.String(), map[string]any{"license_plate": out.LicensePlate})
	}
	return out, nil
}

func (u *Usecases) enrichCustomer(ctx context.Context, in *domain.Vehicle) error {
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

func validateVehicle(in *domain.Vehicle) error {
	in.CustomerName = strings.TrimSpace(in.CustomerName)
	in.LicensePlate = normalizePlate(in.LicensePlate)
	in.VIN = strings.ToUpper(strings.TrimSpace(in.VIN))
	in.Make = strings.TrimSpace(in.Make)
	in.Model = strings.TrimSpace(in.Model)
	in.Color = strings.TrimSpace(in.Color)
	in.Notes = strings.TrimSpace(in.Notes)

	if in.LicensePlate == "" {
		return fmt.Errorf("license_plate is required: %w", httperrors.ErrBadInput)
	}
	if len(in.Make) < 2 {
		return fmt.Errorf("make is required: %w", httperrors.ErrBadInput)
	}
	if len(in.Model) < 1 {
		return fmt.Errorf("model is required: %w", httperrors.ErrBadInput)
	}
	if in.Year < 0 || in.Year > 2100 {
		return fmt.Errorf("year is invalid: %w", httperrors.ErrBadInput)
	}
	if in.Kilometers < 0 {
		return fmt.Errorf("kilometers is invalid: %w", httperrors.ErrBadInput)
	}
	return nil
}

func normalizePlate(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}
