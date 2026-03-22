package payments

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/backend/go/apperror"
	paymentsdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/payments/usecases/domain"
)

type RepositoryPort interface {
	ListSalePayments(ctx context.Context, orgID, saleID uuid.UUID) ([]paymentsdomain.Payment, error)
	CreateSalePayment(ctx context.Context, orgID, saleID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error)
}

// AuditPort registra cobros en audit_log (conciliación caja–venta y trazabilidad anti-fraude).
type AuditPort interface {
	Log(ctx context.Context, orgID string, actor, action, resourceType, resourceID string, payload map[string]any)
}

type Usecases struct {
	repo  RepositoryPort
	audit AuditPort
}

func NewUsecases(repo RepositoryPort, audit AuditPort) *Usecases {
	return &Usecases{repo: repo, audit: audit}
}

func (u *Usecases) ListSalePayments(ctx context.Context, orgID, saleID uuid.UUID) ([]paymentsdomain.Payment, error) {
	return u.repo.ListSalePayments(ctx, orgID, saleID)
}

func (u *Usecases) CreateSalePayment(ctx context.Context, orgID, saleID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error) {
	method := strings.TrimSpace(strings.ToLower(in.Method))
	if method != "cash" && method != "card" && method != "transfer" && method != "check" && method != "other" && method != "credit_note" && method != "mercadopago" {
		return paymentsdomain.Payment{}, apperror.NewBadInput("invalid payment method")
	}
	if in.Amount <= 0 {
		return paymentsdomain.Payment{}, apperror.NewBadInput("amount must be > 0")
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
			"sale_id":      saleID.String(),
			"amount":       out.Amount,
			"method":       out.Method,
			"received_at": out.ReceivedAt.UTC().Format(time.RFC3339),
		}
		if strings.TrimSpace(out.Notes) != "" {
			payload["notes"] = out.Notes
		}
		u.audit.Log(ctx, orgID.String(), out.CreatedBy, "payment.created", "payment", out.ID.String(), payload)
	}
	return out, nil
}
