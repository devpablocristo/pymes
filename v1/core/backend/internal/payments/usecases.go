package payments

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/platform/errors/go/domainerr"
	archive "github.com/devpablocristo/platform/lifecycle/go/archive"
	lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
	"github.com/devpablocristo/pymes/core/backend/internal/ledger"
	paymentsdomain "github.com/devpablocristo/pymes/core/backend/internal/payments/usecases/domain"
)

type RepositoryPort interface {
	ListSalePayments(ctx context.Context, orgID, saleID uuid.UUID) ([]paymentsdomain.Payment, error)
	ListPurchasePayments(ctx context.Context, orgID, purchaseID uuid.UUID) ([]paymentsdomain.Payment, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]paymentsdomain.Payment, error)
	CreateSalePayment(ctx context.Context, orgID, saleID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error)
	CreatePurchasePayment(ctx context.Context, orgID, purchaseID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (paymentsdomain.Payment, error)
	Update(ctx context.Context, in paymentsdomain.Payment) (paymentsdomain.Payment, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	Restore(ctx context.Context, orgID, id uuid.UUID) error
	HardDelete(ctx context.Context, orgID, id uuid.UUID) error
}

// AuditPort registra cobros en audit_log (conciliación caja–venta y trazabilidad anti-fraude).
type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type NotificationPort interface {
	NotifyPaymentCreated(ctx context.Context, orgID, saleID uuid.UUID, payment paymentsdomain.Payment) error
}

// LedgerPort encola el evento contable del cobro (posteo desacoplado por outbox).
// Nil-safe: si no está cableado, el cobro opera igual.
type LedgerPort interface {
	EnqueuePayment(ctx context.Context, evt ledger.PaymentEvent) error
	EnqueuePurchasePayment(ctx context.Context, evt ledger.PurchasePaymentEvent) error
	EnqueueReversal(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID, targetEvent, actor string) error
}

type Usecases struct {
	repo      RepositoryPort
	audit     AuditPort
	notifier  NotificationPort
	ledger    LedgerPort
	lifecycle *lifecycle.Service // optional; when nil, legacy path
}

// Option customizes Usecases at construction.
type Option func(*Usecases)

// WithLifecycle wires lifecycle.Service for Soft/Restore/HardDelete.
func WithLifecycle(svc *lifecycle.Service) Option {
	return func(u *Usecases) {
		if svc != nil {
			u.lifecycle = svc
		}
	}
}

// WithLedger cablea el posteo contable del cobro.
func WithLedger(l LedgerPort) Option { return func(u *Usecases) { u.ledger = l } }

func NewUsecases(repo RepositoryPort, audit AuditPort, notifier NotificationPort, opts ...Option) *Usecases {
	u := &Usecases{repo: repo, audit: audit, notifier: notifier}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

func (u *Usecases) ListSalePayments(ctx context.Context, orgID, saleID uuid.UUID) ([]paymentsdomain.Payment, error) {
	return u.repo.ListSalePayments(ctx, orgID, saleID)
}

func (u *Usecases) ListPurchasePayments(ctx context.Context, orgID, purchaseID uuid.UUID) ([]paymentsdomain.Payment, error) {
	return u.repo.ListPurchasePayments(ctx, orgID, purchaseID)
}

func (u *Usecases) ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]paymentsdomain.Payment, error) {
	return u.repo.ListArchived(ctx, orgID, limit)
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (paymentsdomain.Payment, error) {
	out, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return paymentsdomain.Payment{}, domainerr.NotFoundf("payment", id.String())
		}
		return paymentsdomain.Payment{}, err
	}
	return out, nil
}

func (u *Usecases) Update(ctx context.Context, in paymentsdomain.Payment, actor string) (paymentsdomain.Payment, error) {
	current, err := u.repo.GetByID(ctx, in.OrgID, in.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return paymentsdomain.Payment{}, domainerr.NotFoundf("payment", in.ID.String())
		}
		return paymentsdomain.Payment{}, err
	}
	if err := archive.IfArchived(current.ArchivedAt, "payment"); err != nil {
		return paymentsdomain.Payment{}, err
	}
	// Solo favoritos/tags/notas son editables; monto/método/fecha son inmutables.
	current.Notes = strings.TrimSpace(in.Notes)
	current.IsFavorite = in.IsFavorite
	current.Tags = in.Tags
	out, err := u.repo.Update(ctx, current)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return paymentsdomain.Payment{}, domainerr.NotFoundf("payment", in.ID.String())
		}
		return paymentsdomain.Payment{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, out.OrgID.String(), actor, "payment.updated", "payment", out.ID.String(), nil)
	}
	return out, nil
}

