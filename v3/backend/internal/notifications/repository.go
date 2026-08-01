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
template_version,locale,variables,body,send_at,status,
COALESCE(external_message_id,''),idempotency_key,correlation_id,request_id,
actor_ref,source_version,snapshot_digest,COALESCE(failure_code,''),
created_at,updated_at`

type Postgres struct {
	pool  *pgxpool.Pool
	Clock func() time.Time
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool, Clock: time.Now}
}

func (repository *Postgres) Create(
	ctx context.Context,
	intent domain.Intent,
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
		FROM app.notification_settings
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
			template_name,template_version,locale,variables,body,send_at,status,
			idempotency_key,correlation_id,request_id,actor_ref,source_version,
			snapshot_digest,created_at,updated_at
		)
		VALUES(
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$20
		)
		ON CONFLICT (org_id,idempotency_key) DO NOTHING
		RETURNING `+intentColumns,
		intent.ID, intent.OrganizationID, intent.Kind, intent.AggregateType,
		intent.AggregateID, intent.RecipientE164, intent.TemplateName,
		intent.TemplateVersion, intent.Locale, variables, intent.Body,
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
