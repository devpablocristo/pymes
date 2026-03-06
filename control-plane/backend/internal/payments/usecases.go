package payments

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	paymentsdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/payments/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/apperror"
)

type RepositoryPort interface {
	ListSalePayments(ctx context.Context, orgID, saleID uuid.UUID) ([]paymentsdomain.Payment, error)
	CreateSalePayment(ctx context.Context, orgID, saleID uuid.UUID, in paymentsdomain.Payment) (paymentsdomain.Payment, error)
}

type Usecases struct{ repo RepositoryPort }

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

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
	if in.ReceivedAt.IsZero() { in.ReceivedAt = time.Now().UTC() }
	in.Method = method
	return u.repo.CreateSalePayment(ctx, orgID, saleID, in)
}