func (u *Usecases) SoftDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if u.lifecycle != nil {
		if err := u.lifecycle.SoftDelete(ctx, &lifecycle.ArchiveRequest{
			ResourceType: ResourceTypePayment,
			ResourceID:   id,
			TenantID:     orgID.String(),
			Actor:        actor,
		}); err != nil {
			return err
		}
	} else {
		if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domainerr.NotFoundf("payment", id.String())
			}
			return err
		}
		if u.audit != nil {
			u.audit.Log(ctx, orgID.String(), actor, "payment.archived", "payment", id.String(), nil)
		}
	}
	// Reversa contable del cobro (si posteó asiento; los cobros de venta de
	// contado no postearon, el worker lo ignora).
	if u.ledger != nil {
		_ = u.ledger.EnqueueReversal(ctx, orgID, "payment", id, "", actor)
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if u.lifecycle != nil {
		return u.lifecycle.Restore(ctx, &lifecycle.RestoreRequest{
			ResourceType: ResourceTypePayment,
			ResourceID:   id,
			TenantID:     orgID.String(),
			Actor:        actor,
		})
	}
	if err := u.repo.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("payment", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "payment.restored", "payment", id.String(), nil)
	}
	return nil
}

func (u *Usecases) HardDelete(ctx context.Context, orgID, id uuid.UUID, actor string) error {
	if u.lifecycle != nil {
		return u.lifecycle.HardDelete(ctx, &lifecycle.HardDeleteRequest{
			ResourceType:   ResourceTypePayment,
			ResourceID:     id,
			TenantID:       orgID.String(),
			Actor:          actor,
			MustBeArchived: false,
		})
	}
	if err := u.repo.HardDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("payment", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "payment.hard_deleted", "payment", id.String(), nil)
	}
	return nil
}

func (u *Usecases) CreateSalePayment(ctx context.Context, orgID, saleID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error) {
	method := strings.TrimSpace(strings.ToLower(in.Method))
	if method != "cash" && method != "card" && method != "transfer" && method != "check" && method != "other" && method != "credit_note" && method != "mercadopago" {
		return paymentsdomain.Payment{}, domainerr.Validation("invalid payment method")
	}
	if in.Amount <= 0 {
		return paymentsdomain.Payment{}, domainerr.Validation("amount must be > 0")
	}
	if in.ReceivedAt.IsZero() {
		in.ReceivedAt = time.Now().UTC()
	}
	in.Method = method
	out, err := u.repo.CreateSalePayment(ctx, orgID, saleID, in)
	if err != nil {
		return paymentsdomain.Payment{}, err
	}
	if u.audit != nil {
		payload := map[string]any{
			"sale_id":     saleID.String(),
			"amount":      out.Amount,
			"method":      out.Method,
			"received_at": out.ReceivedAt.UTC().Format(time.RFC3339),
		}
		if strings.TrimSpace(out.Notes) != "" {
			payload["notes"] = out.Notes
		}
		u.audit.Log(ctx, orgID.String(), out.CreatedBy, "payment.created", "payment", out.ID.String(), payload)
	}
	if u.notifier != nil {
		_ = u.notifier.NotifyPaymentCreated(ctx, orgID, saleID, out)
	}
	if u.ledger != nil {
		_ = u.ledger.EnqueuePayment(ctx, ledger.PaymentEvent{
			OrgID:      orgID,
			PaymentID:  out.ID,
			SaleID:     saleID,
			Method:     out.Method,
			Amount:     out.Amount,
			OccurredAt: out.ReceivedAt,
			Actor:      out.CreatedBy,
		})
	}
	return out, nil
}

func isValidPaymentMethod(method string) bool {
	switch strings.TrimSpace(strings.ToLower(method)) {
	case "cash", "card", "transfer", "check", "other", "credit_note", "mercadopago":
		return true
	default:
		return false
	}
}

func (u *Usecases) CreatePurchasePayment(ctx context.Context, orgID, purchaseID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error) {
	method := strings.TrimSpace(strings.ToLower(in.Method))
	if !isValidPaymentMethod(method) {
		return paymentsdomain.Payment{}, domainerr.Validation("invalid payment method")
	}
	if in.Amount <= 0 {
		return paymentsdomain.Payment{}, domainerr.Validation("amount must be > 0")
	}
	if in.ReceivedAt.IsZero() {
		in.ReceivedAt = time.Now().UTC()
	}
	in.Method = method
	out, err := u.repo.CreatePurchasePayment(ctx, orgID, purchaseID, in)
	if err != nil {
		return paymentsdomain.Payment{}, err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), out.CreatedBy, "purchase_payment.created", "payment", out.ID.String(), map[string]any{
			"purchase_id": purchaseID.String(),
			"amount":      out.Amount,
			"method":      out.Method,
		})
	}
	if u.ledger != nil {
		_ = u.ledger.EnqueuePurchasePayment(ctx, ledger.PurchasePaymentEvent{
			OrgID:      orgID,
			PaymentID:  out.ID,
			PurchaseID: purchaseID,
			Method:     out.Method,
			Amount:     out.Amount,
			OccurredAt: out.ReceivedAt,
			Actor:      out.CreatedBy,
		})
	}
	return out, nil
}
