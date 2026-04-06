package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/concurrency/go/resilience"
	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/devpablocristo/core/http/go/httpclient"
	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
	schedulerdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/scheduler/usecases/domain"
)

type RepositoryPort interface {
	ListAutoFetchRateOrgs(ctx context.Context) ([]uuid.UUID, error)
	UpsertExchangeRate(ctx context.Context, orgID uuid.UUID, fromCurrency, toCurrency, rateType string, buyRate, sellRate float64, source string, rateDate time.Time) error
	ListDueRecurring(ctx context.Context, day time.Time) ([]RecurringDue, error)
	ApplyRecurringExpense(ctx context.Context, item RecurringDue, paidAt, nextDue time.Time) error
	ListDueSchedulingReminders(ctx context.Context, now time.Time, limit int) ([]SchedulingReminderDue, error)
	RecordRun(ctx context.Context, task, status, errorMessage string, nextRunAt time.Time) error
}

type WebhookTaskPort interface {
	RetryPending(ctx context.Context) (int, error)
	CleanupOldDeliveries(ctx context.Context, days int) (int64, error)
}

type PaymentGatewayTaskPort interface {
	ProcessPendingWebhookEvents(ctx context.Context, limit int) (int, error)
}

type SchedulingTaskPort interface {
	ExpireOverdueHolds(ctx context.Context, limit int) ([]schedulingdomain.Booking, error)
	CreateBookingActionTokens(ctx context.Context, orgID, bookingID uuid.UUID, ttl time.Duration) (map[schedulingdomain.BookingActionType]schedulingdomain.BookingActionToken, error)
	MarkBookingReminderSent(ctx context.Context, orgID, bookingID uuid.UUID, sentAt time.Time) (schedulingdomain.Booking, error)
	ProcessWaitlistAvailability(ctx context.Context, now time.Time, limit int) ([]schedulingdomain.WaitlistEntry, error)
}

type EmailSenderPort interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

