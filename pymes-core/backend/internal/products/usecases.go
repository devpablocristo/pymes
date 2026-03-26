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
	ListArchived(ctx context.Context, orgID uuid.UUID) ([]productdomain.Product, error)
	Create(ctx context.Context, in productdomain.Product) (productdomain.Product, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (productdomain.Product, error)
	Update(ctx context.Context, in productdomain.Product) (productdomain.Product, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
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
	if in.Type == "" {
		in.Type = "product"
	}
	if in.Type != "product" && in.Type != "service" {
		return productdomain.Product{}, fmt.Errorf("invalid type: %w", httperrors.ErrBadInput)
	}
	if in.Type == "service" {
		in.TrackStock = false
	}
	if in.Unit == "" {
		in.Unit = "unit"
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return productdomain.Product{}, err
	}
	if out.TrackStock && u.inventory != nil {
		_ = u.inventory.EnsureStockLevel(ctx, out.OrgID, out.ID)
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "product.created", "product", out.ID.String(), map[string]any{"name": out.Name, "type": out.Type})
	}
	return out, nil
}

type UpdateInput struct {
	Type        *string
	SKU         *string
	Name        *string
	Description *string
	Unit        *string
	Price       *float64
	CostPrice   *float64
	TaxRate     *float64
	TrackStock  *bool
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
	if in.Type != nil {
		current.Type = strings.TrimSpace(*in.Type)
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
	if in.CostPrice != nil {
		current.CostPrice = *in.CostPrice
	}
	if in.TaxRate != nil {
		v := *in.TaxRate
		current.TaxRate = &v
	}
	if in.TrackStock != nil {
		current.TrackStock = *in.TrackStock
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
	if current.Type != "product" && current.Type != "service" {
		return productdomain.Product{}, fmt.Errorf("invalid type: %w", httperrors.ErrBadInput)
	}
	if current.Type == "service" {
		current.TrackStock = false
	}

	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return productdomain.Product{}, fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		return productdomain.Product{}, err
	}
	if out.TrackStock && u.inventory != nil {
		_ = u.inventory.EnsureStockLevel(ctx, out.OrgID, out.ID)
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "product.updated", "product", out.ID.String(), map[string]any{"name": out.Name, "type": out.Type})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (productdomain.Product, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return productdomain.Product{}, fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		return productdomain.Product{}, err
	}
	return out, nil
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "product.deleted", "product", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID) ([]productdomain.Product, error) {
	return u.repo.ListArchived(ctx, orgID)
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "product.restored", "product", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "product.hard_deleted", "product", id.String(), map[string]any{})
	}
	return nil
}
