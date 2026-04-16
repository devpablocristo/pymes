package workorders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/vertvalues"
	"github.com/devpablocristo/pymes/workshops/backend/internal/shared/workshops"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

// ListParams agrupa filtros de listado.
// TargetType opcional permite a una vertical pedir solo "vehicle" o solo "bicycle".
type ListParams struct {
	OrgID      uuid.UUID
	BranchID   *uuid.UUID
	Limit      int
	After      *uuid.UUID
	Search     string
	Status     string
	TargetType string
}

// UpdateInput agrupa los campos parcheables. TargetID/TargetLabel pueden cambiar
// si se reasigna la OT a otro asset (mover a otro vehículo/bici).
type UpdateInput struct {
	BranchID      *string
	TargetID      *string
	TargetLabel   *string
	CustomerID    *string
	CustomerName  *string
	BookingID     *string
	Status        *string
	RequestedWork *string
	Diagnosis     *string
	Notes         *string
	InternalNotes *string
	Currency      *string
	PromisedAt    *time.Time
	ReadyAt       **time.Time
	DeliveredAt   **time.Time
	Items         *[]domain.WorkOrderItem
}

// RepositoryPort define el contrato del adapter de persistencia.
type RepositoryPort interface {
	List(ctx context.Context, p ListParams) ([]domain.WorkOrder, int64, bool, *uuid.UUID, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, targetType string) ([]domain.WorkOrder, error)
	Create(ctx context.Context, in domain.WorkOrder) (domain.WorkOrder, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.WorkOrder, error)
	Update(ctx context.Context, in domain.WorkOrder) (domain.WorkOrder, error)
	SaveIntegrations(ctx context.Context, orgID, id uuid.UUID, quoteID, saleID *uuid.UUID, status *string) (domain.WorkOrder, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
}

// AuditPort registra eventos de dominio (delegado al servicio compartido).
type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

// controlPlanePort consulta el catálogo del core (productos, parties).
type controlPlanePort interface {
	GetCustomer(ctx context.Context, orgID, customerID string) (map[string]any, error)
	GetParty(ctx context.Context, orgID, partyID string) (map[string]any, error)
	GetProduct(ctx context.Context, orgID, productID string) (map[string]any, error)
}

// whatsAppNotifier envía texto vía API interna de pymes-core (opt-in y reglas en el core).
type whatsAppNotifier interface {
	SendInternalWhatsAppText(ctx context.Context, orgID string, partyID uuid.UUID, body string) error
}

// Usecases es el motor base de work orders. Coordina repo, audit, core, whatsapp y hooks.
type Usecases struct {
	repo     RepositoryPort
	audit    AuditPort
	cp       controlPlanePort
	whatsapp whatsAppNotifier
	hooks    *hookRegistry
}

// NewUsecases construye el motor base. Las hooks de cada vertical se pasan al final.
func NewUsecases(repo RepositoryPort, audit AuditPort, cp controlPlanePort, wa whatsAppNotifier, hooks ...Hook) *Usecases {
	return &Usecases{
		repo:     repo,
		audit:    audit,
		cp:       cp,
		whatsapp: wa,
		hooks:    newHookRegistry(hooks),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Read
// ─────────────────────────────────────────────────────────────────────────────

func (u *Usecases) List(ctx context.Context, p ListParams) ([]domain.WorkOrder, int64, bool, *uuid.UUID, error) {
	return u.repo.List(ctx, p)
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, targetType string) ([]domain.WorkOrder, error) {
	return u.repo.ListArchived(ctx, orgID, branchID, targetType)
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.WorkOrder, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.WorkOrder{}, fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		return domain.WorkOrder{}, err
	}
	return out, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Create
// ─────────────────────────────────────────────────────────────────────────────

func (u *Usecases) Create(ctx context.Context, in domain.WorkOrder, actor string) (domain.WorkOrder, error) {
	if in.OpenedAt.IsZero() {
		in.OpenedAt = time.Now().UTC()
	}
	if strings.TrimSpace(in.CreatedBy) == "" {
		in.CreatedBy = actor
	}
	if strings.TrimSpace(in.Number) == "" {
		in.Number = fmt.Sprintf("OT-%s", time.Now().UTC().Format("20060102-150405"))
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}

	hook := u.hooks.lookup(in.TargetType)
	if err := hook.BeforeCreate(ctx, &in); err != nil {
		return domain.WorkOrder{}, err
	}
	if err := u.enrichReferences(ctx, &in); err != nil {
		return domain.WorkOrder{}, err
	}
	if err := normalizeWorkOrder(&in); err != nil {
		return domain.WorkOrder{}, err
	}
	out, err := u.repo.Create(ctx, in)
	if err != nil {
		return domain.WorkOrder{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "work_order.created", "work_order", out.ID.String(), map[string]any{
			"number":      out.Number,
			"target_type": out.TargetType,
		})
	}
	return out, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Update
// ─────────────────────────────────────────────────────────────────────────────

func (u *Usecases) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, actor string) (domain.WorkOrder, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.WorkOrder{}, fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		return domain.WorkOrder{}, err
	}
	if current.ArchivedAt != nil {
		return domain.WorkOrder{}, fmt.Errorf("work order is archived: %w", httperrors.ErrBadInput)
	}

	prevCanon := normalizeWorkOrderStatus(current.Status)
	next := current

	if in.TargetID != nil {
		parsed, err := uuid.Parse(strings.TrimSpace(*in.TargetID))
		if err != nil {
			return domain.WorkOrder{}, fmt.Errorf("target_id is invalid: %w", httperrors.ErrBadInput)
		}
		next.TargetID = parsed
	}
	if in.BranchID != nil {
		next.BranchID = vertvalues.ParseOptionalUUID(*in.BranchID)
	}
	if in.TargetLabel != nil {
		next.TargetLabel = strings.TrimSpace(*in.TargetLabel)
	}
	if in.CustomerID != nil {
		next.CustomerID = vertvalues.ParseOptionalUUID(*in.CustomerID)
	}
	if in.CustomerName != nil {
		next.CustomerName = strings.TrimSpace(*in.CustomerName)
	}
	if in.BookingID != nil {
		next.BookingID = vertvalues.ParseOptionalUUID(*in.BookingID)
	}
	if in.Status != nil {
		nextRaw := strings.TrimSpace(*in.Status)
		if err := workshops.ValidateStatusTransition(next.Status, nextRaw); err != nil {
			return domain.WorkOrder{}, err
		}
		next.Status = nextRaw
	}
	if in.RequestedWork != nil {
		next.RequestedWork = strings.TrimSpace(*in.RequestedWork)
	}
	if in.Diagnosis != nil {
		next.Diagnosis = strings.TrimSpace(*in.Diagnosis)
	}
	if in.Notes != nil {
		next.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.InternalNotes != nil {
		next.InternalNotes = strings.TrimSpace(*in.InternalNotes)
	}
	if in.Currency != nil {
		next.Currency = strings.ToUpper(strings.TrimSpace(*in.Currency))
	}
	if in.PromisedAt != nil {
		next.PromisedAt = in.PromisedAt
	}
	if in.ReadyAt != nil {
		next.ReadyAt = *in.ReadyAt
	}
	if in.DeliveredAt != nil {
		next.DeliveredAt = *in.DeliveredAt
	}
	if in.Items != nil {
		next.Items = *in.Items
	}

	hook := u.hooks.lookup(next.TargetType)
	if err := hook.BeforeUpdate(ctx, &current, &next); err != nil {
		return domain.WorkOrder{}, err
	}
	if err := u.enrichReferences(ctx, &next); err != nil {
		return domain.WorkOrder{}, err
	}
	if err := normalizeWorkOrder(&next); err != nil {
		return domain.WorkOrder{}, err
	}
	nextCanon := normalizeWorkOrderStatus(next.Status)
	if nextCanon == "ready_for_pickup" && prevCanon != "ready_for_pickup" && next.ReadyAt == nil {
		t := time.Now().UTC()
		next.ReadyAt = &t
	}
	if nextCanon == "delivered" && prevCanon != "delivered" && next.DeliveredAt == nil {
		t := time.Now().UTC()
		next.DeliveredAt = &t
	}
	out, err := u.repo.Update(ctx, next)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.WorkOrder{}, fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		return domain.WorkOrder{}, err
	}
	if normalizeWorkOrderStatus(out.Status) != prevCanon {
		hook.AfterStatusChange(ctx, &out, prevCanon)
	}
	u.maybeNotifyReadyForPickup(ctx, orgID, actor, prevCanon, &out, hook)
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "work_order.updated", "work_order", out.ID.String(), map[string]any{
			"number":      out.Number,
			"status":      out.Status,
			"target_type": out.TargetType,
		})
	}
	return out, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Integrations (quote/sale linked from core)
// ─────────────────────────────────────────────────────────────────────────────

func (u *Usecases) SaveIntegrations(ctx context.Context, orgID, id uuid.UUID, quoteID, saleID *uuid.UUID, status *string, actor string) (domain.WorkOrder, error) {
	current, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.WorkOrder{}, fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		return domain.WorkOrder{}, err
	}
	if current.ArchivedAt != nil {
		return domain.WorkOrder{}, fmt.Errorf("work order is archived: %w", httperrors.ErrBadInput)
	}
	out, err := u.repo.SaveIntegrations(ctx, orgID, id, quoteID, saleID, status)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.WorkOrder{}, fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		return domain.WorkOrder{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "work_order.integration_updated", "work_order", out.ID.String(), map[string]any{
			"quote_id": quoteID, "sale_id": saleID, "status": status,
		})
	}
	return out, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Lifecycle (archive / restore / hard delete)