type Usecases struct {
	repo            RepositoryPort
	provider        string
	caller          *httpclient.Caller
	webhooks        WebhookTaskPort
	paymentGateways PaymentGatewayTaskPort
	scheduling      SchedulingTaskPort
	emailSender     EmailSenderPort
	publicBaseURL   string
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

type SchedulingReminderDue struct {
	OrgID         uuid.UUID
	OrgSlug       string
	BookingID     uuid.UUID
	CustomerName  string
	CustomerEmail string
	ServiceName   string
	BranchName    string
	Status        string
	StartAt       time.Time
}

func NewUsecases(repo RepositoryPort, provider string, webhooks WebhookTaskPort, paymentGateways PaymentGatewayTaskPort, scheduling SchedulingTaskPort, emailSender EmailSenderPort, publicBaseURL string) *Usecases {
	return &Usecases{
		repo:     repo,
		provider: strings.ToLower(strings.TrimSpace(provider)),
		caller: &httpclient.Caller{
			HTTP: &http.Client{Timeout: 10 * time.Second},
		},
		webhooks:        webhooks,
		paymentGateways: paymentGateways,
		scheduling:      scheduling,
		emailSender:     emailSender,
		publicBaseURL:   strings.TrimRight(strings.TrimSpace(publicBaseURL), "/"),
	}
}

func (u *Usecases) Run(ctx context.Context, task string) (schedulerdomain.RunResult, error) {
	task = strings.TrimSpace(strings.ToLower(task))
	if task == "" {
		task = "all"
	}
	if task != "all" && task != "exchange_rates" && task != "recurring_expenses" && task != "retry_webhooks" && task != "cleanup_webhook_deliveries" && task != "payment_gateway_webhooks" && task != "scheduling_holds" && task != "scheduling_reminders" && task != "scheduling_waitlist" {
		return schedulerdomain.RunResult{}, domainerr.Validation("invalid task")
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
	if u.scheduling != nil && (task == "all" || task == "scheduling_holds") {
		expired, err := u.runSchedulingHoldExpiration(ctx)
		if err != nil {
			_ = u.repo.RecordRun(ctx, "scheduling_holds", "error", err.Error(), time.Now().UTC().Add(5*time.Minute))
			return schedulerdomain.RunResult{}, err
		}
		result.Metadata["scheduling_holds_expired"] = expired
		_ = u.repo.RecordRun(ctx, "scheduling_holds", "ok", "", time.Now().UTC().Add(5*time.Minute))
	}
	if u.scheduling != nil && u.emailSender != nil && (task == "all" || task == "scheduling_reminders") {
		sent, err := u.sendSchedulingReminders(ctx)
		if err != nil {
			_ = u.repo.RecordRun(ctx, "scheduling_reminders", "error", err.Error(), time.Now().UTC().Add(10*time.Minute))
			return schedulerdomain.RunResult{}, err
		}
		result.Metadata["scheduling_reminders_sent"] = sent
		_ = u.repo.RecordRun(ctx, "scheduling_reminders", "ok", "", time.Now().UTC().Add(10*time.Minute))
	}
	if u.scheduling != nil && u.emailSender != nil && (task == "all" || task == "scheduling_waitlist") {
		notified, err := u.notifySchedulingWaitlist(ctx)
		if err != nil {
			_ = u.repo.RecordRun(ctx, "scheduling_waitlist", "error", err.Error(), time.Now().UTC().Add(10*time.Minute))
			return schedulerdomain.RunResult{}, err
		}
		result.Metadata["scheduling_waitlist_notified"] = notified
		_ = u.repo.RecordRun(ctx, "scheduling_waitlist", "ok", "", time.Now().UTC().Add(10*time.Minute))
	}
	return result, nil
}

func (u *Usecases) runSchedulingHoldExpiration(ctx context.Context) (int, error) {
	items, err := u.scheduling.ExpireOverdueHolds(ctx, 200)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

func (u *Usecases) sendSchedulingReminders(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	items, err := u.repo.ListDueSchedulingReminders(ctx, now, 200)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, item := range items {
		if strings.TrimSpace(item.CustomerEmail) == "" {
			continue
		}
		tokens, err := u.scheduling.CreateBookingActionTokens(ctx, item.OrgID, item.BookingID, 72*time.Hour)
		if err != nil {
			return sent, err
		}
		subject, textBody, htmlBody := buildSchedulingReminderEmail(u.publicBaseURL, item, tokens)
		if err := u.emailSender.Send(ctx, item.CustomerEmail, subject, htmlBody, textBody); err != nil {
			return sent, err
		}
		if _, err := u.scheduling.MarkBookingReminderSent(ctx, item.OrgID, item.BookingID, now); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

func (u *Usecases) notifySchedulingWaitlist(ctx context.Context) (int, error) {
	items, err := u.scheduling.ProcessWaitlistAvailability(ctx, time.Now().UTC(), 200)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, item := range items {
		if strings.TrimSpace(item.CustomerEmail) == "" {
			continue
		}
		subject := "A slot is available for your waitlist request"
		textBody := fmt.Sprintf("Hi %s,\n\nA slot is now available for %s.\nRequested time: %s\nYou can return to the booking flow to complete the reservation.\n", defaultSchedulingName(item.CustomerName), item.RequestedStartAt.Format("2006-01-02 15:04"), item.RequestedStartAt.Format(time.RFC3339))
		htmlBody := "<p>" + strings.ReplaceAll(textBody, "\n", "<br>") + "</p>"
		if err := u.emailSender.Send(ctx, item.CustomerEmail, subject, htmlBody, textBody); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
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
	Name     string  `json:"nombre"`
	BuyRate  float64 `json:"compra"`
	SellRate float64 `json:"venta"`
}

func (u *Usecases) fetchRates(ctx context.Context) ([]remoteRate, error) {
	if u.provider != "dolarapi" {
		return nil, nil
	}
	var payload []dolarAPIResponse
	err := resilience.Retry(ctx, resilience.Config{
		Attempts:     3,
		InitialDelay: 250 * time.Millisecond,
		MaxDelay:     1 * time.Second,
	}, func(ctx context.Context) error {
		st, raw, err := u.caller.DoJSON(ctx, http.MethodGet, "https://dolarapi.com/v1/dolares", nil)
		if err != nil {
			return err
		}
		if st >= 300 {
			return fmt.Errorf("exchange rate provider returned %d", st)
		}
		return json.Unmarshal(raw, &payload)
	})
	if err != nil {
		return nil, err
	}
	mapped := make([]remoteRate, 0, len(payload))
	for _, item := range payload {
		switch name := strings.ToLower(strings.TrimSpace(item.Name)); {
		case strings.Contains(name, "oficial"):
			mapped = append(mapped, remoteRate{RateType: "official", BuyRate: item.BuyRate, SellRate: item.SellRate})
		case strings.Contains(name, "blue"):
			mapped = append(mapped, remoteRate{RateType: "blue", BuyRate: item.BuyRate, SellRate: item.SellRate})
		case strings.Contains(name, "bolsa") || strings.Contains(name, "mep"):
			mapped = append(mapped, remoteRate{RateType: "mep", BuyRate: item.BuyRate, SellRate: item.SellRate})
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

func buildSchedulingReminderEmail(publicBaseURL string, item SchedulingReminderDue, tokens map[schedulingdomain.BookingActionType]schedulingdomain.BookingActionToken) (subject, textBody, htmlBody string) {
	subject = "Booking reminder"
	if strings.EqualFold(strings.TrimSpace(item.Status), string(schedulingdomain.BookingStatusPendingConfirmation)) {
		subject = "Please confirm your booking"
	}
	confirmURL := schedulingActionURL(publicBaseURL, item.OrgSlug, "confirm", tokens[schedulingdomain.BookingActionConfirm].Token)
	cancelURL := schedulingActionURL(publicBaseURL, item.OrgSlug, "cancel", tokens[schedulingdomain.BookingActionCancel].Token)
	lines := []string{
		fmt.Sprintf("Hi %s,", defaultSchedulingName(item.CustomerName)),
		"",
		fmt.Sprintf("This is a reminder for %s at %s.", defaultSchedulingLabel(item.ServiceName), item.StartAt.Format("2006-01-02 15:04")),
	}
	if strings.TrimSpace(item.BranchName) != "" {
		lines = append(lines, "Branch: "+item.BranchName)
	}
	if confirmURL != "" {
		lines = append(lines, "Confirm: "+confirmURL)
	}
	if cancelURL != "" {
		lines = append(lines, "Cancel: "+cancelURL)
	}
	textBody = strings.Join(lines, "\n")
	htmlBody = "<p>" + strings.ReplaceAll(textBody, "\n", "<br>") + "</p>"
	return subject, textBody, htmlBody
}

func schedulingActionURL(baseURL, orgSlug, action, token string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	token = strings.TrimSpace(token)
	if baseURL == "" || token == "" {
		return ""
	}
	orgSlug = strings.TrimSpace(orgSlug)
	if orgSlug == "" {
		return ""
	}
	return fmt.Sprintf("%s/v1/public/%s/scheduling/bookings/actions/%s?token=%s", baseURL, orgSlug, action, token)
}

func defaultSchedulingName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "there"
	}
	return name
}

func defaultSchedulingLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "your booking"
	}
	return label
}
