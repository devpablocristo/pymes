package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	schedulerdomain "github.com/devpablocristo/pymes/control-plane/backend/internal/scheduler/usecases/domain"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/apperror"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/resilience"
)

type RepositoryPort interface {
	ListAutoFetchRateOrgs(ctx context.Context) ([]uuid.UUID, error)
	UpsertExchangeRate(ctx context.Context, orgID uuid.UUID, fromCurrency, toCurrency, rateType string, buyRate, sellRate float64, source string, rateDate time.Time) error
	ListDueRecurring(ctx context.Context, day time.Time) ([]RecurringDue, error)
	ApplyRecurringExpense(ctx context.Context, item RecurringDue, paidAt, nextDue time.Time) error
	RecordRun(ctx context.Context, task, status, errorMessage string, nextRunAt time.Time) error
}

type WebhookTaskPort interface {
	RetryPending(ctx context.Context) (int, error)
	CleanupOldDeliveries(ctx context.Context, days int) (int64, error)
}

type PaymentGatewayTaskPort interface {
	ProcessPendingWebhookEvents(ctx context.Context, limit int) (int, error)
}

type Usecases struct {
	repo            RepositoryPort
	provider        string
	client          *http.Client
	webhooks        WebhookTaskPort
	paymentGateways PaymentGatewayTaskPort
}

type RecurringDue struct {
	ID            uuid.UUID
	OrgID         uuid.UUID
	Description   string
	Amount        float64
	Currency      string
	Category      string
	PaymentMethod string
	Frequency     string
	DayOfMonth    int
	NextDueDate   time.Time
}

func NewUsecases(repo RepositoryPort, provider string, webhooks WebhookTaskPort, paymentGateways PaymentGatewayTaskPort) *Usecases {
	return &Usecases{
		repo:            repo,
		provider:        strings.ToLower(strings.TrimSpace(provider)),
		client:          &http.Client{Timeout: 10 * time.Second},
		webhooks:        webhooks,
		paymentGateways: paymentGateways,
	}
}

func (u *Usecases) Run(ctx context.Context, task string) (schedulerdomain.RunResult, error) {
	task = strings.TrimSpace(strings.ToLower(task))
	if task == "" {
		task = "all"
	}
	if task != "all" && task != "exchange_rates" && task != "recurring_expenses" && task != "retry_webhooks" && task != "cleanup_webhook_deliveries" && task != "payment_gateway_webhooks" {
		return schedulerdomain.RunResult{}, apperror.NewBadInput("invalid task")
	}
	result := schedulerdomain.RunResult{Task: task, Metadata: map[string]any{}}
	if task == "all" || task == "exchange_rates" {
		updated, err := u.syncRates(ctx)
		if err != nil {
			_ = u.repo.RecordRun(ctx, "exchange_rates", "error", err.Error(), time.Now().UTC().Add(1*time.Hour))
			return schedulerdomain.RunResult{}, err
		}
		result.RatesUpdated = updated
		_ = u.repo.RecordRun(ctx, "exchange_rates", "ok", "", time.Now().UTC().Add(1*time.Hour))
	}
	if task == "all" || task == "recurring_expenses" {
		applied, err := u.applyRecurring(ctx)
		if err != nil {
			_ = u.repo.RecordRun(ctx, "recurring_expenses", "error", err.Error(), time.Now().UTC().Add(24*time.Hour))
			return schedulerdomain.RunResult{}, err
		}
		result.RecurringApplied = applied
		_ = u.repo.RecordRun(ctx, "recurring_expenses", "ok", "", time.Now().UTC().Add(24*time.Hour))
	}
	if u.webhooks != nil && (task == "all" || task == "retry_webhooks") {
		retried, err := u.webhooks.RetryPending(ctx)
		if err != nil {
			_ = u.repo.RecordRun(ctx, "retry_webhooks", "error", err.Error(), time.Now().UTC().Add(5*time.Minute))
			return schedulerdomain.RunResult{}, err
		}
		result.Metadata["webhooks_processed"] = retried
		_ = u.repo.RecordRun(ctx, "retry_webhooks", "ok", "", time.Now().UTC().Add(5*time.Minute))
	}
	if u.webhooks != nil && (task == "all" || task == "cleanup_webhook_deliveries") {
		removed, err := u.webhooks.CleanupOldDeliveries(ctx, 30)
		if err != nil {
			_ = u.repo.RecordRun(ctx, "cleanup_webhook_deliveries", "error", err.Error(), time.Now().UTC().Add(24*time.Hour))
			return schedulerdomain.RunResult{}, err
		}
		result.Metadata["webhooks_deleted"] = removed
		_ = u.repo.RecordRun(ctx, "cleanup_webhook_deliveries", "ok", "", time.Now().UTC().Add(24*time.Hour))
	}
	if u.paymentGateways != nil && (task == "all" || task == "payment_gateway_webhooks") {
		processed, err := u.paymentGateways.ProcessPendingWebhookEvents(ctx, 100)
		if err != nil {
			_ = u.repo.RecordRun(ctx, "payment_gateway_webhooks", "error", err.Error(), time.Now().UTC().Add(5*time.Minute))
			return schedulerdomain.RunResult{}, err
		}
		result.Metadata["payment_gateway_events_processed"] = processed
		_ = u.repo.RecordRun(ctx, "payment_gateway_webhooks", "ok", "", time.Now().UTC().Add(5*time.Minute))
	}
	return result, nil
}

