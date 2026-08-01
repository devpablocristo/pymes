// Package notifications contains Pymes notification application workflows and
// the ports they consume.
package notifications

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
)

type IntentRepository interface {
	Create(context.Context, domain.Intent) (domain.Intent, error)
	Get(context.Context, string, string) (domain.Intent, error)
}

type RequestNotification struct {
	Repository IntentRepository
	Clock      func() time.Time
}

func (usecase RequestNotification) Execute(
	ctx context.Context,
	intent domain.Intent,
) (domain.Intent, error) {
	if ctx == nil {
		return domain.Intent{}, fmt.Errorf("request notification: context is required")
	}
	if usecase.Repository == nil {
		return domain.Intent{}, fmt.Errorf("request notification: repository is required")
	}
	now := time.Now
	if usecase.Clock != nil {
		now = usecase.Clock
	}
	if intent.Status == "" {
		intent.Status = domain.StatusPending
	}
	if intent.SendAt.IsZero() {
		intent.SendAt = now().UTC()
	} else {
		intent.SendAt = intent.SendAt.UTC()
	}
	if err := intent.Validate(); err != nil {
		return domain.Intent{}, err
	}
	digest, err := intent.Digest()
	if err != nil {
		return domain.Intent{}, err
	}
	intent.SnapshotDigest = digest
	return usecase.Repository.Create(ctx, intent)
}

type ReadNotification struct {
	Repository IntentRepository
}

func (usecase ReadNotification) Execute(
	ctx context.Context,
	organizationID string,
	notificationID string,
) (domain.Intent, error) {
	if ctx == nil || usecase.Repository == nil {
		return domain.Intent{}, fmt.Errorf("read notification: dependency is required")
	}
	return usecase.Repository.Get(ctx, organizationID, notificationID)
}

type DeliveryReceipt struct {
	ExternalMessageID string
	Status            string
	QueuedAt          time.Time
}

type DeliveryProvider interface {
	Send(context.Context, domain.Intent) (DeliveryReceipt, error)
}

type DeliveryEventRepository interface {
	ApplyDeliveryEvent(
		context.Context,
		string,
		domain.DeliveryEvent,
		string,
	) (bool, error)
}

type ProcessDeliveryWebhook struct {
	Repository        DeliveryEventRepository
	ExpectedWorkspace string
}

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (usecase ProcessDeliveryWebhook) Execute(
	ctx context.Context,
	organizationID string,
	event domain.DeliveryEvent,
	payloadHash string,
) (bool, error) {
	if ctx == nil || usecase.Repository == nil {
		return false, fmt.Errorf("process delivery webhook: dependency is required")
	}
	if organizationID == "" ||
		event.WorkspaceID != usecase.ExpectedWorkspace ||
		!digestPattern.MatchString(payloadHash) {
		return false, domain.ErrInvalidIntent
	}
	if err := domain.ValidateDeliveryIdentity(
		organizationID,
		event.NotificationID,
	); err != nil {
		return false, err
	}
	if err := domain.ValidateDeliveryEvent(event); err != nil {
		return false, err
	}
	return usecase.Repository.ApplyDeliveryEvent(
		ctx, organizationID, event, payloadHash,
	)
}

type ProviderError struct {
	StableCode string
	Retry      bool
	Unknown    bool
	Cause      error
}

func (err *ProviderError) Error() string {
	if err == nil || err.StableCode == "" {
		return "PERGO_DELIVERY_FAILED"
	}
	return err.StableCode
}

func (err *ProviderError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func AsProviderError(err error) (*ProviderError, bool) {
	var providerError *ProviderError
	return providerError, errors.As(err, &providerError)
}
