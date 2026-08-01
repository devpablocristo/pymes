// Package notifications contains the PostgreSQL notification adapter.
// architecture:adapter repository
package notifications

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/notifications/repository/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/notifications/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const intentColumns = `
id,org_id,kind,aggregate_type,aggregate_id,recipient_e164,template_name,
template_version,locale,variables,body,
COALESCE(delivery_channel,''),COALESCE(sender_identity,''),send_at,status,
COALESCE(external_message_id,''),idempotency_key,correlation_id,request_id,
actor_ref,source_version,snapshot_digest,COALESCE(failure_code,''),
created_at,updated_at`

type Postgres struct {
	pool  *pgxpool.Pool
	Clock func() time.Time
}

func (repository *Postgres) ResolveDeliveryRoute(
	ctx context.Context,
	organizationID string,
) (domain.DeliveryRoute, error) {
	if repository == nil || repository.pool == nil {
		return domain.DeliveryRoute{}, errors.New(
			"notification database is required",
		)
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.DeliveryRoute{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = repositoryhelpers.SetOrganization(
		ctx, tx, organizationID,
	); err != nil {
		return domain.DeliveryRoute{}, err
	}
	var route domain.DeliveryRoute
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(pergo_channel,''),
		       COALESCE(pergo_sender_identity,'')
		FROM app.notification_settings
		WHERE org_id=$1`,
		organizationID,
	).Scan(&route.Channel, &route.SenderIdentity)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DeliveryRoute{}, domain.ErrRouteNotConfigured
	}
	if err != nil {
		return domain.DeliveryRoute{}, err
	}
	if route.Channel == "" || route.SenderIdentity == "" {
		return domain.DeliveryRoute{}, domain.ErrRouteNotConfigured
	}
	if err = route.Validate(); err != nil {
		return domain.DeliveryRoute{}, fmt.Errorf(
			"%w: %v",
			domain.ErrRouteNotConfigured,
			err,
		)
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.DeliveryRoute{}, err
	}
	return route, nil
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool, Clock: time.Now}
}

func (repository *Postgres) Create(
	ctx context.Context,
	intent domain.Intent,
) (domain.Intent, error) {
	return repository.create(ctx, intent, true)
}

func (repository *Postgres) Project(
	ctx context.Context,
	intent domain.Intent,
) (domain.Intent, error) {
	return repository.create(ctx, intent, false)
}

func (repository *Postgres) FindProjected(
	ctx context.Context,
	organizationID string,
	idempotencyKey string,
) (domain.Intent, error) {
	if repository == nil || repository.pool == nil {
		return domain.Intent{}, errors.New("notification database is required")
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Intent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = repositoryhelpers.SetOrganization(
		ctx, tx, organizationID,
	); err != nil {
		return domain.Intent{}, err
	}
	intent, err := repositoryhelpers.ScanIntent(tx.QueryRow(ctx, `
		SELECT `+intentColumns+`
		FROM app.notifications
		WHERE org_id=$1 AND idempotency_key=$2`,
		organizationID,
		idempotencyKey,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Intent{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Intent{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Intent{}, err
	}
	return intent, nil
}

func (repository *Postgres) create(
	ctx context.Context,
	intent domain.Intent,
	enqueue bool,
) (domain.Intent, error) {
	if repository == nil || repository.pool == nil {
		return domain.Intent{}, errors.New("notification database is required")
	}
	variables, err := json.Marshal(intent.Variables)
	if err != nil {
		return domain.Intent{}, fmt.Errorf("encode notification variables: %w", err)
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Intent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = repositoryhelpers.SetOrganization(ctx, tx, intent.OrganizationID); err != nil {
		return domain.Intent{}, err
	}
	var enabled bool
	if err = tx.QueryRow(ctx, `
		SELECT whatsapp_enabled
		FROM app.organization_feature_flags
		WHERE org_id=$1`,
		intent.OrganizationID,
	).Scan(&enabled); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Intent{}, domain.ErrDisabled
		}
		return domain.Intent{}, err
	}
	if !enabled {
		return domain.Intent{}, domain.ErrDisabled
	}
	now := repository.now()
	row := tx.QueryRow(ctx, `
		INSERT INTO app.notifications(
			id,org_id,kind,aggregate_type,aggregate_id,recipient_e164,
			template_name,template_version,locale,variables,body,
			delivery_channel,sender_identity,send_at,status,
			idempotency_key,correlation_id,request_id,actor_ref,source_version,
			snapshot_digest,created_at,updated_at
		)
		VALUES(
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,
			$19,$20,$21,$22,$22
		)
		ON CONFLICT (org_id,idempotency_key) DO NOTHING
		RETURNING `+intentColumns,
		intent.ID, intent.OrganizationID, intent.Kind, intent.AggregateType,
		intent.AggregateID, intent.RecipientE164, intent.TemplateName,
		intent.TemplateVersion, intent.Locale, variables, intent.Body,
		nullIfEmpty(intent.DeliveryChannel), nullIfEmpty(intent.SenderIdentity),
		intent.SendAt.UTC(), intent.Status, intent.IdempotencyKey,
		intent.CorrelationID, intent.RequestID, intent.ActorRef,
		intent.SourceVersion, intent.SnapshotDigest, now,
	)
	stored, scanErr := repositoryhelpers.ScanIntent(row)
	if errors.Is(scanErr, pgx.ErrNoRows) {
		stored, scanErr = repositoryhelpers.ScanIntent(tx.QueryRow(ctx, `
			SELECT `+intentColumns+`
			FROM app.notifications
			WHERE org_id=$1 AND idempotency_key=$2`,
			intent.OrganizationID, intent.IdempotencyKey,
		))
		if scanErr == nil && stored.SnapshotDigest != intent.SnapshotDigest {
			return domain.Intent{}, domain.ErrIdempotencyKeyReused
		}
	}
	if scanErr != nil {
		return domain.Intent{}, scanErr
	}
	if !enqueue {
		if err = tx.Commit(ctx); err != nil {
			return domain.Intent{}, err
		}
		return stored, nil
	}
	payload, err := json.Marshal(map[string]string{"notification_id": stored.ID})
	if err != nil {
		return domain.Intent{}, err
	}
	digest := sha256.Sum256(payload)
	_, err = tx.Exec(ctx, `
		INSERT INTO app.outbox(
			id,org_id,topic,payload,payload_hash,idempotency_key,request_id,
			actor_ref,source_version,snapshot_digest,correlation_id,
			available_at,created_at
		)
		VALUES($1,$2,'NotificationRequested',$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (org_id,topic,idempotency_key) DO NOTHING`,
		uuid.New(), stored.OrganizationID, payload,
		hex.EncodeToString(digest[:]), stored.IdempotencyKey, stored.RequestID,
		stored.ActorRef, stored.SourceVersion, stored.SnapshotDigest,
		stored.CorrelationID, stored.SendAt, now,
	)
	if err != nil {
		return domain.Intent{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Intent{}, err
	}
	return stored, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (repository *Postgres) Get(
	ctx context.Context,
	organizationID string,
	notificationID string,
) (domain.Intent, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Intent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = repositoryhelpers.SetOrganization(ctx, tx, organizationID); err != nil {
		return domain.Intent{}, err
	}
	intent, err := repositoryhelpers.ScanIntent(tx.QueryRow(ctx, `
		SELECT `+intentColumns+`
		FROM app.notifications
		WHERE org_id=$1 AND id=$2`,
		organizationID, notificationID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Intent{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Intent{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Intent{}, err
	}
	return intent, nil
}

func (repository *Postgres) MarkQueued(
	ctx context.Context,
	intent domain.Intent,
	externalMessageID string,
) error {
	return repository.updateDispatchState(
		ctx, intent, domain.StatusQueued, externalMessageID, "",
		[]domain.Status{domain.StatusPending, domain.StatusUncertain},
	)
}

func (repository *Postgres) MarkUncertain(
	ctx context.Context,
	intent domain.Intent,
	failureCode string,
) error {
	return repository.updateDispatchState(
		ctx, intent, domain.StatusUncertain, "", stableFailureCode(failureCode),
		[]domain.Status{domain.StatusPending, domain.StatusUncertain},
	)
}

func (repository *Postgres) MarkFailed(
	ctx context.Context,
	intent domain.Intent,
	failureCode string,
) error {
	return repository.updateDispatchState(
		ctx, intent, domain.StatusFailed, "", stableFailureCode(failureCode),
		[]domain.Status{domain.StatusPending, domain.StatusUncertain, domain.StatusQueued},
	)
}

func (repository *Postgres) updateDispatchState(
	ctx context.Context,
	intent domain.Intent,
	status domain.Status,
	externalMessageID string,
	failureCode string,
	allowed []domain.Status,
) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = repositoryhelpers.SetOrganization(ctx, tx, intent.OrganizationID); err != nil {
		return err
	}
	allowedValues := make([]string, 0, len(allowed))
	for _, value := range allowed {
		allowedValues = append(allowedValues, string(value))
	}
	tag, err := tx.Exec(ctx, `
		UPDATE app.notifications
		SET status=$1,
		    external_message_id=COALESCE(NULLIF($2,''),external_message_id),
		    failure_code=NULLIF($3,''),
		    updated_at=$4
		WHERE org_id=$5 AND id=$6 AND status=ANY($7)`,
		status, externalMessageID, failureCode, repository.now(),
		intent.OrganizationID, intent.ID, allowedValues,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		current, currentErr := repositoryhelpers.ScanIntent(tx.QueryRow(ctx, `
			SELECT `+intentColumns+` FROM app.notifications
			WHERE org_id=$1 AND id=$2`,
			intent.OrganizationID, intent.ID,
		))
		if currentErr != nil {
			return currentErr
		}
		if current.Status != status && !current.TerminalForDispatch() {
			return domain.ErrInvalidTransition
		}
	}
	return tx.Commit(ctx)
}

func (repository *Postgres) ApplyDeliveryEvent(
	ctx context.Context,
	organizationID string,
	event domain.DeliveryEvent,
	payloadHash string,
) (bool, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = repositoryhelpers.SetOrganization(ctx, tx, organizationID); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO app.notification_webhook_inbox(
			org_id,payload_hash,event_type,trace_id,message_id,workspace_id,
			occurred_at,received_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (org_id,payload_hash) DO NOTHING`,
		organizationID, payloadHash, event.Event, event.TraceID,
		event.MessageID, event.WorkspaceID, event.Timestamp, repository.now(),
	)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		if err = tx.Commit(ctx); err != nil {
			return false, err
		}
		return true, nil
	}
	current, err := repositoryhelpers.ScanIntent(tx.QueryRow(ctx, `
		SELECT `+intentColumns+`
		FROM app.notifications
		WHERE org_id=$1 AND id=$2
		FOR UPDATE`,
		organizationID, event.NotificationID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.ErrNotFound
	}
	if err != nil {
		return false, err
	}
	next, transitionErr := domain.NextStatus(current.Status, event.Event)
	if transitionErr == nil {
		failureCode := ""
		if next == domain.StatusFailed {
			failureCode = "PERGO_DELIVERY_FAILED"
		}
		_, err = tx.Exec(ctx, `
			UPDATE app.notifications
			SET status=$1,
			    external_message_id=$2,
			    failure_code=NULLIF($3,''),
			    updated_at=$4
			WHERE org_id=$5 AND id=$6`,
			next, event.MessageID, failureCode, repository.now(),
			organizationID, event.NotificationID,
		)
		if err != nil {
			return false, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return false, nil
}

func (repository *Postgres) LeaseNotifications(
	ctx context.Context,
	limit int,
	duration time.Duration,
) ([]domain.OutboxEvent, error) {
	if repository == nil || repository.pool == nil {
		return nil, errors.New("notification database is required")
	}
	if limit < 1 || duration <= 0 {
		return nil, nil
	}
	rows, err := repository.pool.Query(
		ctx,
		`SELECT id FROM app.organizations
		 WHERE status <> 'suspended'
		 ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	var organizationIDs []string
	for rows.Next() {
		var organizationID string
		if err = rows.Scan(&organizationID); err != nil {
			rows.Close()
			return nil, err
		}
		organizationIDs = append(organizationIDs, organizationID)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	now := repository.now()
	leaseToken := uuid.NewString()
	events := make([]domain.OutboxEvent, 0, limit)
	for len(events) < limit {
		leasedThisRound := 0
		for _, organizationID := range organizationIDs {
			if len(events) >= limit {
				break
			}
			tx, beginErr := repository.pool.BeginTx(ctx, pgx.TxOptions{})
			if beginErr != nil {
				return nil, beginErr
			}
			if beginErr = repositoryhelpers.SetOrganization(
				ctx,
				tx,
				organizationID,
			); beginErr != nil {
				_ = tx.Rollback(ctx)
				return nil, beginErr
			}
			event, leaseErr := repositoryhelpers.ScanOutboxEvent(
				tx.QueryRow(ctx, `
					WITH candidate AS (
					  SELECT id
					  FROM app.outbox
					  WHERE org_id=$1
					    AND topic='NotificationRequested'
					    AND published_at IS NULL
					    AND available_at <= $2
					    AND (
					      lease_expires_at IS NULL
					      OR lease_expires_at <= $2
					    )
					  ORDER BY available_at,created_at
					  FOR UPDATE SKIP LOCKED
					  LIMIT 1
					)
					UPDATE app.outbox value
					SET lease_token=$3,
					    lease_expires_at=$4,
					    attempts=value.attempts+1
					FROM candidate
					WHERE value.id=candidate.id
					RETURNING
					  value.id,value.org_id,value.topic,value.payload,
					  value.payload_hash,value.idempotency_key,
					  value.request_id,value.actor_ref,value.source_version,
					  value.snapshot_digest,value.correlation_id,
					  value.available_at,value.created_at,value.attempts,value.lease_token,
					  value.lease_expires_at`,
					organizationID,
					now,
					leaseToken,
					now.Add(duration),
				),
			)
			if errors.Is(leaseErr, pgx.ErrNoRows) {
				_ = tx.Rollback(ctx)
				continue
			}
			if leaseErr != nil {
				_ = tx.Rollback(ctx)
				return nil, leaseErr
			}
			if leaseErr = tx.Commit(ctx); leaseErr != nil {
				return nil, leaseErr
			}
			events = append(events, event)
			leasedThisRound++
		}
		if leasedThisRound == 0 {
			break
		}
	}
	return events, nil
}

func (repository *Postgres) MarkNotificationPublished(
	ctx context.Context,
	event domain.OutboxEvent,
) error {
	return repository.updateNotificationLease(
		ctx,
		event,
		`UPDATE app.outbox
		 SET published_at=$1,lease_token=NULL,lease_expires_at=NULL
		 WHERE org_id=$2 AND id=$3
		   AND topic='NotificationRequested'
		   AND lease_token=$4`,
		repository.now(),
	)
}

func (repository *Postgres) RetryNotification(
	ctx context.Context,
	event domain.OutboxEvent,
) error {
	attempt := event.Attempts
	if attempt < 1 {
		attempt = 1
	}
	exponent := attempt - 1
	if exponent > 6 {
		exponent = 6
	}
	backoff := time.Second * time.Duration(1<<exponent)
	jitterDigest := sha256.Sum256([]byte(event.ID))
	jitterMillis := (int(jitterDigest[0])<<8 | int(jitterDigest[1])) % 1000
	return repository.updateNotificationLease(
		ctx,
		event,
		`UPDATE app.outbox
		 SET available_at=$1,lease_token=NULL,lease_expires_at=NULL
		 WHERE org_id=$2 AND id=$3
		   AND topic='NotificationRequested'
		   AND lease_token=$4`,
		repository.now().Add(
			backoff+time.Duration(jitterMillis)*time.Millisecond,
		),
	)
}

func (repository *Postgres) updateNotificationLease(
	ctx context.Context,
	event domain.OutboxEvent,
	statement string,
	timestamp time.Time,
) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = repositoryhelpers.SetOrganization(
		ctx,
		tx,
		event.OrganizationID,
	); err != nil {
		return err
	}
	tag, err := tx.Exec(
		ctx,
		statement,
		timestamp,
		event.OrganizationID,
		event.ID,
		event.LeaseToken,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrLeaseLost
	}
	return tx.Commit(ctx)
}

func (repository *Postgres) DeadLetterNotification(
	ctx context.Context,
	event domain.OutboxEvent,
	failureCode string,
) error {
	failureCode = stableFailureCode(failureCode)
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = repositoryhelpers.SetOrganization(
		ctx,
		tx,
		event.OrganizationID,
	); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO app.outbox_dead_letters(
			id,org_id,topic,payload,payload_hash,idempotency_key,
			request_id,actor_ref,source_version,snapshot_digest,
			correlation_id,attempts,failure_code,failed_at
		)
		SELECT
			id,org_id,topic,payload,payload_hash,idempotency_key,
			request_id,actor_ref,source_version,snapshot_digest,
			correlation_id,attempts,$1,$2
		FROM app.outbox
		WHERE org_id=$3 AND id=$4
		  AND topic='NotificationRequested'
		  AND lease_token=$5`,
		failureCode,
		repository.now(),
		event.OrganizationID,
		event.ID,
		event.LeaseToken,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrLeaseLost
	}
	tag, err = tx.Exec(ctx, `
		DELETE FROM app.outbox
		WHERE org_id=$1 AND id=$2
		  AND topic='NotificationRequested'
		  AND lease_token=$3`,
		event.OrganizationID,
		event.ID,
		event.LeaseToken,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrLeaseLost
	}
	return tx.Commit(ctx)
}

func (repository *Postgres) now() time.Time {
	if repository.Clock == nil {
		return time.Now().UTC()
	}
	return repository.Clock().UTC()
}

func stableFailureCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 80 {
		return "PERGO_DELIVERY_FAILED"
	}
	for _, value := range code {
		if !(value == '_' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9') {
			return "PERGO_DELIVERY_FAILED"
		}
	}
	return code
}
