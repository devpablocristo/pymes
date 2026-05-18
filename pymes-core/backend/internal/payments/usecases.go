package payments

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/core/errors/go/domainerr"
	archive "github.com/devpablocristo/modules/crud/archive/go/archive"
	paymentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/payments/usecases/domain"
)

type RepositoryPort interface {
	ListSalePayments(ctx context.Context, orgID, saleID uuid.UUID) ([]paymentsdomain.Payment, error)
	ListArchived(ctx context.Context, orgID uuid.UUID, limit int) ([]paymentsdomain.Payment, error)
	CreateSalePayment(ctx context.Context, orgID, saleID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error)
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

type Usecases struct {
	repo     RepositoryPort
	audit    AuditPort
	notifier NotificationPort
}

func NewUsecases(repo RepositoryPort, audit AuditPort, notifier NotificationPort) *Usecases {
	return &Usecases{repo: repo, audit: audit, notifier: notifier}
}

func (u *Usecases) ListSalePayments(ctx context.Context, orgID, saleID uuid.UUID) ([]paymentsdomain.Payment, error) {
	return u.repo.ListSalePayments(ctx, orgID, saleID)
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
	if err := u.repo.SoftDelete(ctx, orgID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainerr.NotFoundf("payment", id.String())
		}
		return err
	}
	if u.audit != nil {
		u.audit.Log(ctx, orgID.String(), actor, "payment.archived", "payment", id.String(), nil)
	}
	return nil
}

func (u *Usecases) Restore(ctx context.Context, orgID, id uuid.UUID, actor string) error {
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
	return out, nil
}
