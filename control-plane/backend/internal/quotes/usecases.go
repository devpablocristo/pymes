package quotes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	quotedomain "github.com/devpablocristo/pymes/control-plane/backend/internal/quotes/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/sales"
	salesdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/sales/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/control-plane/backend/internal/shared/httperrors"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]quotedomain.Quote, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in CreateInput) (quotedomain.Quote, error)
	GetByID(ctx context.Context, orgID, quoteID uuid.UUID) (quotedomain.Quote, error)
	UpdateDraft(ctx context.Context, in UpdateInput) (quotedomain.Quote, error)
	DeleteDraft(ctx context.Context, orgID, quoteID uuid.UUID) error
	SetStatus(ctx context.Context, orgID, quoteID uuid.UUID, status string) (quotedomain.Quote, error)
	GetTenantSettings(ctx context.Context, orgID uuid.UUID) (currency string, taxRate float64, quotePrefix string, err error)
	GetProductSnapshot(ctx context.Context, orgID, productID uuid.UUID) (ProductSnapshot, error)
}

type SalesPort interface {
	Create(ctx context.Context, in sales.CreateSaleInput) (salesdomain.Sale, error)
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	repo  RepositoryPort
	sales SalesPort
	audit AuditPort
}

func NewUsecases(repo RepositoryPort, salesUC SalesPort, audit AuditPort) *Usecases {
	return &Usecases{repo: repo, sales: salesUC, audit: audit}
}

type QuoteItemInput struct {
	ProductID   *uuid.UUID
	Description string
	Quantity    float64
	UnitPrice   float64
	TaxRate     *float64
	SortOrder   int
}

type CreateQuoteInput struct {
	OrgID        uuid.UUID
	CustomerID   *uuid.UUID
	CustomerName string
	Items        []QuoteItemInput
	Notes        string
	ValidUntil   *time.Time
	CreatedBy    string
}

func (u *Usecases) List(ctx context.Context, p ListParams) ([]quotedomain.Quote, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in CreateQuoteInput) (quotedomain.Quote, error) {
	if len(in.Items) == 0 {
		return quotedomain.Quote{}, fmt.Errorf("at least one item is required: %w", httperrors.ErrBadInput)
	}
	currency, defaultTaxRate, _, err := u.repo.GetTenantSettings(ctx, in.OrgID)
	if err != nil {
		return quotedomain.Quote{}, err
	}
	items, subtotal, taxTotal, err := u.buildItems(ctx, in.OrgID, defaultTaxRate, in.Items)
	if err != nil {
		return quotedomain.Quote{}, err
	}
	out, err := u.repo.Create(ctx, CreateInput{
		OrgID:        in.OrgID,
		CustomerID:   in.CustomerID,
		CustomerName: in.CustomerName,
		Subtotal:     subtotal,
		TaxTotal:     taxTotal,
		Total:        subtotal + taxTotal,
		Currency:     currency,
		Notes:        in.Notes,
		ValidUntil:   in.ValidUntil,
		CreatedBy:    in.CreatedBy,
		Items:        items,
	})
	if err != nil {
		return quotedomain.Quote{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.CreatedBy, "quote.created", "quote", out.ID.String(), map[string]any{
			"number": out.Number,
			"total":  out.Total,
		})
	}
	return out, nil
}

type UpdateQuoteInput struct {
	OrgID        uuid.UUID
	ID           uuid.UUID
	CustomerID   **uuid.UUID
	CustomerName *string
	Items        *[]QuoteItemInput
	Notes        *string
	ValidUntil   **time.Time
	Actor        string
}