// ─────────────────────────────────────────────────────────────────────────────

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "work_order.archived", "work_order", id.String(), nil)
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "work_order.restored", "work_order", id.String(), nil)
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("work order not found: %w", httperrors.ErrNotFound)
		}
		if errors.Is(err, ErrWorkOrderHasIntegrations) {
			return fmt.Errorf("work order has quote or sale: %w", httperrors.ErrConflict)
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "work_order.hard_deleted", "work_order", id.String(), nil)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Internals
// ─────────────────────────────────────────────────────────────────────────────

// normalizeWorkOrderStatus delega a la FSM compartida (auto_repair y bike_shop usan los mismos estados).
func normalizeWorkOrderStatus(raw string) string {
	return workshops.NormalizeStatus(raw)
}

func (u *Usecases) maybeNotifyReadyForPickup(ctx context.Context, orgID uuid.UUID, actor, prevCanon string, out *domain.WorkOrder, hook Hook) {
	if u.whatsapp == nil || out == nil {
		return
	}
	if normalizeWorkOrderStatus(out.Status) != "ready_for_pickup" {
		return
	}
	if prevCanon == "ready_for_pickup" {
		return
	}
	if out.CustomerID == nil {
		return
	}
	if out.ReadyPickupNotifiedAt != nil {
		return
	}
	msg := strings.TrimSpace(hook.ReadyForPickupMessage(out))
	if msg == "" {
		// Texto genérico de fallback.
		msg = fmt.Sprintf("Hola: su orden %s está lista para retirar. Coordiná la entrega con el taller.", strings.TrimSpace(out.Number))
	}
	if err := u.whatsapp.SendInternalWhatsAppText(ctx, orgID.String(), *out.CustomerID, msg); err != nil {
		if u.audit != nil {
			u.audit.Log(ctx, orgID.String(), actor, "work_order.ready_whatsapp_failed", "work_order", out.ID.String(), map[string]any{"error": "send_failed"})
		}
		return
	}
	at := time.Now().UTC()
	// Marcar idempotencia: actualizamos el campo en memoria; el repo lo persistirá en el próximo Update.
	out.ReadyPickupNotifiedAt = &at
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "work_order.ready_whatsapp_sent", "work_order", out.ID.String(), nil)
	}
}

