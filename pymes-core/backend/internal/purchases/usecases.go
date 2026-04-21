package purchases

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/errors/go/domainerr"
	purchasesdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/purchases/usecases/domain"
)

type RepositoryPort interface {
	List(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, status string, limit int) ([]purchasesdomain.Purchase, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, status string, limit int) ([]purchasesdomain.Purchase, error)
	Create(ctx context.Context, in CreateInput) (purchasesdomain.Purchase, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (purchasesdomain.Purchase, error)
	Update(ctx context.Context, in UpdateInput) (purchasesdomain.Purchase, error)
	UpdateStatus(ctx context.Context, in UpdateStatusInput) (purchasesdomain.Purchase, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
	GetSupplierName(ctx context.Context, orgID, supplierID uuid.UUID) (string, error)
	GetCurrency(ctx context.Context, orgID uuid.UUID) string
	GetTaxRate(ctx context.Context, orgID uuid.UUID) float64
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
	repo     RepositoryPort
	audit    AuditPort
	timeline TimelinePort
	webhooks WebhookPort
}

type Option func(*Usecases)

func WithTimeline(t TimelinePort) Option { return func(u *Usecases) { u.timeline = t } }
func WithWebhooks(w WebhookPort) Option  { return func(u *Usecases) { u.webhooks = w } }

func NewUsecases(repo RepositoryPort, audit AuditPort, opts ...Option) *Usecases {
	uc := &Usecases{repo: repo, audit: audit}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

type CreateInput struct {
	OrgID         uuid.UUID
	BranchID      *uuid.UUID
	SupplierID    *uuid.UUID
	SupplierName  string
	Status        string
	PaymentStatus string
	IsFavorite    bool
	Tags          []string
	Notes         string
	CreatedBy     string
	Items         []purchasesdomain.PurchaseItem
}

type UpdateInput struct {
	ID            uuid.UUID
	OrgID         uuid.UUID
	BranchID      *uuid.UUID
	SupplierID    *uuid.UUID
	SupplierName  string
	Status        string
	PaymentStatus string
	IsFavorite    bool
	Tags          []string
	Notes         string
	Items         []purchasesdomain.PurchaseItem
}

type UpdateStatusInput struct {
	ID     uuid.UUID
	OrgID  uuid.UUID
	Status string
}

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, status string, limit int) ([]purchasesdomain.Purchase, error) {
	return u.repo.List(ctx, orgID, branchID, strings.TrimSpace(status), limit)
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID, branchID *uuid.UUID, status string, limit int) ([]purchasesdomain.Purchase, error) {
	return u.repo.ListArchived(ctx, orgID, branchID, strings.TrimSpace(status), limit)
}

func (u *Usecases) Create(ctx context.Context, in CreateInput) (purchasesdomain.Purchase, error) {
	prepared, err := u.prepareCreate(ctx, in)
	if err != nil {
		return purchasesdomain.Purchase{}, err
	}
	out, err := u.repo.Create(ctx, prepared)
	if err != nil {
		return purchasesdomain.Purchase{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), in.CreatedBy, "purchase.created", "purchase", out.ID.String(), map[string]any{"number": out.Number, "total": out.Total})
	}
	if u.timeline != nil && out.SupplierID != nil {
		_ = u.timeline.RecordEvent(ctx, in.OrgID, "parties", *out.SupplierID, "purchase.created", "Compra registrada", out.Number, in.CreatedBy, map[string]any{"purchase_id": out.ID.String(), "total": out.Total})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, in.OrgID, "purchase.created", map[string]any{"purchase_id": out.ID.String(), "supplier_id": nullableUUID(out.SupplierID), "total": out.Total, "status": out.Status})
	}
	return out, nil
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (purchasesdomain.Purchase, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return purchasesdomain.Purchase{}, domainerr.NotFoundf("purchase", id.String())
		}
		return purchasesdomain.Purchase{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, in UpdateInput, actor string) (purchasesdomain.Purchase, error) {
	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return purchasesdomain.Purchase{}, domainerr.NotFoundf("purchase", in.ID.String())
		}
		return purchasesdomain.Purchase{}, err
	}
	if current.DeletedAt != nil {
		return purchasesdomain.Purchase{}, domainerr.NotFoundf("purchase", in.ID.String())
	}
	prepared, err := u.prepareCreate(ctx, CreateInput{OrgID: in.OrgID, BranchID: in.BranchID, SupplierID: in.SupplierID, SupplierName: in.SupplierName, Status: in.Status, PaymentStatus: in.PaymentStatus, Notes: in.Notes, CreatedBy: current.CreatedBy, Items: in.Items})
	if err != nil {
		return purchasesdomain.Purchase{}, err
	}
	out, err := u.repo.Update(ctx, UpdateInput{ID: in.ID, OrgID: in.OrgID, BranchID: prepared.BranchID, SupplierID: prepared.SupplierID, SupplierName: prepared.SupplierName, Status: prepared.Status, PaymentStatus: prepared.PaymentStatus, IsFavorite: prepared.IsFavorite, Tags: prepared.Tags, Notes: prepared.Notes, Items: prepared.Items})
	if err != nil {
		return purchasesdomain.Purchase{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), actor, "purchase.updated", "purchase", out.ID.String(), map[string]any{"status": out.Status, "total": out.Total})
	}
	if u.timeline != nil && out.SupplierID != nil {
		_ = u.timeline.RecordEvent(ctx, in.OrgID, "parties", *out.SupplierID, "purchase.updated", "Compra actualizada", out.Number, actor, map[string]any{"purchase_id": out.ID.String(), "status": out.Status})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, in.OrgID, "purchase.updated", map[string]any{"purchase_id": out.ID.String(), "supplier_id": nullableUUID(out.SupplierID), "status": out.Status})
	}
	return out, nil
}