func (u *Usecases) Update(ctx context.Context, in UpdateQuoteInput) (quotedomain.Quote, error) {
	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return quotedomain.Quote{}, fmt.Errorf("quote not found: %w", httperrors.ErrNotFound)
		}
		return quotedomain.Quote{}, err
	}
	if current.Status != "draft" {
		return quotedomain.Quote{}, fmt.Errorf("only draft quotes can be updated: %w", httperrors.ErrNotDraft)
	}

	customerID := current.CustomerID
	if in.CustomerID != nil {
		customerID = *in.CustomerID
	}
	customerName := current.CustomerName
	if in.CustomerName != nil {
		customerName = strings.TrimSpace(*in.CustomerName)
	}
	notes := current.Notes
	if in.Notes != nil {
		notes = strings.TrimSpace(*in.Notes)
	}
	validUntil := current.ValidUntil
	if in.ValidUntil != nil {
		validUntil = *in.ValidUntil
	}

	itemInputs := make([]QuoteItemInput, 0, len(current.Items))
	if in.Items != nil {
		itemInputs = append(itemInputs, (*in.Items)...)
	} else {
		for _, item := range current.Items {
			t := item.TaxRate
			itemInputs = append(itemInputs, QuoteItemInput{
				ProductID:   item.ProductID,
				Description: item.Description,
				Quantity:    item.Quantity,
				UnitPrice:   item.UnitPrice,
				TaxRate:     &t,
				SortOrder:   item.SortOrder,
			})
		}
	}

	_, defaultTaxRate, _, err := u.repo.GetTenantSettings(ctx, in.OrgID)
	if err != nil {
		return quotedomain.Quote{}, err
	}
	items, subtotal, taxTotal, err := u.buildItems(ctx, in.OrgID, defaultTaxRate, itemInputs)
	if err != nil {
		return quotedomain.Quote{}, err
	}

	out, err := u.repo.UpdateDraft(ctx, UpdateInput{
		OrgID:        in.OrgID,
		ID:           in.ID,
		CustomerID:   customerID,
		CustomerName: customerName,
		Subtotal:     subtotal,
		TaxTotal:     taxTotal,
		Total:        subtotal + taxTotal,
		Currency:     current.Currency,
		Notes:        notes,
		ValidUntil:   validUntil,
		Items:        items,
	})
	if err != nil {
		if errors.Is(err, ErrQuoteNotDraft) {
			return quotedomain.Quote{}, fmt.Errorf("quote is not in draft status: %w", httperrors.ErrNotDraft)
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return quotedomain.Quote{}, fmt.Errorf("quote not found: %w", httperrors.ErrNotFound)
		}
		return quotedomain.Quote{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.Actor, "quote.updated", "quote", in.ID.String(), map[string]any{
			"status": out.Status,
			"total":  out.Total,
		})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, quoteID uuid.UUID) (quotedomain.Quote, error) {
	out, err := u.repo.GetByID(ctx, orgID, quoteID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return quotedomain.Quote{}, fmt.Errorf("quote not found: %w", httperrors.ErrNotFound)
		}
		return quotedomain.Quote{}, err
	}
	return out, nil
}

