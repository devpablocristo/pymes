package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
)

type fakeRepo struct {
	recordedRuns []recordedRun
}

type recordedRun struct {
	Task, Status, ErrorMessage string
}

func (f *fakeRepo) ListAutoFetchRateOrgs(_ context.Context) ([]uuid.UUID, error) {
	return nil, nil
}

func (f *fakeRepo) UpsertExchangeRate(_ context.Context, _ uuid.UUID, _, _, _ string, _, _ float64, _ string, _ time.Time) error {
	return nil
}

func (f *fakeRepo) ListDueRecurring(_ context.Context, _ time.Time) ([]RecurringDue, error) {
	return nil, nil
}

func (f *fakeRepo) ApplyRecurringExpense(_ context.Context, _ RecurringDue, _, _ time.Time) error {
	return nil
}

func (f *fakeRepo) ListDueSchedulingReminders(_ context.Context, _ time.Time, _ int) ([]SchedulingReminderDue, error) {
	return nil, nil
}

func (f *fakeRepo) RecordRun(_ context.Context, task, status, errorMessage string, _ time.Time) error {
	f.recordedRuns = append(f.recordedRuns, recordedRun{Task: task, Status: status, ErrorMessage: errorMessage})
	return nil
}

type fakeWebhooks struct {
	retried int
	cleaned int64
}

func (f *fakeWebhooks) RetryPending(_ context.Context) (int, error) { return f.retried, nil }
func (f *fakeWebhooks) CleanupOldDeliveries(_ context.Context, _ int) (int64, error) {
	return f.cleaned, nil
}

type fakePaymentGateways struct {
	processed int
}

func (f *fakePaymentGateways) ProcessPendingWebhookEvents(_ context.Context, _ int) (int, error) {
	return f.processed, nil
}

type fakeScheduling struct {
	expired []schedulingdomain.Booking
}

func (f *fakeScheduling) ExpireOverdueHolds(_ context.Context, _ int) ([]schedulingdomain.Booking, error) {
	return f.expired, nil
}

func (f *fakeScheduling) CreateBookingActionTokens(_ context.Context, _, _ uuid.UUID, _ time.Duration) (map[schedulingdomain.BookingActionType]schedulingdomain.BookingActionToken, error) {
	return nil, nil
}

func (f *fakeScheduling) MarkBookingReminderSent(_ context.Context, _, _ uuid.UUID, _ time.Time) (schedulingdomain.Booking, error) {
	return schedulingdomain.Booking{}, nil
}

func (f *fakeScheduling) ProcessWaitlistAvailability(_ context.Context, _ time.Time, _ int) ([]schedulingdomain.WaitlistEntry, error) {
	return nil, nil
}

func TestRunAllTasksHappyPath(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, "", &fakeWebhooks{retried: 3, cleaned: 5}, &fakePaymentGateways{processed: 2}, &fakeScheduling{}, nil, "")

	result, err := uc.Run(context.Background(), "all")
	if err != nil {
		t.Fatalf("Run(all) error = %v", err)
	}
	if result.Task != "all" {
		t.Fatalf("expected task=all, got %s", result.Task)
	}
	if result.Metadata["webhooks_processed"] != 3 {
		t.Fatalf("expected webhooks_processed=3, got %v", result.Metadata["webhooks_processed"])
	}
	if result.Metadata["webhooks_deleted"] != int64(5) {
		t.Fatalf("expected webhooks_deleted=5, got %v", result.Metadata["webhooks_deleted"])
	}
	if result.Metadata["payment_gateway_events_processed"] != 2 {
		t.Fatalf("expected payment_gateway_events_processed=2, got %v", result.Metadata["payment_gateway_events_processed"])
	}
}

func TestRunSingleTask(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, "", nil, nil, nil, nil, "")

	result, err := uc.Run(context.Background(), "exchange_rates")
	if err != nil {
		t.Fatalf("Run(exchange_rates) error = %v", err)
	}
	if result.Task != "exchange_rates" {
		t.Fatalf("expected task=exchange_rates, got %s", result.Task)
	}
	if len(repo.recordedRuns) == 0 {
		t.Fatal("expected at least one recorded run")
	}
	if repo.recordedRuns[0].Task != "exchange_rates" {
		t.Fatalf("expected recorded task=exchange_rates, got %s", repo.recordedRuns[0].Task)
	}
}

func TestRunInvalidTask(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(&fakeRepo{}, "", nil, nil, nil, nil, "")

	_, err := uc.Run(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid task")
	}
}

func TestRunRecordsError(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	failingWebhooks := &failingWebhookPort{}
	uc := NewUsecases(repo, "", failingWebhooks, nil, nil, nil, "")

	_, err := uc.Run(context.Background(), "retry_webhooks")
	if err == nil {
		t.Fatal("expected error from failing webhook port")
	}
	found := false
	for _, r := range repo.recordedRuns {
		if r.Task == "retry_webhooks" && r.Status == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected error run to be recorded")
	}
}

func TestRunSchedulingHoldsExpiration(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	sched := &fakeScheduling{expired: []schedulingdomain.Booking{{ID: uuid.New()}}}
	uc := NewUsecases(repo, "", nil, nil, sched, nil, "")

	result, err := uc.Run(context.Background(), "scheduling_holds")
	if err != nil {
		t.Fatalf("Run(scheduling_holds) error = %v", err)
	}
	if result.Metadata["scheduling_holds_expired"] != 1 {
		t.Fatalf("expected 1 expired hold, got %v", result.Metadata["scheduling_holds_expired"])
	}
}

type failingWebhookPort struct{}

func (f *failingWebhookPort) RetryPending(_ context.Context) (int, error) {
	return 0, errors.New("webhook retry failed")
}
func (f *failingWebhookPort) CleanupOldDeliveries(_ context.Context, _ int) (int64, error) {
	return 0, nil
}
