package workshopservices

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/values"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]Service, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in Service) (Service, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (Service, error)
	Update(ctx context.Context, in Service) (Service, error)
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type controlPlanePort interface {
	GetProduct(ctx context.Context, orgID, productID string) (map[string]any, error)
}

type Usecases struct {
	repo  RepositoryPort
	audit AuditPort
	cp    controlPlanePort
}

func NewUsecases(repo RepositoryPort, audit AuditPort, cp controlPlanePort) *Usecases {
	return &Usecases{repo: repo, audit: audit, cp: cp}
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]Service, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in Service, actor string) (Service, error) {
	if err := u.enrichLinkedProduct(ctx, &in); err != nil {
		return Service{}, err
	}
	if err := validateService(&in); err != nil {
		return Service{}, err
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return Service{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "workshop_service.created", "workshop_service", out.ID.String(), map[string]any{"code": out.Code})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (Service, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return Service{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return Service{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (Service, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return Service{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return Service{}, err
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
	if in.Category != nil {
		current.Category = strings.TrimSpace(*in.Category)
	}
	if in.EstimatedHours != nil {
		current.EstimatedHours = *in.EstimatedHours
	}
	if in.BasePrice != nil {
		current.BasePrice = *in.BasePrice
	}
	if in.Currency != nil {
		current.Currency = strings.ToUpper(strings.TrimSpace(*in.Currency))
	}
	if in.TaxRate != nil {
		current.TaxRate = *in.TaxRate
	}
	if in.LinkedProductID != nil {
		current.LinkedProductID = values.ParseOptionalUUID(*in.LinkedProductID)
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	if err := u.enrichLinkedProduct(ctx, &current); err != nil {
		return Service{}, err
	}
	if err := validateService(&current); err != nil {
		return Service{}, err
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return Service{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
		}
		return Service{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "workshop_service.updated", "workshop_service", out.ID.String(), map[string]any{"code": out.Code})
	}
	return out, nil
}

func validateService(in *Service) error {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Category = strings.TrimSpace(in.Category)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Currency == "" {
		in.Currency = "ARS"
	}
	if len(in.Code) < 2 {
		return fmt.Errorf("code is required: %w", httperrors.ErrBadInput)
	}
	if len(in.Name) < 2 {
		return fmt.Errorf("name is required: %w", httperrors.ErrBadInput)
	}
	if in.EstimatedHours < 0 || in.BasePrice < 0 || in.TaxRate < 0 {
		return fmt.Errorf("numeric values must be positive: %w", httperrors.ErrBadInput)
	}
	return nil
}

func (u *Usecases) enrichLinkedProduct(ctx context.Context, in *Service) error {
	if in.LinkedProductID == nil || u.cp == nil {
		return nil
	}
	product, err := u.cp.GetProduct(ctx, in.OrgID.String(), in.LinkedProductID.String())
	if err != nil {
		return fmt.Errorf("linked_product_id is invalid: %w", httperrors.ErrBadInput)
	}
	if strings.TrimSpace(in.Description) == "" {
		if description, ok := product["description"].(string); ok && strings.TrimSpace(description) != "" {
			in.Description = strings.TrimSpace(description)
		}
	}
	if in.BasePrice == 0 {
		in.BasePrice = parseMapFloat(product["price"])
	}
	if in.TaxRate == 0 {
		if value, ok := product["tax_rate"]; ok && value != nil {
			in.TaxRate = parseMapFloat(value)
		}
	}
	return nil
}

func parseMapFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}
