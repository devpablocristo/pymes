package currency

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	currencydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/currency/usecases/domain"
	"github.com/devpablocristo/pymes/pkgs/go-pkg/apperror"
)

type RepositoryPort interface {
	ListLatest(ctx context.Context, orgID uuid.UUID, fromCurrency, toCurrency string, limit int) ([]currencydomain.ExchangeRate, error)
	Upsert(ctx context.Context, in currencydomain.ExchangeRate) (currencydomain.ExchangeRate, error)
}

type Usecases struct {
	repo RepositoryPort
}

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

func (u *Usecases) ListLatest(ctx context.Context, orgID uuid.UUID, fromCurrency, toCurrency string, limit int) ([]currencydomain.ExchangeRate, error) {
	return u.repo.ListLatest(ctx, orgID, strings.TrimSpace(fromCurrency), strings.TrimSpace(toCurrency), limit)
}

func (u *Usecases) Upsert(ctx context.Context, in currencydomain.ExchangeRate) (currencydomain.ExchangeRate, error) {
	if strings.TrimSpace(in.FromCurrency) == "" || strings.TrimSpace(in.ToCurrency) == "" {
		return currencydomain.ExchangeRate{}, apperror.NewBadInput("from_currency and to_currency are required")
	}
	if strings.TrimSpace(in.RateType) == "" {
		return currencydomain.ExchangeRate{}, apperror.NewBadInput("rate_type is required")
	}
	if in.BuyRate <= 0 || in.SellRate <= 0 {
		return currencydomain.ExchangeRate{}, apperror.NewBadInput("buy_rate and sell_rate must be > 0")
	}
	if in.RateDate.IsZero() {
		in.RateDate = time.Now().UTC()
	}
	if strings.TrimSpace(in.Source) == "" {
		in.Source = "manual"
	}
	if in.ID == uuid.Nil {
		in.ID = uuid.New()
	}
	return u.repo.Upsert(ctx, in)
}