func normalizeWorkOrder(in *domain.WorkOrder) error {
	in.Number = strings.ToUpper(strings.TrimSpace(in.Number))
	in.TargetType = strings.ToLower(strings.TrimSpace(in.TargetType))
	in.TargetLabel = strings.TrimSpace(in.TargetLabel)
	in.CustomerName = strings.TrimSpace(in.CustomerName)
	in.Status = normalizeWorkOrderStatus(in.Status)
	in.RequestedWork = strings.TrimSpace(in.RequestedWork)
	in.Diagnosis = strings.TrimSpace(in.Diagnosis)
	in.Notes = strings.TrimSpace(in.Notes)
	in.InternalNotes = strings.TrimSpace(in.InternalNotes)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Currency == "" {
		in.Currency = "ARS"
	}
	if in.TargetType == "" {
		return fmt.Errorf("target_type is required: %w", httperrors.ErrBadInput)
	}
	if in.TargetID == uuid.Nil {
		return fmt.Errorf("target_id is required: %w", httperrors.ErrBadInput)
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

func (u *Usecases) enrichReferences(ctx context.Context, in *domain.WorkOrder) error {
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
			item.UnitPrice = vertvalues.ParseFloat(product["price"])
		}
	}
	return nil
}

func normalizeItemType(raw string) string {
	switch strings.TrimSpace(raw) {
	case "part":
		return "part"
	default:
		return "service"
	}
}
