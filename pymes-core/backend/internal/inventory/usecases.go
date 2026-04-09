package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	inventorydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/inventory/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	EnsureStockLevel(ctx context.Context, orgID, productID uuid.UUID) error
	GetLevel(ctx context.Context, orgID, productID uuid.UUID) (inventorydomain.StockLevel, error)
	ListLevels(ctx context.Context, p ListStockParams) ([]inventorydomain.StockLevel, int64, bool, *uuid.UUID, error)
	ListMovements(ctx context.Context, p ListMovementParams) ([]inventorydomain.StockMovement, int64, bool, *uuid.UUID, error)
	AdjustAndMove(ctx context.Context, orgID, productID uuid.UUID, delta float64, reason string, referenceID *uuid.UUID, notes, actor string, minQuantity *float64, movementType string) error
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type NotificationPort interface {
	NotifyInventoryAdjusted(ctx context.Context, level inventorydomain.StockLevel, delta float64, actor, notes string) error
}

type Usecases struct {
	repo     RepositoryPort
	audit    AuditPort
	notifier NotificationPort
}

func NewUsecases(repo RepositoryPort, audit AuditPort, notifier NotificationPort) *Usecases {
	return &Usecases{repo: repo, audit: audit, notifier: notifier}
}

func (u *Usecases) EnsureStockLevel(ctx context.Context, orgID, productID uuid.UUID) error {
	return u.repo.EnsureStockLevel(ctx, orgID, productID)
}

func (u *Usecases) List(ctx context.Context, p ListStockParams) ([]inventorydomain.StockLevel, int64, bool, *uuid.UUID, error) {
	return u.repo.ListLevels(ctx, p)
}

func (u *Usecases) GetByProduct(ctx context.Context, orgID, productID uuid.UUID) (inventorydomain.StockLevel, error) {
	out, err := u.repo.GetLevel(ctx, orgID, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return inventorydomain.StockLevel{}, fmt.Errorf("stock not found: %w", httperrors.ErrNotFound)
		}
		return inventorydomain.StockLevel{}, err
	}
	return out, nil
}

func (u *Usecases) AdjustManual(ctx context.Context, orgID, productID uuid.UUID, quantity float64, minQuantity *float64, notes, actor string) (inventorydomain.StockLevel, error) {
	if strings.TrimSpace(notes) == "" {
		return inventorydomain.StockLevel{}, fmt.Errorf("notes required: %w", httperrors.ErrBadInput)
	}
	if err := u.repo.AdjustAndMove(ctx, orgID, productID, quantity, "adjustment", nil, notes, actor, minQuantity, "adjustment"); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return inventorydomain.StockLevel{}, fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
		}
		return inventorydomain.StockLevel{}, err
	}
	out, err := u.repo.GetLevel(ctx, orgID, productID)
	if err != nil {
		return inventorydomain.StockLevel{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "inventory.adjusted", "stock_level", productID.String(), map[string]any{
			"delta":        quantity,
			"min_quantity": out.MinQuantity,
			"quantity":     out.Quantity,
		})
	}
	if u.notifier != nil {
		_ = u.notifier.NotifyInventoryAdjusted(ctx, out, quantity, actor, notes)
	}
	return out, nil
}

func (u *Usecases) ListMovements(ctx context.Context, p ListMovementParams) ([]inventorydomain.StockMovement, int64, bool, *uuid.UUID, error) {
	return u.repo.ListMovements(ctx, p)
}

func (u *Usecases) LowStock(ctx context.Context, orgID uuid.UUID, limit int, after *uuid.UUID) ([]inventorydomain.StockLevel, int64, bool, *uuid.UUID, error) {
	return u.repo.ListLevels(ctx, ListStockParams{OrgID: orgID, Limit: limit, After: after, LowStock: true, Order: "desc"})
}

type SaleItemStock struct {
	ProductID *uuid.UUID
	Quantity  float64
}

func (u *Usecases) ApplySaleItems(ctx context.Context, orgID, saleID uuid.UUID, actor string, items []SaleItemStock) error {
	for _, it := range items {
		if it.ProductID == nil || *it.ProductID == uuid.Nil || it.Quantity == 0 {
			continue
		}
		qty := -abs(it.Quantity)
		if err := u.repo.AdjustAndMove(ctx, orgID, *it.ProductID, qty, "sale", &saleID, "sale stock deduction", actor, nil, "out"); err != nil {
			return err
		}
	}
	return nil
}

func (u *Usecases) ReverseSaleItems(ctx context.Context, orgID, saleID uuid.UUID, actor string, items []SaleItemStock) error {
	for _, it := range items {
		if it.ProductID == nil || *it.ProductID == uuid.Nil || it.Quantity == 0 {
			continue
		}
		qty := abs(it.Quantity)
		if err := u.repo.AdjustAndMove(ctx, orgID, *it.ProductID, qty, "void", &saleID, "sale void stock reversal", actor, nil, "in"); err != nil {
			return err
		}
	}
	return nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