func (u *Usecases) Delete(ctx context.Context, orgID, quoteID uuid.UUID, actor string) error {
	if err := u.repo.DeleteDraft(ctx, orgID, quoteID); err != nil {
		if errors.Is(err, ErrQuoteNotDraft) {
			return fmt.Errorf("quote is not in draft status: %w", httperrors.ErrNotDraft)
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("quote not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "quote.deleted", "quote", quoteID.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) Send(ctx context.Context, orgID, quoteID uuid.UUID, actor string) (quotedomain.Quote, error) {
	current, err := u.GetByID(ctx, orgID, quoteID)
	if err != nil {
		return quotedomain.Quote{}, err
	}
	if current.Status != "draft" {
		return quotedomain.Quote{}, fmt.Errorf("only draft quotes can be sent: %w", httperrors.ErrNotDraft)
	}
	out, err := u.repo.SetStatus(ctx, orgID, quoteID, "sent")
	if err != nil {
		return quotedomain.Quote{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "quote.sent", "quote", quoteID.String(), map[string]any{})
	}
	return out, nil
}

func (u *Usecases) Accept(ctx context.Context, orgID, quoteID uuid.UUID, actor string) (quotedomain.Quote, error) {
	current, err := u.GetByID(ctx, orgID, quoteID)
	if err != nil {
		return quotedomain.Quote{}, err
	}
	if current.Status == "rejected" || current.Status == "expired" {
		return quotedomain.Quote{}, fmt.Errorf("quote cannot be accepted from current status: %w", httperrors.ErrConflict)
	}
	out, err := u.repo.SetStatus(ctx, orgID, quoteID, "accepted")
	if err != nil {
		return quotedomain.Quote{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "quote.accepted", "quote", quoteID.String(), map[string]any{})
	}
	return out, nil
}

func (u *Usecases) Reject(ctx context.Context, orgID, quoteID uuid.UUID, actor string) (quotedomain.Quote, error) {
	current, err := u.GetByID(ctx, orgID, quoteID)
	if err != nil {
		return quotedomain.Quote{}, err
	}
	if current.Status == "accepted" {
		return quotedomain.Quote{}, fmt.Errorf("accepted quote cannot be rejected: %w", httperrors.ErrConflict)
	}
	out, err := u.repo.SetStatus(ctx, orgID, quoteID, "rejected")
	if err != nil {
		return quotedomain.Quote{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "quote.rejected", "quote", quoteID.String(), map[string]any{})
	}
	return out, nil
}

func (u *Usecases) ToSale(ctx context.Context, orgID, quoteID uuid.UUID, paymentMethod, notes, actor string) (salesdomain.Sale, error) {
	q, err := u.GetByID(ctx, orgID, quoteID)
	if err != nil {
		return salesdomain.Sale{}, err
	}
	if q.Status == "rejected" || q.Status == "expired" {
		return salesdomain.Sale{}, fmt.Errorf("quote cannot be converted to sale from current status: %w", httperrors.ErrConflict)
	}
	if u.sales == nil {
		return salesdomain.Sale{}, fmt.Errorf("sales service unavailable")
	}

	saleItems := make([]sales.CreateSaleItemInput, 0, len(q.Items))
	for _, item := range q.Items {
		t := item.TaxRate
		saleItems = append(saleItems, sales.CreateSaleItemInput{
			ProductID:   item.ProductID,
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TaxRate:     &t,
			SortOrder:   item.SortOrder,
		})
	}

	saleOut, err := u.sales.Create(ctx, sales.CreateSaleInput{
		OrgID:         orgID,
		CustomerID:    q.CustomerID,
		CustomerName:  q.CustomerName,
		QuoteID:       &q.ID,
		PaymentMethod: paymentMethod,
		Items:         saleItems,
		Notes:         notes,
		CreatedBy:     actor,
	})
	if err != nil {
		return salesdomain.Sale{}, err
	}
	if _, err := u.repo.SetStatus(ctx, orgID, quoteID, "accepted"); err != nil {
		return salesdomain.Sale{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "quote.to_sale", "quote", quoteID.String(), map[string]any{
			"sale_id": saleOut.ID.String(),
		})
	}
	return saleOut, nil
}

func (u *Usecases) buildItems(ctx context.Context, orgID uuid.UUID, defaultTaxRate float64, in []QuoteItemInput) ([]CreateItemInput, float64, float64, error) {
	items := make([]CreateItemInput, 0, len(in))
	subtotal := 0.0
	taxTotal := 0.0
	for i, item := range in {
		if item.Quantity <= 0 {
			return nil, 0, 0, fmt.Errorf("item quantity must be > 0: %w", httperrors.ErrBadInput)
		}
		desc := strings.TrimSpace(item.Description)
		unitPrice := item.UnitPrice
		taxRate := defaultTaxRate
		var productID *uuid.UUID

		if item.ProductID != nil && *item.ProductID != uuid.Nil {
			snapshot, err := u.repo.GetProductSnapshot(ctx, orgID, *item.ProductID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, 0, 0, fmt.Errorf("product not found: %w", httperrors.ErrNotFound)
				}
				return nil, 0, 0, err
			}
			pid := snapshot.ID
			productID = &pid
			if desc == "" {
				desc = snapshot.Name
			}
			if unitPrice <= 0 {
				unitPrice = snapshot.Price
			}
			if snapshot.TaxRate != nil {
				taxRate = *snapshot.TaxRate
			}
		}
		if item.TaxRate != nil {
			taxRate = *item.TaxRate
		}
		if desc == "" {
			return nil, 0, 0, fmt.Errorf("item description is required: %w", httperrors.ErrBadInput)
		}
		if unitPrice < 0 {
			return nil, 0, 0, fmt.Errorf("item unit_price must be >= 0: %w", httperrors.ErrBadInput)
		}

		lineSubtotal := item.Quantity * unitPrice
		lineTax := lineSubtotal * taxRate / 100.0
		subtotal += lineSubtotal
		taxTotal += lineTax

		sort := item.SortOrder
		if sort == 0 {
			sort = i + 1
		}
		items = append(items, CreateItemInput{
			ProductID:   productID,
			Description: desc,
			Quantity:    item.Quantity,
			UnitPrice:   unitPrice,
			TaxRate:     taxRate,
			Subtotal:    lineSubtotal,
			SortOrder:   sort,
		})
	}
	return items, subtotal, taxTotal, nil
}