func (u *Usecases) UpdateStatus(ctx context.Context, in UpdateStatusInput, actor string) (purchasesdomain.Purchase, error) {
	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return purchasesdomain.Purchase{}, domainerr.NotFoundf("purchase", in.ID.String())
		}
		return purchasesdomain.Purchase{}, err
	}
	if current.DeletedAt != nil {
		return purchasesdomain.Purchase{}, domainerr.NotFoundf("purchase", in.ID.String())
	}
	nextStatus, err := normalizePurchaseStatus(in.Status, "")
	if err != nil {
		return purchasesdomain.Purchase{}, err
	}
	if !canTransitionPurchaseStatus(current.Status, nextStatus) {
		return purchasesdomain.Purchase{}, domainerr.BusinessRule("purchase status transition is not allowed")
	}
	out, err := u.repo.UpdateStatus(ctx, UpdateStatusInput{
		ID:     in.ID,
		OrgID:  in.OrgID,
		Status: nextStatus,
	})
	if err != nil {
		return purchasesdomain.Purchase{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, in.OrgID.String(), actor, "purchase.status_updated", "purchase", out.ID.String(), map[string]any{"status": out.Status})
	}
	if u.timeline != nil && out.SupplierID != nil {
		_ = u.timeline.RecordEvent(ctx, in.OrgID, "parties", *out.SupplierID, "purchase.status_updated", "Estado de compra actualizado", out.Number, actor, map[string]any{"purchase_id": out.ID.String(), "status": out.Status})
	}
	if u.webhooks != nil {
		_ = u.webhooks.Enqueue(ctx, in.OrgID, "purchase.status_updated", map[string]any{"purchase_id": out.ID.String(), "supplier_id": nullableUUID(out.SupplierID), "status": out.Status})
	}
	return out, nil
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("purchase", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "purchase.archived", "purchase", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("purchase", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "purchase.restored", "purchase", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("purchase", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "purchase.deleted", "purchase", id.String(), map[string]any{})
	}
	return nil
}

func (u *Usecases) prepareCreate(ctx context.Context, in CreateInput) (CreateInput, error) {
	if in.OrgID == uuid.Nil {
		return CreateInput{}, domainerr.Validation("org_id is required")
	}
	if len(in.Items) == 0 {
		return CreateInput{}, domainerr.Validation("items are required")
	}
	if in.SupplierID != nil && *in.SupplierID != uuid.Nil && strings.TrimSpace(in.SupplierName) == "" {
		if name, err := u.repo.GetSupplierName(ctx, in.OrgID, *in.SupplierID); err == nil && strings.TrimSpace(name) != "" {
			in.SupplierName = name
		}
	}
	if strings.TrimSpace(in.SupplierName) == "" {
		in.SupplierName = "Proveedor sin nombre"
	}
	status, err := normalizePurchaseStatus(in.Status, "draft")
	if err != nil {
		return CreateInput{}, err
	}
	in.Status = status
	in.PaymentStatus = defaultString(strings.ToLower(in.PaymentStatus), "pending")
	currency := u.repo.GetCurrency(ctx, in.OrgID)
	defaultTax := u.repo.GetTaxRate(ctx, in.OrgID)
	items := make([]purchasesdomain.PurchaseItem, 0, len(in.Items))
	for idx, item := range in.Items {
		if item.Quantity <= 0 || item.UnitCost < 0 {
			return CreateInput{}, domainerr.Validation("invalid purchase item")
		}
		taxRate := item.TaxRate
		if taxRate <= 0 {
			taxRate = defaultTax
		}
		description := strings.TrimSpace(item.Description)
		if description == "" {
			description = "Item"
		}
		items = append(items, purchasesdomain.PurchaseItem{ID: item.ID, ProductID: item.ProductID, ServiceID: item.ServiceID, Description: description, Quantity: item.Quantity, UnitCost: item.UnitCost, TaxRate: taxRate, Subtotal: item.Quantity * item.UnitCost, SortOrder: idx + 1})
	}
	_ = currency
	in.Items = items
	return in, nil
}

func normalizePurchaseStatus(raw, defaultValue string) (string, error) {
	status := strings.TrimSpace(strings.ToLower(raw))
	if status == "" {
		status = strings.TrimSpace(strings.ToLower(defaultValue))
	}
	switch status {
	case "draft", "received", "partial", "voided":
		return status, nil
	default:
		return "", domainerr.Validation("invalid status")
	}
}

func canTransitionPurchaseStatus(from, to string) bool {
	fromStatus, err := normalizePurchaseStatus(from, "")
	if err != nil {
		return false
	}
	toStatus, err := normalizePurchaseStatus(to, "")
	if err != nil {
		return false
	}
	switch fromStatus {
	case "draft":
		return toStatus == "draft" || toStatus == "partial" || toStatus == "received" || toStatus == "voided"
	case "partial":
		return toStatus == "draft" || toStatus == "partial" || toStatus == "received" || toStatus == "voided"
	case "received":
		return toStatus == "draft" || toStatus == "partial" || toStatus == "received" || toStatus == "voided"
	case "voided":
		return toStatus == "draft" || toStatus == "partial" || toStatus == "received" || toStatus == "voided"
	default:
		return false
	}
}

func defaultString(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}

func markReceivedAt(status string) *time.Time {
	if status == "received" {
		now := time.Now().UTC()
		return &now
	}
	return nil
}

func nullableUUID(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
