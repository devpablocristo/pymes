// Package notifications contains the durable notification outbox consumer.
// architecture:adapter worker
package notifications

import (
	"context"
	"fmt"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
	workerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/notifications/worker/helpers"
)

const NotificationRequestedTopic = "NotificationRequested"

type DeliveryStateStore interface {
	Get(context.Context, string, string) (domain.Intent, error)
	MarkQueued(context.Context, domain.Intent, string) error
	MarkUncertain(context.Context, domain.Intent, string) error
	MarkFailed(context.Context, domain.Intent, string) error
}

type NotificationRelayStore interface {
	DeliveryStateStore
	LeaseNotifications(
		context.Context,
		int,
		time.Duration,
	) ([]domain.OutboxEvent, error)
	RetryNotification(context.Context, domain.OutboxEvent) error
	DeadLetterNotification(
		context.Context,
		domain.OutboxEvent,
		string,
	) error
	MarkNotificationPublished(context.Context, domain.OutboxEvent) error
}

type Worker struct {
	Store       NotificationRelayStore
	Provider    DeliveryProvider
	LeaseFor    time.Duration
	MaxAttempts int
}

func NewWorker(
	store NotificationRelayStore,
	provider DeliveryProvider,
) Worker {
	return Worker{Store: store, Provider: provider}
}

func (worker Worker) DispatchOnce(ctx context.Context) error {
	if worker.Store == nil || worker.Provider == nil {
		return fmt.Errorf("notification worker dependencies are not configured")
	}
	leaseFor := worker.LeaseFor
	if leaseFor <= 0 {
		leaseFor = 30 * time.Second
	}
	events, err := worker.Store.LeaseNotifications(ctx, 20, leaseFor)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err = worker.dispatch(ctx, event); err != nil {
			maxAttempts := worker.MaxAttempts
			if maxAttempts <= 0 {
				maxAttempts = 10
			}
			if event.Attempts >= maxAttempts {
				if deadLetterErr := worker.Store.DeadLetterNotification(
					ctx,
					event,
					"PERGO_DELIVERY_FAILED",
				); deadLetterErr != nil {
					return fmt.Errorf(
						"dead-letter notification %s: %w (delivery: %v)",
						event.ID,
						deadLetterErr,
						err,
					)
				}
				continue
			}
			if retryErr := worker.Store.RetryNotification(
				ctx,
				event,
			); retryErr != nil {
				return fmt.Errorf(
					"retry notification %s: %w (delivery: %v)",
					event.ID,
					retryErr,
					err,
				)
			}
			continue
		}
		if err = worker.Store.MarkNotificationPublished(
			ctx,
			event,
		); err != nil {
			return err
		}
	}
	return nil
}

func (worker Worker) dispatch(
	ctx context.Context,
	outboxEvent domain.OutboxEvent,
) error {
	if outboxEvent.Topic != NotificationRequestedTopic {
		return fmt.Errorf("unexpected notification topic %q", outboxEvent.Topic)
	}
	event, err := workerhelpers.DecodeRequested(outboxEvent.Payload)
	if err != nil {
		return fmt.Errorf("decode notification request: %w", err)
	}
	intent, err := worker.Store.Get(
		ctx,
		outboxEvent.OrganizationID,
		event.NotificationID,
	)
	if err != nil {
		return err
	}
	if intent.OrganizationID != outboxEvent.OrganizationID {
		return fmt.Errorf("notification organization mismatch")
	}
	if intent.TerminalForDispatch() {
		return nil
	}
	if !intent.CanDispatch() {
		return domain.ErrInvalidTransition
	}
	receipt, err := worker.Provider.Send(ctx, intent)
	if err == nil {
		if markErr := worker.Store.MarkQueued(
			ctx,
			intent,
			receipt.ExternalMessageID,
		); markErr != nil {
			return markErr
		}
		return nil
	}
	providerError, known := AsProviderError(err)
	if !known {
		if markErr := worker.Store.MarkUncertain(
			ctx, intent, "PERGO_RESPONSE_UNCERTAIN",
		); markErr != nil {
			return markErr
		}
		return err
	}
	code := workerhelpers.FailureCode(providerError.StableCode)
	if providerError.Retry || providerError.Unknown {
		if markErr := worker.Store.MarkUncertain(ctx, intent, code); markErr != nil {
			return markErr
		}
		return err
	}
	if markErr := worker.Store.MarkFailed(ctx, intent, code); markErr != nil {
		return markErr
	}
	return nil
}
