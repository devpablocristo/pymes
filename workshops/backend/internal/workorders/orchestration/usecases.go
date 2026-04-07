package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

type controlPlanePort interface {
	CreateBooking(ctx context.Context, payload map[string]any) (map[string]any, error)
	CreateQuote(ctx context.Context, payload map[string]any) (map[string]any, error)
	CreateSale(ctx context.Context, payload map[string]any) (map[string]any, error)
	CreateSalePaymentLink(ctx context.Context, orgID, saleID string) (map[string]any, error)
}

type workOrderPort interface {
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.WorkOrder, error)
	SaveIntegrations(ctx context.Context, orgID, id uuid.UUID, quoteID, saleID *uuid.UUID, status *string, actor string) (domain.WorkOrder, error)
}

type auditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	cp     controlPlanePort
	orders workOrderPort
	audit  auditPort
}

func NewUsecases(cp controlPlanePort, orders workOrderPort, audit auditPort) *Usecases {
	return &Usecases{cp: cp, orders: orders, audit: audit}
}

func (u *Usecases) CreateBooking(ctx context.Context, orgID string, payload map[string]any) (map[string]any, error) {
	if strings.TrimSpace(orgID) == "" {
		return nil, fmt.Errorf("org_id is required: %w", httperrors.ErrBadInput)
	}
	out := copyMap(payload)
	out["org_id"] = orgID
	return u.cp.CreateBooking(ctx, out)
}

func (u *Usecases) CreateQuoteFromWorkOrder(ctx context.Context, orgID string, workOrderID uuid.UUID, actor string) (map[string]any, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return nil, fmt.Errorf("org_id is invalid: %w", httperrors.ErrBadInput)
	}
	order, err := u.orders.GetByID(ctx, orgUUID, workOrderID)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"org_id":        orgID,
		"customer_name": fallback(order.CustomerName, order.TargetLabel),
		"notes":         order.Notes,
		"items":         toCommercialItems(order.Items),
	}
	if order.CustomerID != nil {
		payload["customer_id"] = order.CustomerID.String()
	}
	result, err := u.cp.CreateQuote(ctx, payload)
	if err != nil {
		return nil, err
	}
	if quoteID := parseResultID(result["id"]); quoteID != nil {
		_, _ = u.orders.SaveIntegrations(ctx, orgUUID, workOrderID, quoteID, nil, nil, actor)
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID, actor, "work_order.quote_created", "work_order", workOrderID.String(), map[string]any{"quote": result})
	}
	return result, nil
}

func (u *Usecases) CreateSaleFromWorkOrder(ctx context.Context, orgID string, workOrderID uuid.UUID, actor string) (map[string]any, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return nil, fmt.Errorf("org_id is invalid: %w", httperrors.ErrBadInput)
	}
	order, err := u.orders.GetByID(ctx, orgUUID, workOrderID)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"org_id":         orgID,
		"customer_name":  fallback(order.CustomerName, order.TargetLabel),
		"payment_method": "transfer",
		"notes":          order.Notes,
		"items":          toCommercialItems(order.Items),
	}
	if order.CustomerID != nil {
		payload["customer_id"] = order.CustomerID.String()
	}
	result, err := u.cp.CreateSale(ctx, payload)
	if err != nil {
		return nil, err
	}
	var saleID *uuid.UUID
	if parsed := parseResultID(result["id"]); parsed != nil {
		saleID = parsed
		status := "invoiced"
		_, _ = u.orders.SaveIntegrations(ctx, orgUUID, workOrderID, nil, saleID, &status, actor)
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID, actor, "work_order.sale_created", "work_order", workOrderID.String(), map[string]any{"sale": result})
	}
	return result, nil
}

func (u *Usecases) CreatePaymentLinkFromWorkOrder(ctx context.Context, orgID string, workOrderID uuid.UUID, actor string) (map[string]any, error) {
	orgUUID, err := uuid.Parse(strings.TrimSpace(orgID))
	if err != nil {
		return nil, fmt.Errorf("org_id is invalid: %w", httperrors.ErrBadInput)
	}
	order, err := u.orders.GetByID(ctx, orgUUID, workOrderID)
	if err != nil {
		return nil, err
	}
	saleID := order.SaleID
	if saleID == nil {
		saleResult, err := u.CreateSaleFromWorkOrder(ctx, orgID, workOrderID, actor)
		if err != nil {
			return nil, err
		}
		saleID = parseResultID(saleResult["id"])
	}
	if saleID == nil {
		return nil, fmt.Errorf("sale_id is required: %w", httperrors.ErrBadInput)
	}
	result, err := u.cp.CreateSalePaymentLink(ctx, orgID, saleID.String())
	if err != nil {
		return nil, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID, actor, "work_order.payment_link_created", "work_order", workOrderID.String(), map[string]any{"payment_link": result})
	}
	return result, nil
}

func toCommercialItems(items []domain.WorkOrderItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for index, item := range items {
		row := map[string]any{
			"description": item.Description,
			"quantity":    item.Quantity,
			"unit_price":  item.UnitPrice,
			"tax_rate":    item.TaxRate,
			"sort_order":  index,
		}
		if item.ProductID != nil {
			row["product_id"] = item.ProductID.String()
		}
		out = append(out, row)
	}
	return out
}

func parseResultID(value any) *uuid.UUID {
	raw, ok := value.(string)
	if !ok {
		return nil
	}
	parsed, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil
	}
	return &parsed
}

func fallback(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "Orden de taller"
}

func copyMap(payload map[string]any) map[string]any {
	out := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		out[key] = value
	}
	return out
}
