// Package notifications contains the durable notification outbox consumer.
// architecture:adapter worker
package notifications

import (
	"context"
	"encoding/json"
	"fmt"

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

type Worker struct {
	Store    DeliveryStateStore
	Provider DeliveryProvider
}

func NewWorker(
	store DeliveryStateStore,
	provider DeliveryProvider,
) Worker {
	return Worker{Store: store, Provider: provider}
}

func (Worker) Topics() []string {
	return []string{NotificationRequestedTopic}
}

// Consume is compatible with the outbox consumer port owned by the existing
// relay. It deliberately receives primitives so notifications does not import
// commerce or worker persistence models.
func (worker Worker) Consume(
	ctx context.Context,
	topic string,
	organizationID string,
	payload json.RawMessage,
) (bool, error) {
	if topic != NotificationRequestedTopic {
		return false, nil
	}
	if worker.Store == nil || worker.Provider == nil {
		return true, fmt.Errorf("notification worker dependencies are not configured")
	}
	event, err := workerhelpers.DecodeRequested(payload)
	if err != nil {
		return true, fmt.Errorf("decode notification request: %w", err)
	}
	intent, err := worker.Store.Get(ctx, organizationID, event.NotificationID)
	if err != nil {
		return true, err
	}
	if intent.OrganizationID != organizationID {
		return true, fmt.Errorf("notification organization mismatch")
	}
	if intent.TerminalForDispatch() {
		return true, nil
	}
	if !intent.CanDispatch() {
		return true, domain.ErrInvalidTransition
	}
	receipt, err := worker.Provider.Send(ctx, intent)
	if err == nil {
		if markErr := worker.Store.MarkQueued(
			ctx,
			intent,
			receipt.ExternalMessageID,
		); markErr != nil {
			return true, markErr
		}
		return true, nil
	}
	providerError, known := AsProviderError(err)
	if !known {
		if markErr := worker.Store.MarkUncertain(
			ctx, intent, "PERGO_RESPONSE_UNCERTAIN",
		); markErr != nil {
			return true, markErr
		}
		return true, err
	}
	code := workerhelpers.FailureCode(providerError.StableCode)
	if providerError.Retry || providerError.Unknown {
		if markErr := worker.Store.MarkUncertain(ctx, intent, code); markErr != nil {
			return true, markErr
		}
		return true, err
	}
	if markErr := worker.Store.MarkFailed(ctx, intent, code); markErr != nil {
		return true, markErr
	}
	return true, nil
}
