package workorders

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/control-plane/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/values"
)

type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]WorkOrder, int64, bool, *uuid.UUID, error)
	Create(ctx context.Context, in WorkOrder) (WorkOrder, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (WorkOrder, error)
	Update(ctx context.Context, in WorkOrder) (WorkOrder, error)
	SaveIntegrations(ctx context.Context, orgID, id uuid.UUID, quoteID, saleID *uuid.UUID, status *string) (WorkOrder, error)
}

type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type controlPlanePort interface {
	GetCustomer(ctx context.Context, orgID, customerID string) (map[string]any, error)
	GetParty(ctx context.Context, orgID, partyID string) (map[string]any, error)
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

func (u *Usecases) List(ctx context.Context, p ListParams) ([]WorkOrder, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) Create(ctx context.Context, in WorkOrder, actor string) (WorkOrder, error) {
	if in.OpenedAt.IsZero() {
		in.OpenedAt = time.Now().UTC()
	}
	if strings.TrimSpace(in.CreatedBy) == "" {
		in.CreatedBy = actor
	}
	if strings.TrimSpace(in.Number) == "" {
		in.Number = fmt.Sprintf("OT-%s", time.Now().UTC().Format("20060102-150405"))
	}
	if err := u.enrichReferences(ctx, &in); err != nil {
		return WorkOrder{}, err
	}
	if err := normalizeWorkOrder(&in); err != nil {
		return WorkOrder{}, err
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return WorkOrder{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "work_order.created", "work_order", out.ID.String(), map[string]any{"number": out.Number})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (WorkOrder, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return WorkOrder{}, fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		return WorkOrder{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (WorkOrder, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return WorkOrder{}, fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		return WorkOrder{}, err
	}
	if in.VehicleID != nil {
		parsed, err := uuid.Parse(strings.TrimSpace(*in.VehicleID))
		if err != nil {
			return WorkOrder{}, fmt.Errorf("vehicle_id is invalid: %w", httperrors.ErrBadInput)
		}
		current.VehicleID = parsed
	}
	if in.VehiclePlate != nil {
		current.VehiclePlate = strings.ToUpper(strings.TrimSpace(*in.VehiclePlate))
	}
	if in.CustomerID != nil {
		current.CustomerID = values.ParseOptionalUUID(*in.CustomerID)
	}
	if in.CustomerName != nil {
		current.CustomerName = strings.TrimSpace(*in.CustomerName)
	}
	if in.AppointmentID != nil {
		current.AppointmentID = values.ParseOptionalUUID(*in.AppointmentID)
	}
	if in.Status != nil {
		current.Status = strings.TrimSpace(*in.Status)
	}
	if in.RequestedWork != nil {
		current.RequestedWork = strings.TrimSpace(*in.RequestedWork)
	}
	if in.Diagnosis != nil {
		current.Diagnosis = strings.TrimSpace(*in.Diagnosis)
	}
	if in.Notes != nil {
		current.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.InternalNotes != nil {
		current.InternalNotes = strings.TrimSpace(*in.InternalNotes)
	}
	if in.Currency != nil {
		current.Currency = strings.ToUpper(strings.TrimSpace(*in.Currency))
	}
	if in.PromisedAt != nil {
		current.PromisedAt = in.PromisedAt
	}
	if in.ReadyAt != nil {
		current.ReadyAt = *in.ReadyAt
	}
	if in.DeliveredAt != nil {
		current.DeliveredAt = *in.DeliveredAt
	}
	if in.Items != nil {
		current.Items = *in.Items
	}
	if err := u.enrichReferences(ctx, &current); err != nil {
		return WorkOrder{}, err
	}
	if err := normalizeWorkOrder(&current); err != nil {
		return WorkOrder{}, err
	}
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return WorkOrder{}, fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		return WorkOrder{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "work_order.updated", "work_order", out.ID.String(), map[string]any{"number": out.Number, "status": out.Status})
	}
	return out, nil
}

func (u *Usecases) SaveIntegrations(ctx context.Context, orgID, id uuid.UUID, quoteID, saleID *uuid.UUID, status *string, actor string) (WorkOrder, error) {
	out, err := u.repo.SaveIntegrations(ctx, orgID, id, quoteID, saleID, status)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return WorkOrder{}, fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		return WorkOrder{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "work_order.integration_updated", "work_order", out.ID.String(), map[string]any{"quote_id": quoteID, "sale_id": saleID, "status": status})
	}
	return out, nil
}

func normalizeWorkOrder(in *WorkOrder) error {
	in.Number = strings.ToUpper(strings.TrimSpace(in.Number))
	in.VehiclePlate = strings.ToUpper(strings.TrimSpace(in.VehiclePlate))
	in.CustomerName = strings.TrimSpace(in.CustomerName)
	in.Status = normalizeStatus(in.Status)
	in.RequestedWork = strings.TrimSpace(in.RequestedWork)
	in.Diagnosis = strings.TrimSpace(in.Diagnosis)
	in.Notes = strings.TrimSpace(in.Notes)
	in.InternalNotes = strings.TrimSpace(in.InternalNotes)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Currency == "" {
		in.Currency = "ARS"
	}
	if in.VehicleID == uuid.Nil {
		return fmt.Errorf("vehicle_id is required: %w", httperrors.ErrBadInput)
	}
	if in.Number == "" {
		return fmt.Errorf("number is required: %w", httperrors.ErrBadInput)
	}
	if in.OpenedAt.IsZero() {
		return fmt.Errorf("opened_at is required: %w", httperrors.ErrBadInput)
	}
	if len(in.Items) == 0 {
		return fmt.Errorf("at least one item is required: %w", httperrors.ErrBadInput)
	}

	var subtotalServices float64
	var subtotalParts float64
	var taxTotal float64
	for index := range in.Items {
		item := &in.Items[index]
		item.Description = strings.TrimSpace(item.Description)
		item.ItemType = normalizeItemType(item.ItemType)
		if item.Description == "" {
			return fmt.Errorf("item description is required: %w", httperrors.ErrBadInput)
		}
		if item.Quantity <= 0 || item.UnitPrice < 0 || item.TaxRate < 0 {
			return fmt.Errorf("item values are invalid: %w", httperrors.ErrBadInput)
		}
		lineSubtotal := item.Quantity * item.UnitPrice
		lineTax := lineSubtotal * item.TaxRate / 100
		if item.ItemType == "service" {
			subtotalServices += lineSubtotal
		} else {
			subtotalParts += lineSubtotal
		}
		taxTotal += lineTax
		item.SortOrder = index
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
	}
	in.SubtotalServices = subtotalServices
	in.SubtotalParts = subtotalParts
	in.TaxTotal = taxTotal
	in.Total = subtotalServices + subtotalParts + taxTotal
	return nil
}

func (u *Usecases) enrichReferences(ctx context.Context, in *WorkOrder) error {
	if u.cp == nil {
		return nil
	}
	if in.CustomerID != nil {
		customer, err := u.cp.GetCustomer(ctx, in.OrgID.String(), in.CustomerID.String())
		if err == nil {
			if strings.TrimSpace(in.CustomerName) == "" {
				if name, ok := customer["name"].(string); ok {
					in.CustomerName = strings.TrimSpace(name)
				}
			}
		} else {
			party, partyErr := u.cp.GetParty(ctx, in.OrgID.String(), in.CustomerID.String())
			if partyErr != nil {
				return fmt.Errorf("customer_id is invalid: %w", httperrors.ErrBadInput)
			}
			if strings.TrimSpace(in.CustomerName) == "" {
				if displayName, ok := party["display_name"].(string); ok {
					in.CustomerName = strings.TrimSpace(displayName)
				}
			}
		}
	}
	for index := range in.Items {
		item := &in.Items[index]
		if item.ProductID == nil {
			continue
		}
		product, err := u.cp.GetProduct(ctx, in.OrgID.String(), item.ProductID.String())
		if err != nil {
			return fmt.Errorf("product_id is invalid: %w", httperrors.ErrBadInput)
		}
		if strings.TrimSpace(item.Description) == "" {
			if name, ok := product["name"].(string); ok {
				item.Description = strings.TrimSpace(name)
			}
		}
		if item.UnitPrice == 0 {
			item.UnitPrice = parseMapFloat(product["price"])
		}
	}
	return nil
}

func normalizeStatus(raw string) string {
	switch strings.TrimSpace(raw) {
	case "diagnosis", "in_progress", "ready", "delivered", "invoiced", "cancelled":
		return strings.TrimSpace(raw)
	default:
		return "received"
	}
}

func normalizeItemType(raw string) string {
	switch strings.TrimSpace(raw) {
	case "part":
		return "part"
	default:
		return "service"
	}
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
