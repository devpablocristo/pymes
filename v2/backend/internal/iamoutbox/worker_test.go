package iamoutbox

import (
	"context"
	"errors"
	"testing"
	"time"

	platformoutbox "github.com/devpablocristo/platform/outbox/go"
)

func TestWorkerUsesPlatformDispatcherForSuccessfulPublication(t *testing.T) {
	message := leasedMessageFixture()
	store := &recordingDeliveryStore{leased: []platformoutbox.LeasedMessage{message}}
	published := make([]platformoutbox.Publication, 0, 1)
	publisher := platformoutbox.PublisherFunc(func(
		_ context.Context,
		publication platformoutbox.Publication,
	) error {
		published = append(published, publication)
		return nil
	})
	worker, err := NewWorker(store, publisher, DefaultWorkerConfig())
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}

	result, err := worker.Dispatch(context.Background())
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if result != (platformoutbox.DispatchResult{Leased: 1, Published: 1}) {
		t.Fatalf("result = %#v", result)
	}
	if len(published) != 1 ||
		published[0].MessageID != message.ID ||
		published[0].IdempotencyKey != message.IdempotencyKey {
		t.Fatalf("publications = %#v", published)
	}
	if len(store.markedPublished) != 1 || len(store.markedFailed) != 0 {
		t.Fatalf(
			"transitions = published:%d failed:%d",
			len(store.markedPublished),
			len(store.markedFailed),
		)
	}
}

func TestWorkerReturnsProcessorFailureAfterSchedulingRetry(t *testing.T) {
	message := leasedMessageFixture()
	store := &recordingDeliveryStore{leased: []platformoutbox.LeasedMessage{message}}
	publishFailure := errors.New("provider unavailable")
	worker, err := NewWorker(
		store,
		platformoutbox.PublisherFunc(func(
			context.Context,
			platformoutbox.Publication,
		) error {
			return publishFailure
		}),
		DefaultWorkerConfig(),
	)
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}

	result, err := worker.Dispatch(context.Background())
	if !errors.Is(err, publishFailure) {
		t.Fatalf("Dispatch() error = %v, want publisher failure", err)
	}
	if result != (platformoutbox.DispatchResult{Leased: 1, Retried: 1}) {
		t.Fatalf("result = %#v", result)
	}
	if len(store.markedFailed) != 1 || len(store.markedPublished) != 0 {
		t.Fatalf(
			"transitions = published:%d failed:%d",
			len(store.markedPublished),
			len(store.markedFailed),
		)
	}
}

func TestWorkerRunStopsCleanlyWhenContextIsCancelled(t *testing.T) {
	store := &recordingDeliveryStore{}
	worker, err := NewWorker(
		store,
		platformoutbox.PublisherFunc(func(
			context.Context,
			platformoutbox.Publication,
		) error {
			return nil
		}),
		WorkerConfig{
			BatchSize:      1,
			LeaseDuration:  time.Second,
			PublishTimeout: 500 * time.Millisecond,
			PollInterval:   time.Millisecond,
			InitialBackoff: time.Millisecond,
			MaximumBackoff: time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := worker.Run(ctx, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestWorkerCancelsPublicationBeforeLeaseExpires(t *testing.T) {
	message := leasedMessageFixture()
	store := &recordingDeliveryStore{leased: []platformoutbox.LeasedMessage{message}}
	config := DefaultWorkerConfig()
	config.PublishTimeout = 10 * time.Millisecond
	config.LeaseDuration = time.Second
	worker, err := NewWorker(
		store,
		platformoutbox.PublisherFunc(func(
			ctx context.Context,
			_ platformoutbox.Publication,
		) error {
			<-ctx.Done()
			return ctx.Err()
		}),
		config,
	)
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}

	result, err := worker.Dispatch(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Dispatch() error = %v, want deadline", err)
	}
	if result.Retried != 1 || len(store.markedFailed) != 1 {
		t.Fatalf("result/transitions = %#v/%d", result, len(store.markedFailed))
	}
}

func TestWorkerRejectsPublishTimeoutOutsideLease(t *testing.T) {
	config := DefaultWorkerConfig()
	config.PublishTimeout = config.LeaseDuration
	_, err := NewWorker(
		&recordingDeliveryStore{},
		platformoutbox.PublisherFunc(func(
			context.Context,
			platformoutbox.Publication,
		) error {
			return nil
		}),
		config,
	)
	if err == nil {
		t.Fatal("NewWorker() accepted a publish timeout equal to its lease")
	}
}

type failedTransition struct {
	message platformoutbox.LeasedMessage
	err     error
	delay   time.Duration
}

type recordingDeliveryStore struct {
	leased          []platformoutbox.LeasedMessage
	leaseErr        error
	markedPublished []platformoutbox.LeasedMessage
	markedFailed    []failedTransition
}

func (store *recordingDeliveryStore) Lease(
	context.Context,
	platformoutbox.LeaseRequest,
) ([]platformoutbox.LeasedMessage, error) {
	return append([]platformoutbox.LeasedMessage(nil), store.leased...), store.leaseErr
}

func (store *recordingDeliveryStore) MarkPublished(
	_ context.Context,
	message platformoutbox.LeasedMessage,
) error {
	store.markedPublished = append(store.markedPublished, message)
	return nil
}

func (store *recordingDeliveryStore) MarkFailed(
	_ context.Context,
	message platformoutbox.LeasedMessage,
	failure error,
	delay time.Duration,
) (platformoutbox.FailureDisposition, error) {
	store.markedFailed = append(store.markedFailed, failedTransition{
		message: message,
		err:     failure,
		delay:   delay,
	})
	return platformoutbox.FailureRetryScheduled, nil
}

func leasedMessageFixture() platformoutbox.LeasedMessage {
	now := time.Now().UTC()
	return platformoutbox.LeasedMessage{
		Message: platformoutbox.Message{
			ID:             "message-1",
			IdempotencyKey: "iam.provision-org:acme-argentina",
			Topic:          ProvisionOrganizationTopic,
			Payload:        []byte(`{}`),
			Headers:        map[string]string{"content-type": "application/json"},
			AvailableAt:    now,
			Attempts:       1,
			MaxAttempts:    12,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		LeaseToken:     "lease-1",
		LeasedAt:       now,
		LeaseExpiresAt: now.Add(time.Minute),
	}
}
