package iamsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkerConfig struct {
	BatchSize      int
	LeaseDuration  time.Duration
	PollInterval   time.Duration
	InitialBackoff time.Duration
	MaximumBackoff time.Duration
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		BatchSize:      1,
		LeaseDuration:  30 * time.Second,
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
		return nil, errors.New("iam sync: poll interval must be positive")
	}
	backoff, err := platformoutbox.NewExponentialBackoff(platformoutbox.ExponentialBackoffConfig{
		Initial:    config.InitialBackoff,
		Maximum:    config.MaximumBackoff,
		Multiplier: 2,
		Jitter:     0.2,
	})
	if err != nil {
		return nil, fmt.Errorf("iam sync: create retry backoff: %w", err)
	}
	dispatcher, err := platformoutbox.NewDispatcher(store, publisher, platformoutbox.DispatcherConfig{
		BatchSize:     config.BatchSize,
		LeaseDuration: config.LeaseDuration,
		Backoff:       backoff,
	})
	if err != nil {
		return nil, fmt.Errorf("iam sync: create dispatcher: %w", err)
	}
	return &Worker{dispatcher: dispatcher, pollInterval: config.PollInterval}, nil
}

func NewPostgresWorker(
	pool *pgxpool.Pool,
	provider *clerk.Client,
	config WorkerConfig,
) (*Worker, error) {
	if pool == nil {
		return nil, errors.New("iam sync: PostgreSQL pool is required")
	}
	if provider == nil {
		return nil, errors.New("iam sync: Clerk client is required")
	}
	store, err := newOrderedPostgresStore(pool, 12)
	if err != nil {
		return nil, err
	}
	repository, err := NewPostgresRepository(pool)
	if err != nil {
		return nil, err
	}
	processor, err := NewProcessor(provider, repository)
	if err != nil {
		return nil, err
	}
	return NewWorker(store, processor, config)
}

func (worker *Worker) Dispatch(ctx context.Context) (platformoutbox.DispatchResult, error) {
	if worker == nil || worker.dispatcher == nil {
		return platformoutbox.DispatchResult{}, errors.New("iam sync: worker is not configured")
	}
	return worker.dispatcher.Dispatch(ctx)
}

