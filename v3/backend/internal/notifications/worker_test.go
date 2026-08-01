package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type workerStore struct {
	intent       domain.Intent
	event        domain.OutboxEvent
	published    bool
	retried      bool
	deadLettered bool
	failureCode  string
}

func (store *workerStore) Get(context.Context, string, string) (domain.Intent, error) {
	return store.intent, nil
}
func (store *workerStore) MarkQueued(_ context.Context, _ domain.Intent, external string) error {
	if !store.intent.TerminalForDispatch() {
		store.intent.Status = domain.StatusQueued
		store.intent.ExternalMessageID = external
	}
	return nil
}
func (store *workerStore) MarkUncertain(_ context.Context, _ domain.Intent, code string) error {
	if store.intent.CanDispatch() {
		store.intent.Status = domain.StatusUncertain
		store.intent.FailureCode = code
	}
	return nil
}
func (store *workerStore) MarkFailed(_ context.Context, _ domain.Intent, code string) error {
	store.intent.Status = domain.StatusFailed
	store.intent.FailureCode = code
	return nil
}
func (store *workerStore) LeaseNotifications(
	context.Context,
	int,
	time.Duration,
) ([]domain.OutboxEvent, error) {
	if store.published || store.deadLettered {
		return nil, nil
	}
	store.event.Attempts++
	store.event.LeaseToken = "lease"
	return []domain.OutboxEvent{store.event}, nil
}
func (store *workerStore) RetryNotification(
	_ context.Context,
	event domain.OutboxEvent,
) error {
	if event.ID != store.event.ID {
		return domain.ErrLeaseLost
	}
	store.retried = true
	return nil
}
func (store *workerStore) DeadLetterNotification(
	_ context.Context,
	event domain.OutboxEvent,
	code string,
) error {
	if event.ID != store.event.ID {
		return domain.ErrLeaseLost
	}
	store.deadLettered = true
	store.failureCode = code
	return nil
}
func (store *workerStore) MarkNotificationPublished(
	_ context.Context,
	event domain.OutboxEvent,
) error {
	if event.ID != store.event.ID {
		return domain.ErrLeaseLost
	}
	store.published = true
	return nil
}

type deliveryProvider func(context.Context, domain.Intent) (DeliveryReceipt, error)

func (provider deliveryProvider) Send(
	ctx context.Context,
	intent domain.Intent,
) (DeliveryReceipt, error) {
	return provider(ctx, intent)
}

func requestedPayload() json.RawMessage {
	return json.RawMessage(`{"notification_id":"notification-1"}`)
}

func pendingIntent() domain.Intent {
	return domain.Intent{
		ID: "notification-1", OrganizationID: "org-1",
		Status: domain.StatusPending,
	}
}

func requestedEvent() domain.OutboxEvent {
	return domain.OutboxEvent{
		ID: "outbox-1", OrganizationID: "org-1",
		Topic: NotificationRequestedTopic, Payload: requestedPayload(),
	}
}

func TestWorkerRetriesTimeoutBeforeProcessingAndConverges(t *testing.T) {
	store := &workerStore{
		intent: pendingIntent(),
		event:  requestedEvent(),
	}
	attempts := 0
	worker := Worker{
		Store: store,
		Provider: deliveryProvider(func(
			context.Context,
			domain.Intent,
		) (DeliveryReceipt, error) {
			attempts++
			if attempts == 1 {
				return DeliveryReceipt{}, &ProviderError{
					StableCode: "PERGO_RESPONSE_UNCERTAIN",
					Retry:      true, Unknown: true,
				}
			}
			return DeliveryReceipt{ExternalMessageID: "external-1"}, nil
		}),
	}
	err := worker.DispatchOnce(context.Background())
	if err != nil || !store.retried ||
		store.intent.Status != domain.StatusUncertain {
		t.Fatalf(
			"first attempt err=%v retried=%v status=%q",
			err,
			store.retried,
			store.intent.Status,
		)
	}
	err = worker.DispatchOnce(context.Background())
	if err != nil || !store.published ||
		store.intent.Status != domain.StatusQueued || attempts != 2 {
		t.Fatalf(
			"recovery err=%v published=%v status=%q attempts=%d",
			err,
			store.published,
			store.intent.Status,
			attempts,
		)
	}
}

func TestWorkerLostResponseWebhookPreventsDuplicateSend(t *testing.T) {
	store := &workerStore{
		intent: pendingIntent(),
		event:  requestedEvent(),
	}
	attempts := 0
	worker := Worker{
		Store: store,
		Provider: deliveryProvider(func(
			context.Context,
			domain.Intent,
		) (DeliveryReceipt, error) {
			attempts++
			store.intent.Status = domain.StatusSent
			return DeliveryReceipt{}, &ProviderError{
				StableCode: "PERGO_RESPONSE_UNCERTAIN",
				Retry:      true, Unknown: true,
			}
		}),
	}
	err := worker.DispatchOnce(context.Background())
	if err != nil || !store.retried ||
		store.intent.Status != domain.StatusSent {
		t.Fatalf(
			"lost response err=%v retried=%v status=%q",
			err,
			store.retried,
			store.intent.Status,
		)
	}
	err = worker.DispatchOnce(context.Background())
	if err != nil || !store.published || attempts != 1 {
		t.Fatalf(
			"retry err=%v published=%v sends=%d",
			err,
			store.published,
			attempts,
		)
	}
}

func TestWorkerTerminalProviderFailureDoesNotRetry(t *testing.T) {
	store := &workerStore{
		intent: pendingIntent(),
		event:  requestedEvent(),
	}
	worker := Worker{
		Store: store,
		Provider: deliveryProvider(func(
			context.Context,
			domain.Intent,
		) (DeliveryReceipt, error) {
			return DeliveryReceipt{}, &ProviderError{
				StableCode: "PERGO_REQUEST_REJECTED",
				Cause:      errors.New("terminal"),
			}
		}),
	}
	err := worker.DispatchOnce(context.Background())
	if err != nil || !store.published || store.retried ||
		store.intent.Status != domain.StatusFailed {
		t.Fatalf(
			"err=%v published=%v retried=%v status=%q",
			err,
			store.published,
			store.retried,
			store.intent.Status,
		)
	}
}

func TestWorkerDeadLettersExhaustedRetryWithoutPII(t *testing.T) {
	store := &workerStore{
		intent: pendingIntent(),
		event:  requestedEvent(),
	}
	worker := Worker{
		Store: store,
		Provider: deliveryProvider(func(
			context.Context,
			domain.Intent,
		) (DeliveryReceipt, error) {
			return DeliveryReceipt{}, &ProviderError{
				StableCode: "PERGO_UNAVAILABLE",
				Retry:      true,
			}
		}),
		MaxAttempts: 1,
	}
	if err := worker.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !store.deadLettered ||
		store.failureCode != "PERGO_DELIVERY_FAILED" ||
		store.published {
		t.Fatalf(
			"dead_lettered=%v code=%q published=%v",
			store.deadLettered,
			store.failureCode,
			store.published,
		)
	}
}
