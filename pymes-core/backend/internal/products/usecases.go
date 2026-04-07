package products

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	productdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/products/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]productdomain.Product, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in productdomain.Product) (productdomain.Product, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (productdomain.Product, error)
	Update(ctx context.Context, in productdomain.Product) (productdomain.Product, error)
	Archive(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type InventoryPort interface {
	EnsureStockLevel(ctx context.Context, orgID, productID uuid.UUID) error
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	repo      RepositoryPort
	inventory InventoryPort
	audit     AuditPort
}

func NewUsecases(repo RepositoryPort, inventory InventoryPort, audit AuditPort) *Usecases {
	return &Usecases{repo: repo, inventory: inventory, audit: audit}
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]productdomain.Product, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in productdomain.Product, actor string) (productdomain.Product, error) {
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 2 {
		return productdomain.Product{}, fmt.Errorf("name must be at least 2 characters: %w", httperrors.ErrBadInput)
	}
	in.ImageURL = strings.TrimSpace(in.ImageURL)
	if len(in.ImageURL) > 2048 {
		return productdomain.Product{}, fmt.Errorf("image_url too long: %w", httperrors.ErrBadInput)
	}
	if in.Unit == "" {
		in.Unit = "unit"
	}
	if strings.TrimSpace(in.Currency) == "" {
		in.Currency = "ARS"
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return productdomain.Product{}, fmt.Errorf("product already exists: %w", httperrors.ErrConflict)
		}
		return productdomain.Product{}, err
	}
	if out.TrackStock && u.inventory != nil {
		_ = u.inventory.EnsureStockLevel(ctx, out.OrgID, out.ID)
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "product.created", "product", out.ID.String(), map[string]any{"name": out.Name})
	}
	return out, nil
}

type UpdateInput struct {
	SKU         *string
	Name        *string
	Description *string
	Unit        *string
	Price       *float64
	Currency    *string
	CostPrice   *float64
	TaxRate     *float64
	ImageURL    *string
	TrackStock  *bool
	IsActive    *bool
	Tags        *[]string
	Metadata    *map[string]any
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (productdomain.Product, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return productdomain.Product{}, fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		return productdomain.Product{}, err
	}
	if in.SKU != nil {
		current.SKU = strings.TrimSpace(*in.SKU)
	}
	if in.Name != nil {
		current.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		current.Description = strings.TrimSpace(*in.Description)
	}
	if in.Unit != nil {
		current.Unit = strings.TrimSpace(*in.Unit)
	}
	if in.Price != nil {
		current.Price = *in.Price
	}
	if in.Currency != nil {
		current.Currency = strings.TrimSpace(*in.Currency)
	}
	if in.CostPrice != nil {
		current.CostPrice = *in.CostPrice
	}
	if in.TaxRate != nil {
		v := *in.TaxRate
		current.TaxRate = &v
	}
	if in.ImageURL != nil {
		current.ImageURL = strings.TrimSpace(*in.ImageURL)
	}
	if in.TrackStock != nil {
		current.TrackStock = *in.TrackStock
	}
	if in.IsActive != nil {
		current.IsActive = *in.IsActive
	}
	if in.Tags != nil {
		current.Tags = append([]string(nil), (*in.Tags)...)
	}
	if in.Metadata != nil {
		current.Metadata = *in.Metadata
	}

	if len(current.Name) < 2 {
		return productdomain.Product{}, fmt.Errorf("name must be at least 2 characters: %w", httperrors.ErrBadInput)
	}
	if len(current.ImageURL) > 2048 {
		return productdomain.Product{}, fmt.Errorf("image_url too long: %w", httperrors.ErrBadInput)
	}
	if strings.TrimSpace(current.Currency) == "" {
		current.Currency = "ARS"
	}

	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return productdomain.Product{}, fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		if errors.Is(err, ErrAlreadyExists) {
			return productdomain.Product{}, fmt.Errorf("product already exists: %w", httperrors.ErrConflict)
		}
		return productdomain.Product{}, err
	}
	if out.TrackStock && u.inventory != nil {
		_ = u.inventory.EnsureStockLevel(ctx, out.OrgID, out.ID)
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "product.updated", "product", out.ID.String(), map[string]any{"name": out.Name})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (productdomain.Product, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return productdomain.Product{}, fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		return productdomain.Product{}, err
	}
	return out, nil
}

func (u *Usecases) Archive(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Archive(ctx, orgID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "product.archived", "product", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "product.restored", "product", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) Delete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Delete(ctx, orgID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "product.deleted", "product", id.String(), map[string]any{})
	}
	return nil
}