func (worker *Worker) Run(ctx context.Context, observe DispatchObserver) error {
	if worker == nil || worker.dispatcher == nil || worker.pollInterval <= 0 {
		return errors.New("iam sync: worker is not configured")
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

// orderedPostgresStore leases only post-provisioning IAM topics and keeps
// their global creation order. This avoids cross-replica reordering without a
// product-specific view or schema change. MarkPublished/MarkFailed remain the
// published platform implementation.
type orderedPostgresStore struct {
	pool *pgxpool.Pool
	base *platformoutbox.Store
}

func newOrderedPostgresStore(
	pool *pgxpool.Pool,
	defaultMaxAttempts int,
) (*orderedPostgresStore, error) {
	if pool == nil {
		return nil, errors.New("iam sync: PostgreSQL pool is required")
	}
	base, err := platformoutbox.NewStore(pool, platformoutbox.StoreConfig{
		Table:              pgx.Identifier{"public", platformoutbox.DefaultTableName},
		DefaultMaxAttempts: defaultMaxAttempts,
	})
	if err != nil {
		return nil, fmt.Errorf("iam sync: create platform outbox store: %w", err)
	}
	return &orderedPostgresStore{pool: pool, base: base}, nil
}

func (store *orderedPostgresStore) Lease(
	ctx context.Context,
	request platformoutbox.LeaseRequest,
) ([]platformoutbox.LeasedMessage, error) {
	if store == nil || store.pool == nil || store.base == nil {
		return nil, errors.New("iam sync: ordered store is not configured")
	}
	if request.Limit < 1 || request.Duration <= 0 {
		return nil, errors.New("iam sync: invalid lease request")
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("iam sync: begin lease transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var now time.Time
	if err := tx.QueryRow(ctx, "SELECT statement_timestamp()").Scan(&now); err != nil {
		return nil, fmt.Errorf("iam sync: read database clock: %w", err)
	}
	now = now.UTC()
	leaseToken := uuid.NewString()
	leaseExpiresAt := now.Add(request.Duration)

	row := tx.QueryRow(ctx, `
		WITH candidate AS (
			SELECT message.id
			FROM public.platform_outbox_messages AS message
			WHERE message.topic = ANY($1::text[])
			  AND message.published_at IS NULL
			  AND message.failed_at IS NULL
			  AND message.attempts < message.max_attempts
			  AND message.available_at <= $2
			  AND (
			      message.lease_expires_at IS NULL
			      OR message.lease_expires_at <= $2
			  )
			  AND NOT EXISTS (
			      SELECT 1
			      FROM public.platform_outbox_messages AS predecessor
			      WHERE predecessor.topic = ANY($1::text[])
			        AND predecessor.published_at IS NULL
			        AND predecessor.failed_at IS NULL
			        AND predecessor.attempts < predecessor.max_attempts
			        AND (predecessor.created_at, predecessor.id)
			            < (message.created_at, message.id)
			  )
			ORDER BY message.available_at, message.created_at, message.id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE public.platform_outbox_messages AS message
		SET lease_token = $3,
		    leased_at = $2,
		    lease_expires_at = $4,
		    attempts = message.attempts + 1,
		    last_attempt_at = $2,
		    updated_at = $2
		FROM candidate
		WHERE message.id = candidate.id
		RETURNING
			message.id,
			message.idempotency_key,
			message.topic,
			message.payload,
			message.headers,
			message.available_at,
			message.attempts,
			message.max_attempts,
			message.last_attempt_at,
			message.last_error,
			message.last_error_at,
			message.created_at,
			message.updated_at,
			message.published_at,
			message.failed_at,
			message.lease_token,
			message.leased_at,
			message.lease_expires_at
	`, syncTopics, now, leaseToken, leaseExpiresAt)

	message, found, err := scanLeasedMessage(row)
	if err != nil {
		return nil, fmt.Errorf("iam sync: lease message: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("iam sync: commit lease: %w", err)
	}
	if !found {
		return []platformoutbox.LeasedMessage{}, nil
	}
	return []platformoutbox.LeasedMessage{message}, nil
}

func (store *orderedPostgresStore) MarkPublished(
	ctx context.Context,
	message platformoutbox.LeasedMessage,
) error {
	if store == nil || store.base == nil {
		return errors.New("iam sync: ordered store is not configured")
	}
	return store.base.MarkPublished(ctx, message)
}

func (store *orderedPostgresStore) MarkFailed(
	ctx context.Context,
	message platformoutbox.LeasedMessage,
	failure error,
	retryDelay time.Duration,
) (platformoutbox.FailureDisposition, error) {
	if store == nil || store.base == nil {
		return 0, errors.New("iam sync: ordered store is not configured")
	}
	return store.base.MarkFailed(ctx, message, failure, retryDelay)
}

type rowScanner interface {
	Scan(...any) error
}

func scanLeasedMessage(
	row rowScanner,
) (platformoutbox.LeasedMessage, bool, error) {
	message := platformoutbox.LeasedMessage{}
	var (
		headersRaw []byte
		lastError  *string
	)
	err := row.Scan(
		&message.ID,
		&message.IdempotencyKey,
		&message.Topic,
		&message.Payload,
		&headersRaw,
		&message.AvailableAt,
		&message.Attempts,
		&message.MaxAttempts,
		&message.LastAttemptAt,
		&lastError,
		&message.LastErrorAt,
		&message.CreatedAt,
		&message.UpdatedAt,
		&message.PublishedAt,
		&message.FailedAt,
		&message.LeaseToken,
		&message.LeasedAt,
		&message.LeaseExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return platformoutbox.LeasedMessage{}, false, nil
	}
	if err != nil {
		return platformoutbox.LeasedMessage{}, false, err
	}
	if err := json.Unmarshal(headersRaw, &message.Headers); err != nil {
		return platformoutbox.LeasedMessage{}, false, fmt.Errorf("decode headers: %w", err)
	}
	if lastError != nil {
		message.LastError = strings.TrimSpace(*lastError)
	}
	message.Payload = append([]byte(nil), message.Payload...)
	headers := make(map[string]string, len(message.Headers))
	for key, value := range message.Headers {
		headers[key] = value
	}
	message.Headers = headers
	return message, true, nil
}
