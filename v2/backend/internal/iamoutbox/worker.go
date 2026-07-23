package iamoutbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkerConfig struct {
	BatchSize      int
	LeaseDuration  time.Duration
	PublishTimeout time.Duration
	PollInterval   time.Duration
	InitialBackoff time.Duration
	MaximumBackoff time.Duration
}

var provisioningOutboxView = pgx.Identifier{
	"app",
	"organization_provisioning_outbox_messages",
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		BatchSize:      1,
		LeaseDuration:  2 * time.Minute,
		PublishTimeout: 90 * time.Second,
		PollInterval:   time.Second,
		InitialBackoff: time.Second,
		MaximumBackoff: time.Minute,
	}
}

type DispatchObserver func(platformoutbox.DispatchResult, error)

type Worker struct {
	dispatcher   *platformoutbox.Dispatcher
	pollInterval time.Duration
}

func NewWorker(
	store platformoutbox.DeliveryStore,
	publisher platformoutbox.Publisher,
	config WorkerConfig,
) (*Worker, error) {
	if config.PollInterval <= 0 {
		return nil, errors.New("iam outbox: poll interval must be positive")
	}
	if config.PublishTimeout <= 0 || config.PublishTimeout >= config.LeaseDuration {
		return nil, errors.New(
			"iam outbox: publish timeout must be positive and shorter than the lease",
		)
	}
	backoff, err := platformoutbox.NewExponentialBackoff(platformoutbox.ExponentialBackoffConfig{
		Initial:    config.InitialBackoff,
		Maximum:    config.MaximumBackoff,
		Multiplier: 2,
		Jitter:     0.2,
	})
	if err != nil {
		return nil, fmt.Errorf("iam outbox: create retry backoff: %w", err)
	}
	dispatcher, err := platformoutbox.NewDispatcher(
		store,
		timeoutPublisher{next: publisher, timeout: config.PublishTimeout},
		platformoutbox.DispatcherConfig{
			BatchSize:     config.BatchSize,
			LeaseDuration: config.LeaseDuration,
			Backoff:       backoff,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("iam outbox: create dispatcher: %w", err)
	}
	return &Worker{dispatcher: dispatcher, pollInterval: config.PollInterval}, nil
}

type timeoutPublisher struct {
	next    platformoutbox.Publisher
	timeout time.Duration
}

func (publisher timeoutPublisher) Publish(
	ctx context.Context,
	publication platformoutbox.Publication,
) error {
	publishCtx, cancel := context.WithTimeout(ctx, publisher.timeout)
	defer cancel()
	return publisher.next.Publish(publishCtx, publication)
}

func NewPostgresWorker(
	pool *pgxpool.Pool,
	provider *clerk.Client,
	config WorkerConfig,
) (*Worker, error) {
	if pool == nil {
		return nil, errors.New("iam outbox: PostgreSQL pool is required")
	}
	if provider == nil {
		return nil, errors.New("iam outbox: Clerk client is required")
	}
	store, err := platformoutbox.NewStore(pool, platformoutbox.StoreConfig{
		Table:              provisioningOutboxView,
		DefaultMaxAttempts: 12,
	})
	if err != nil {
		return nil, fmt.Errorf("iam outbox: create store: %w", err)
	}
	completion, err := NewPostgresFinalizer(pool)
	if err != nil {
		return nil, err
	}
	processor, err := NewProcessor(provider, completion)
	if err != nil {
		return nil, err
	}
	return NewWorker(store, processor, config)
}

func (worker *Worker) Dispatch(ctx context.Context) (platformoutbox.DispatchResult, error) {
	if worker == nil || worker.dispatcher == nil {
		return platformoutbox.DispatchResult{}, errors.New("iam outbox: worker is not configured")
	}
	return worker.dispatcher.Dispatch(ctx)
}

func (worker *Worker) Run(ctx context.Context, observe DispatchObserver) error {
	if worker == nil || worker.dispatcher == nil || worker.pollInterval <= 0 {
		return errors.New("iam outbox: worker is not configured")
	}
	if observe == nil {
		observe = func(platformoutbox.DispatchResult, error) {}
	}

	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			result, err := worker.Dispatch(ctx)
			if ctx.Err() != nil {
				return nil
			}
			observe(result, err)
			timer.Reset(worker.pollInterval)
		}
	}
}
