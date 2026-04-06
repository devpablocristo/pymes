package sales

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/inventory"
	saledomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/sales/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]saledomain.Sale, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in CreateInput) (saledomain.Sale, error)
	GetByID(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error)
	Void(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error)
	GetTenantSettings(ctx context.Context, orgID uuid.UUID) (currency string, taxRate float64, salePrefix string, err error)
	GetProductSnapshot(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error)
	GetServiceSnapshot(ctx context.Context, orgID, serviceID uuid.UUID) (ServiceSnapshot, error)
}

type InventoryPort interface {
	ApplySaleItems(ctx context.Context, orgID, saleID uuid.UUID, actor string, items []inventory.SaleItemStock) error
	ReverseSaleItems(ctx context.Context, orgID, saleID uuid.UUID, actor string, items []inventory.SaleItemStock) error
}

type CashflowPort interface {
	RecordSaleIncome(ctx context.Context, orgID, saleID uuid.UUID, amount float64, currency, paymentMethod, actor string) error
	RecordSaleVoidExpense(ctx context.Context, orgID, saleID uuid.UUID, amount float64, currency, paymentMethod, actor string) error
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type TimelinePort interface {
	RecordEvent(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID, eventType, title, description, actor string, metadata map[string]any) error
}

type WebhookPort interface {
	Enqueue(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) error
}

type Usecases struct {
	repo      RepositoryPort
	inventory InventoryPort
	cashflow  CashflowPort
	audit     AuditPort
	timeline  TimelinePort
	webhooks  WebhookPort
}

type Option func(*Usecases)

func WithTimeline(t TimelinePort) Option { return func(u *Usecases) { u.timeline = t } }
func WithWebhooks(w WebhookPort) Option  { return func(u *Usecases) { u.webhooks = w } }

func NewUsecases(repo RepositoryPort, inventory InventoryPort, cashflow CashflowPort, audit AuditPort, opts ...Option) *Usecases {
	uc := &Usecases{repo: repo, inventory: inventory, cashflow: cashflow, audit: audit}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

type CreateSaleItemInput struct {
	ProductID   *uuid.UUID
	ServiceID   *uuid.UUID
	Description string
	Quantity    float64
	UnitPrice   float64
	TaxRate     *float64
	SortOrder   int
}

type CreateSaleInput struct {
	OrgID         uuid.UUID
	CustomerID    *uuid.UUID
	CustomerName  string
	QuoteID       *uuid.UUID
	PaymentMethod string
	Items         []CreateSaleItemInput
	Notes         string
	CreatedBy     string
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]saledomain.Sale, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in CreateSaleInput) (saledomain.Sale, error) {
	if len(in.Items) == 0 {
		return saledomain.Sale{}, fmt.Errorf("at least one item is required: %w", httperrors.ErrBadInput)
	}
	if !isValidPaymentMethod(in.PaymentMethod) {
		return saledomain.Sale{}, fmt.Errorf("invalid payment_method: %w", httperrors.ErrBadInput)
	}

	currency, defaultTaxRate, _, err := u.repo.GetTenantSettings(ctx, in.OrgID)
	if err != nil {
		return saledomain.Sale{}, err
	}

	createItems := make([]CreateItemInput, 0, len(in.Items))
	stockItems := make([]inventory.SaleItemStock, 0, len(in.Items))
	subtotal := 0.0
	taxTotal := 0.0

	for i, item := range in.Items {
		if item.Quantity <= 0 {
			return saledomain.Sale{}, fmt.Errorf("item quantity must be > 0: %w", httperrors.ErrBadInput)
		}

		desc := strings.TrimSpace(item.Description)
		unitPrice := item.UnitPrice
		costPrice := 0.0
		taxRate := defaultTaxRate
		var pid *uuid.UUID
		var sid *uuid.UUID

		if item.ProductID != nil && *item.ProductID != uuid.Nil {
			snapshot, err := u.repo.GetProductSnapshot(ctx, in.OrgID, *item.ProductID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return saledomain.Sale{}, fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
				}
				return saledomain.Sale{}, err
			}
			id := snapshot.ID
			pid = &id
			if desc == "" {
				desc = snapshot.Name
			}
			if unitPrice <= 0 {
				unitPrice = snapshot.Price
			}
			costPrice = snapshot.CostPrice
			if snapshot.TaxRate != nil {
				taxRate = *snapshot.TaxRate
			}
		} else if item.ServiceID != nil && *item.ServiceID != uuid.Nil {
			snapshot, err := u.repo.GetServiceSnapshot(ctx, in.OrgID, *item.ServiceID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return saledomain.Sale{}, fmt.Errorf("service not found: %w", httperrors.ErrNotFound)
				}
				return saledomain.Sale{}, err
			}
			id := snapshot.ID
			sid = &id
			if desc == "" {
				desc = snapshot.Name
			}
			if unitPrice <= 0 {
				unitPrice = snapshot.Price
			}
			costPrice = snapshot.CostPrice
			if snapshot.TaxRate != nil {
				taxRate = *snapshot.TaxRate
			}
		}

		if item.TaxRate != nil {
			taxRate = *item.TaxRate
		}
		if strings.TrimSpace(desc) == "" {
			return saledomain.Sale{}, fmt.Errorf("item description is required: %w", httperrors.ErrBadInput)
		}
		if unitPrice < 0 {
			return saledomain.Sale{}, fmt.Errorf("item unit_price must be >= 0: %w", httperrors.ErrBadInput)
		}

		lineSubtotal := item.Quantity * unitPrice
		lineTax := lineSubtotal * taxRate / 100.0
		subtotal += lineSubtotal
		taxTotal += lineTax

		createItems = append(createItems, CreateItemInput{
			ProductID:   pid,
			ServiceID:   sid,
			Description: desc,
			Quantity:    item.Quantity,
			UnitPrice:   unitPrice,
			CostPrice:   costPrice,
			TaxRate:     taxRate,
			Subtotal:    lineSubtotal,
			SortOrder:   item.SortOrder,
		})
		stockItems = append(stockItems, inventory.SaleItemStock{
			ProductID: pid,
			Quantity:  item.Quantity,
		})

		if item.SortOrder == 0 {
			createItems[len(createItems)-1].SortOrder = i + 1
		}
	}

	total := subtotal + taxTotal
	out, err := u.repo.Create(ctx, CreateInput{
		OrgID:         in.OrgID,
		CustomerID:    in.CustomerID,
		CustomerName:  strings.TrimSpace(in.CustomerName),
		QuoteID:       in.QuoteID,
		PaymentMethod: in.PaymentMethod,
		Subtotal:      subtotal,
		TaxTotal:      taxTotal,
		Total:         total,
		Currency:      currency,
		Notes:         strings.TrimSpace(in.Notes),
		CreatedBy:     strings.TrimSpace(in.CreatedBy),
		Items:         createItems,
	})
	if err != nil {
		return saledomain.Sale{}, err
	}

	if u.inventory != nil {
		if err := u.inventory.ApplySaleItems(ctx, in.OrgID, out.ID, in.CreatedBy, stockItems); err != nil {
			return saledomain.Sale{}, err
		}
	}
	if u.cashflow != nil {
		if err := u.cashflow.RecordSaleIncome(ctx, in.OrgID, out.ID, out.Total, out.Currency, out.PaymentMethod, in.CreatedBy); err != nil {
			return saledomain.Sale{}, err
		}
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.CreatedBy, "sale.created", "sale", out.ID.String(), map[string]any{
			"number": out.Number,
			"total":  out.Total,
		})
	}
	if u.timeline != nil && out.CustomerID != nil {
		_ = u.timeline.RecordEvent(ctx, in.OrgID, "parties", *out.CustomerID, "sale.created", "Venta registrada", out.Number, in.CreatedBy, map[string]any{"sale_id": out.ID.String(), "total": out.Total})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, in.OrgID, "sale.created", map[string]any{"sale_id": out.ID.String(), "customer_id": nullableUUID(out.CustomerID), "total": out.Total, "payment_method": out.PaymentMethod})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, saleID uuid.UUID) (saledomain.Sale, error) {
	out, err := u.repo.GetByID(ctx, orgID, saleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return saledomain.Sale{}, fmt.Errorf("sale not found: %w", httperrors.ErrNotFound)
		}
		return saledomain.Sale{}, err
	}
	return out, nil
}

