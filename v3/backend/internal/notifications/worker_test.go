package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type workerStore struct {
	intent domain.Intent
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

func TestWorkerRetriesTimeoutBeforeProcessingAndConverges(t *testing.T) {
	store := &workerStore{intent: pendingIntent()}
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
	handled, err := worker.Consume(
		context.Background(), NotificationRequestedTopic, "org-1",
		requestedPayload(),
	)
	if !handled || err == nil || store.intent.Status != domain.StatusUncertain {
		t.Fatalf("first attempt handled=%v err=%v status=%q", handled, err, store.intent.Status)
	}
	_, err = worker.Consume(
		context.Background(), NotificationRequestedTopic, "org-1",
		requestedPayload(),
	)
	if err != nil || store.intent.Status != domain.StatusQueued || attempts != 2 {
		t.Fatalf("recovery err=%v status=%q attempts=%d", err, store.intent.Status, attempts)
	}
}

func TestWorkerLostResponseWebhookPreventsDuplicateSend(t *testing.T) {
	store := &workerStore{intent: pendingIntent()}
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
	_, err := worker.Consume(
		context.Background(), NotificationRequestedTopic, "org-1",
		requestedPayload(),
	)
	if err == nil || store.intent.Status != domain.StatusSent {
		t.Fatalf("lost response err=%v status=%q", err, store.intent.Status)
	}
	handled, err := worker.Consume(
		context.Background(), NotificationRequestedTopic, "org-1",
		requestedPayload(),
	)
	if !handled || err != nil || attempts != 1 {
		t.Fatalf("retry handled=%v err=%v sends=%d", handled, err, attempts)
	}
}

func TestWorkerTerminalProviderFailureDoesNotRetry(t *testing.T) {
	store := &workerStore{intent: pendingIntent()}
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
	handled, err := worker.Consume(
		context.Background(), NotificationRequestedTopic, "org-1",
		requestedPayload(),
	)
	if !handled || err != nil || store.intent.Status != domain.StatusFailed {
		t.Fatalf("handled=%v err=%v status=%q", handled, err, store.intent.Status)
	}
}
