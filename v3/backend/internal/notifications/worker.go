// Package notifications contains the durable notification outbox consumer.
// architecture:adapter worker
package notifications

import (
	"context"
	"fmt"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
	workerhelpers "github.com/devpablocristo/pymes/v3/backend/internal/notifications/worker/helpers"
	workermodels "github.com/devpablocristo/pymes/v3/backend/internal/notifications/worker/models"
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

type SchedulingProjection interface {
	Execute(
		context.Context,
		ProjectionMetadata,
		SchedulingNotification,
	) (domain.Intent, bool, error)
}

type Worker struct {
	Store       NotificationRelayStore
	Provider    DeliveryProvider
	Projection  SchedulingProjection
	LeaseFor    time.Duration
	MaxAttempts int
}

func NewWorker(
	store NotificationRelayStore,
	provider DeliveryProvider,
	projection SchedulingProjection,
) Worker {
	return Worker{
		Store: store, Provider: provider, Projection: projection,
	}
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
	intent, deliver, err := worker.resolveIntent(ctx, outboxEvent, event)
	if err != nil {
		return err
	}
	if !deliver {
		return nil
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

func (worker Worker) resolveIntent(
	ctx context.Context,
	outboxEvent domain.OutboxEvent,
	event workermodels.Requested,
) (domain.Intent, bool, error) {
	if event.Delivery != nil {
		intent, err := worker.Store.Get(
			ctx,
			outboxEvent.OrganizationID,
			event.Delivery.NotificationID,
		)
		return intent, true, err
	}
	if event.Scheduling == nil {
		return domain.Intent{}, false, fmt.Errorf(
			"notification request has no supported payload",
		)
	}
	if worker.Projection == nil {
		return domain.Intent{}, false, fmt.Errorf(
			"scheduling notification projection is not configured",
		)
	}
	projected := event.Scheduling
	return worker.Projection.Execute(
		ctx,
		ProjectionMetadata{
			EventID:        outboxEvent.ID,
			OrganizationID: outboxEvent.OrganizationID,
			IdempotencyKey: outboxEvent.IdempotencyKey,
			CorrelationID:  outboxEvent.CorrelationID,
			RequestID:      outboxEvent.RequestID,
			ActorRef:       outboxEvent.ActorRef,
			SourceVersion:  outboxEvent.SourceVersion,
			OccurredAt:     outboxEvent.CreatedAt,
		},
		SchedulingNotification{
			Trigger:       projected.Trigger,
			AggregateType: projected.AggregateType,
			AggregateID:   projected.AggregateID,
			RecipientE164: projected.RecipientE164,
			CustomerName:  projected.CustomerName,
			ServiceName:   projected.ServiceName,
			StartAt:       projected.StartAt,
			EndAt:         projected.EndAt,
			Timezone:      projected.Timezone,
			Reason:        projected.Reason,
			ActionToken:   projected.ActionToken,
			ActionTokens:  projected.ActionTokens,
			ExpiresAt:     projected.ExpiresAt,
		},
	)
}