func (u *Usecases) applyRecurring(ctx context.Context) (int, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	items, err := u.repo.ListDueRecurring(ctx, today)
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, item := range items {
		nextDue := nextRecurringDate(today, item.Frequency, item.DayOfMonth)
		if err := u.repo.ApplyRecurringExpense(ctx, item, today, nextDue); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

func (u *Usecases) syncRates(ctx context.Context) (int, error) {
	if u.provider == "" || u.provider == "manual" {
		return 0, nil
	}
	rates, err := u.fetchRates(ctx)
	if err != nil {
		return 0, err
	}
	orgs, err := u.repo.ListAutoFetchRateOrgs(ctx)
	if err != nil {
		return 0, err
	}
	if len(orgs) == 0 || len(rates) == 0 {
		return 0, nil
	}
	today := time.Now().UTC()
	updated := 0
	for _, orgID := range orgs {
		for _, rate := range rates {
			if err := u.repo.UpsertExchangeRate(ctx, orgID, "USD", "ARS", rate.RateType, rate.BuyRate, rate.SellRate, "api", today); err != nil {
				return updated, err
			}
			updated++
		}
	}
	return updated, nil
}

type remoteRate struct {
	RateType string
	BuyRate  float64
	SellRate float64
}

type dolarAPIResponse struct {
	Nombre string  `json:"nombre"`
	Compra float64 `json:"compra"`
	Venta  float64 `json:"venta"`
}

func (u *Usecases) fetchRates(ctx context.Context) ([]remoteRate, error) {
	if u.provider != "dolarapi" {
		return nil, nil
	}
	var payload []dolarAPIResponse
	err := resilience.Retry(ctx, resilience.Backoff{Attempts: 3, Initial: 250 * time.Millisecond, Max: 1 * time.Second}, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://dolarapi.com/v1/dolares", nil)
		if err != nil {
			return err
		}
		res, err := u.client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode >= 300 {
			return fmt.Errorf("exchange rate provider returned %d", res.StatusCode)
		}
		return json.NewDecoder(res.Body).Decode(&payload)
	})
	if err != nil {
		return nil, err
	}
	mapped := make([]remoteRate, 0, len(payload))
	for _, item := range payload {
		switch name := strings.ToLower(strings.TrimSpace(item.Nombre)); {
		case strings.Contains(name, "oficial"):
			mapped = append(mapped, remoteRate{RateType: "official", BuyRate: item.Compra, SellRate: item.Venta})
		case strings.Contains(name, "blue"):
			mapped = append(mapped, remoteRate{RateType: "blue", BuyRate: item.Compra, SellRate: item.Venta})
		case strings.Contains(name, "bolsa") || strings.Contains(name, "mep"):
			mapped = append(mapped, remoteRate{RateType: "mep", BuyRate: item.Compra, SellRate: item.Venta})
		}
	}
	return mapped, nil
}

func nextRecurringDate(base time.Time, frequency string, dayOfMonth int) time.Time {
	switch strings.TrimSpace(strings.ToLower(frequency)) {
	case "weekly":
		return base.AddDate(0, 0, 7)
	case "biweekly":
		return base.AddDate(0, 0, 14)
	case "quarterly":
		return time.Date(base.Year(), base.Month()+3, normalizeDay(dayOfMonth), 0, 0, 0, 0, time.UTC)
	case "yearly":
		return time.Date(base.Year()+1, base.Month(), normalizeDay(dayOfMonth), 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(base.Year(), base.Month()+1, normalizeDay(dayOfMonth), 0, 0, 0, 0, time.UTC)
	}
}

func normalizeDay(day int) int {
	if day <= 0 {
		return 1
	}
	if day > 28 {
		return 28
	}
	return day
}