func (u *Usecases) Void(ctx context.Context, orgID, saleID uuid.UUID, actor string) (saledomain.Sale, error) {
	current, err := u.repo.GetByID(ctx, orgID, saleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return saledomain.Sale{}, fmt.Errorf("sale not found: %w", httperrors.ErrNotFound)
		}
		return saledomain.Sale{}, err
	}
	if current.Status == "voided" {
		return current, nil
	}

	out, err := u.repo.Void(ctx, orgID, saleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return saledomain.Sale{}, fmt.Errorf("sale not found: %w", httperrors.ErrNotFound)
		}
		return saledomain.Sale{}, err
	}

	stockItems := make([]inventory.SaleItemStock, 0, len(current.Items))
	for _, item := range current.Items {
		stockItems = append(stockItems, inventory.SaleItemStock{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	if u.inventory != nil {
		if err := u.inventory.ReverseSaleItems(ctx, orgID, saleID, actor, stockItems); err != nil {
			return saledomain.Sale{}, err
		}
	}
	if u.cashflow != nil {
		if err := u.cashflow.RecordSaleVoidExpense(ctx, orgID, saleID, current.Total, current.Currency, current.PaymentMethod, actor); err != nil {
			return saledomain.Sale{}, err
		}
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "sale.voided", "sale", saleID.String(), map[string]any{
			"number": current.Number,
			"total":  current.Total,
		})
	}
	if u.timeline != nil && current.CustomerID != nil {
		_ = u.timeline.RecordEvent(ctx, orgID, "parties", *current.CustomerID, "sale.voided", "Venta anulada", current.Number, actor, map[string]any{"sale_id": saleID.String(), "total": current.Total})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, orgID, "sale.voided", map[string]any{"sale_id": saleID.String(), "customer_id": nullableUUID(current.CustomerID), "total": current.Total})
	}
	return out, nil
}

func isValidPaymentMethod(v string) bool {
	switch strings.TrimSpace(v) {
	case "", "cash", "card", "transfer", "other":
		return true
	default:
		return false
	}
}

func nullableUUID(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
