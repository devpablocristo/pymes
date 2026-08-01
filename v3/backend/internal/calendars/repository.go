// architecture:adapter repository
package calendars

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/calendars/repository/helpers"
	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/calendars/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (store *Store) LeaseCalendarEvents(
	ctx context.Context,
	limit int,
	duration time.Duration,
) ([]domain.OutboxEvent, error) {
	if limit < 1 || duration <= 0 {
		return nil, nil
	}
	organizations, err := store.organizationIDs(ctx)
	if err != nil {
		return nil, err
	}
	now := store.clock()
	token := uuid.NewString()
	var events []domain.OutboxEvent
	for len(events) < limit {
		leasedThisRound := 0
		for _, organizationID := range organizations {
			if len(events) >= limit {
				break
			}
			tx, beginErr := store.beginOrganization(ctx, organizationID)
			if beginErr != nil {
				return nil, beginErr
			}
			var event domain.OutboxEvent
			queryErr := tx.QueryRow(ctx, `
				WITH candidate AS (
					SELECT id FROM app.outbox
					WHERE org_id=$1
					  AND topic='CalendarSyncRequested'
					  AND published_at IS NULL
					  AND available_at <= $2
					  AND (
						lease_expires_at IS NULL OR
						lease_expires_at <= $2
					  )
					ORDER BY available_at,created_at
					FOR UPDATE SKIP LOCKED LIMIT 1
				)
				UPDATE app.outbox o
				SET lease_token=$3,lease_expires_at=$4,
				    attempts=o.attempts+1
				FROM candidate c WHERE o.id=c.id
				RETURNING o.id::text,o.org_id,o.topic,o.payload,o.attempts,
				          o.lease_token,o.available_at,o.lease_expires_at`,
				organizationID, now, token, now.Add(duration),
			).Scan(
				&event.ID, &event.OrganizationID, &event.Topic,
				&event.Payload, &event.Attempts, &event.LeaseToken,
				&event.AvailableAt, &event.LeaseExpiresAt,
			)
			if errors.Is(queryErr, pgx.ErrNoRows) {
				_ = tx.Rollback(ctx)
				continue
			}
			if queryErr != nil {
				_ = tx.Rollback(ctx)
				return nil, queryErr
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, commitErr
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

func (store *Store) RetryCalendarEvent(
	ctx context.Context,
	event domain.OutboxEvent,
) error {
	attempt := event.Attempts
	if attempt < 1 {
		attempt = 1
	}
	backoff := time.Second * time.Duration(1<<minCalendar(attempt-1, 6))
	if backoff > time.Minute {
		backoff = time.Minute
	}
	digest := sha256.Sum256([]byte(event.ID))
	jitter := time.Duration(
		(int(digest[0])<<8|int(digest[1]))%1000,
	) * time.Millisecond
	tx, err := store.beginOrganization(ctx, event.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		UPDATE app.outbox
		SET available_at=$1,lease_token=NULL,lease_expires_at=NULL
		WHERE id=$2::uuid AND org_id=$3 AND topic='CalendarSyncRequested'
		  AND lease_token=$4`,
		store.clock().Add(backoff+jitter), event.ID,
		event.OrganizationID, event.LeaseToken,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("calendar outbox lease lost")
	}
	return tx.Commit(ctx)
}

func (store *Store) MarkCalendarEventPublished(
	ctx context.Context,
	event domain.OutboxEvent,
) error {
	tx, err := store.beginOrganization(ctx, event.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		UPDATE app.outbox
		SET published_at=$1,lease_token=NULL,lease_expires_at=NULL
		WHERE id=$2::uuid AND org_id=$3 AND topic='CalendarSyncRequested'
		  AND lease_token=$4`,
		store.clock(), event.ID, event.OrganizationID, event.LeaseToken,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("calendar outbox lease lost")
	}
	return tx.Commit(ctx)
}

func (store *Store) DeadLetterCalendarEvent(
	ctx context.Context,
	event domain.OutboxEvent,
	failureCode string,
) error {
	tx, err := store.beginOrganization(ctx, event.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		INSERT INTO app.outbox_dead_letters (
			id,org_id,topic,payload,payload_hash,idempotency_key,
			request_id,actor_ref,source_version,snapshot_digest,
			correlation_id,attempts,failure_code,failed_at
		)
		SELECT id,org_id,topic,payload,payload_hash,idempotency_key,
		       request_id,actor_ref,source_version,snapshot_digest,
		       correlation_id,attempts,$1,$2
		FROM app.outbox
		WHERE id=$3::uuid AND org_id=$4
		  AND topic='CalendarSyncRequested' AND lease_token=$5`,
		failureCode, store.clock(), event.ID,
		event.OrganizationID, event.LeaseToken,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("calendar outbox lease lost")
	}
	result, err = tx.Exec(ctx, `
		DELETE FROM app.outbox
		WHERE id=$1::uuid AND org_id=$2
		  AND topic='CalendarSyncRequested' AND lease_token=$3`,
		event.ID, event.OrganizationID, event.LeaseToken,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("calendar outbox lease lost")
	}
	return tx.Commit(ctx)
}

func minCalendar(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func (store *Store) clock() time.Time {
	if store == nil || store.Now == nil {
		return time.Now().UTC()
	}
	return store.Now().UTC()
}

type Store struct {
	Pool   *pgxpool.Pool
	Cipher SecretCipher
	Now    func() time.Time
}

func NewStore(pool *pgxpool.Pool, cipher SecretCipher) *Store {
	return &Store{Pool: pool, Cipher: cipher}
}

func (store *Store) BeginOAuth(
	ctx context.Context,
	connection domain.Connection,
	state domain.OAuthState,
) error {
	if store == nil || store.Pool == nil || !connection.Valid() ||
		state.OrganizationID != connection.OrganizationID ||
		state.ConnectionID != connection.ID {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	tx, err := store.beginOrganization(ctx, connection.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO app.calendar_connections (
			id,org_id,actor_id,provider,status,calendar_id,time_zone,scopes,
			free_busy_enabled,meet_enabled,access_token_expires_at,version,
			created_at,updated_at
		) VALUES ($1,$2,$3,$4,$5,'',$6,$7,$8,$9,NULL,$10,$11,$11)
		ON CONFLICT (org_id,id) DO UPDATE
		SET actor_id=EXCLUDED.actor_id,status='pending',calendar_id='',
		    time_zone=EXCLUDED.time_zone,scopes=EXCLUDED.scopes,
		    free_busy_enabled=EXCLUDED.free_busy_enabled,
		    meet_enabled=EXCLUDED.meet_enabled,token_envelope=NULL,
		    access_token_expires_at=NULL,
		    version=app.calendar_connections.version+1,
		    updated_at=EXCLUDED.updated_at
		WHERE app.calendar_connections.status IN ('pending','reauth_required','revoked')`,
		connection.ID, connection.OrganizationID, connection.ActorID,
		connection.Provider, connection.Status, connection.TimeZone,
		connection.Scopes, connection.FreeBusyEnabled, connection.MeetEnabled,
		connection.Version, connection.CreatedAt,
	)
	if err != nil {
		return err
	}
	hash, err := hex.DecodeString(state.Hash)
	if err != nil || len(hash) != 32 {
		return fmt.Errorf("invalid OAuth state hash")
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO app.calendar_oauth_states (
			state_hash,org_id,actor_id,connection_id,session_binding,time_zone,
			free_busy_enabled,meet_enabled,expires_at,created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		hash, state.OrganizationID, state.ActorID, state.ConnectionID,
		state.SessionBinding, state.TimeZone, state.FreeBusyEnabled,
		state.MeetEnabled, state.ExpiresAt, state.CreatedAt,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (store *Store) ConsumeOAuthState(
	ctx context.Context,
	organizationID, actorID, sessionBinding, hash string,
	now time.Time,
) (domain.OAuthState, error) {
	decodedHash, err := hex.DecodeString(hash)
	if err != nil || len(decodedHash) != 32 {
		return domain.OAuthState{}, domain.ErrOAuthStateInvalid
	}
	tx, err := store.beginOrganization(ctx, organizationID)
	if err != nil {
		return domain.OAuthState{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var row repositorymodels.OAuthStateRow
	err = tx.QueryRow(ctx, `
		SELECT encode(state_hash,'hex'),org_id,actor_id,connection_id,
		       session_binding,time_zone,free_busy_enabled,meet_enabled,
		       expires_at,consumed_at,created_at
		FROM app.calendar_oauth_states
		WHERE org_id=$1 AND state_hash=$2
		FOR UPDATE`,
		organizationID, decodedHash,
	).Scan(
		&row.Hash, &row.OrganizationID, &row.ActorID, &row.ConnectionID,
		&row.SessionBinding, &row.TimeZone, &row.FreeBusyEnabled,
		&row.MeetEnabled, &row.ExpiresAt, &row.ConsumedAt, &row.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OAuthState{}, domain.ErrOAuthStateInvalid
	}
	if err != nil {
		return domain.OAuthState{}, err
	}
	if row.ActorID != actorID || row.SessionBinding != sessionBinding {
		return domain.OAuthState{}, domain.ErrOAuthStateInvalid
	}
	if row.ConsumedAt != nil {
		return domain.OAuthState{}, domain.ErrOAuthStateConsumed
	}
	if !row.ExpiresAt.After(now) {
		return domain.OAuthState{}, domain.ErrOAuthStateExpired
	}
	if _, err = tx.Exec(ctx, `
		UPDATE app.calendar_oauth_states SET consumed_at=$1
		WHERE org_id=$2 AND state_hash=$3 AND consumed_at IS NULL`,
		now, organizationID, decodedHash,
	); err != nil {
		return domain.OAuthState{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.OAuthState{}, err
	}
	consumed := now
	row.ConsumedAt = &consumed
	return repositoryhelpers.OAuthStateFromRow(row), nil
}

func (store *Store) SaveConnectionGrant(
	ctx context.Context,
	connection domain.Connection,
	grant domain.OAuthGrant,
) error {
	if store == nil || store.Pool == nil || store.Cipher == nil ||
		!connection.Valid() || !grant.Valid() {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	plain, err := repositoryhelpers.EncodeGrant(grant)
	if err != nil {
		return err
	}
	envelope, err := store.Cipher.Seal(
		ctx, connection.OrganizationID, connection.ID, plain,
	)
	if err != nil {
		return err
	}
	tx, err := store.beginOrganization(ctx, connection.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		UPDATE app.calendar_connections
		SET status=$1,calendar_id=$2,time_zone=$3,scopes=$4,
		    free_busy_enabled=$5,meet_enabled=$6,token_envelope=$7,
		    access_token_expires_at=$8,version=$9,updated_at=$10
		WHERE org_id=$11 AND id=$12 AND provider='google'
		  AND version <= $9`,
		connection.Status, connection.CalendarID, connection.TimeZone,
		connection.Scopes, connection.FreeBusyEnabled, connection.MeetEnabled,
		envelope, grant.ExpiresAt, connection.Version, connection.UpdatedAt,
		connection.OrganizationID, connection.ID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrPreconditionFailed
	}
	return tx.Commit(ctx)
}

func (store *Store) GetConnection(
	ctx context.Context,
	organizationID, connectionID string,
) (domain.Connection, domain.OAuthGrant, error) {
	if store == nil || store.Pool == nil || store.Cipher == nil {
		return domain.Connection{}, domain.OAuthGrant{}, fmt.Errorf("repository unavailable")
	}
	tx, err := store.beginOrganization(ctx, organizationID)
	if err != nil {
		return domain.Connection{}, domain.OAuthGrant{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row, err := scanConnection(tx.QueryRow(ctx, `
		SELECT id,org_id,actor_id,provider,status,calendar_id,time_zone,scopes,
		       free_busy_enabled,meet_enabled,COALESCE(token_envelope,''::bytea),
		       COALESCE(access_token_expires_at,'epoch'::timestamptz),
		       version,created_at,updated_at
		FROM app.calendar_connections
		WHERE org_id=$1 AND id=$2`,
		organizationID, connectionID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Connection{}, domain.OAuthGrant{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Connection{}, domain.OAuthGrant{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Connection{}, domain.OAuthGrant{}, err
	}
	connection := repositoryhelpers.ConnectionFromRow(row)
	if len(row.TokenEnvelope) == 0 {
		return connection, domain.OAuthGrant{}, nil
	}
	plain, err := store.Cipher.Open(
		ctx, organizationID, connectionID, row.TokenEnvelope,
	)
	if err != nil {
		return domain.Connection{}, domain.OAuthGrant{}, err
	}
	grant, err := repositoryhelpers.DecodeGrant(plain)
	return connection, grant, err
}

func (store *Store) ListConnections(
	ctx context.Context,
	organizationID string,
) ([]domain.Connection, error) {
	tx, err := store.beginOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT id,org_id,actor_id,provider,status,calendar_id,time_zone,scopes,
		       free_busy_enabled,meet_enabled,''::bytea,
		       COALESCE(access_token_expires_at,'epoch'::timestamptz),
		       version,created_at,updated_at
		FROM app.calendar_connections
		WHERE org_id=$1 ORDER BY created_at,id`,
		organizationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var connections []domain.Connection
	for rows.Next() {
		row, scanErr := scanConnection(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		connections = append(
			connections,
			repositoryhelpers.ConnectionFromRow(row),
		)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return connections, nil
}

func (store *Store) RevokeConnection(
	ctx context.Context,
	organizationID, connectionID string,
	now time.Time,
) error {
	tx, err := store.beginOrganization(ctx, organizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		UPDATE app.calendar_connections
		SET status='revoked',token_envelope=NULL,
		    access_token_expires_at=NULL,version=version+1,updated_at=$1
		WHERE org_id=$2 AND id=$3`,
		now, organizationID, connectionID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (store *Store) ListActiveConnections(
	ctx context.Context,
	organizationID string,
) ([]domain.Connection, error) {
	connections, err := store.ListConnections(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	active := connections[:0]
	for _, connection := range connections {
		if connection.Status == domain.ConnectionActive {
			active = append(active, connection)
		}
	}
	return active, nil
}

func (store *Store) GetExternalEvent(
	ctx context.Context,
	organizationID, connectionID, bookingID string,
) (domain.ExternalEvent, error) {
	tx, err := store.beginOrganization(ctx, organizationID)
	if err != nil {
		return domain.ExternalEvent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var row repositorymodels.ExternalEventRow
	err = tx.QueryRow(ctx, `
		SELECT org_id,connection_id,booking_id,google_event_id,etag,
		       meet_request_id,meet_status,meet_uri,source_version,
		       snapshot_digest,status,last_error_code,created_at,updated_at
		FROM app.external_calendar_events
		WHERE org_id=$1 AND connection_id=$2 AND booking_id=$3`,
		organizationID, connectionID, bookingID,
	).Scan(
		&row.OrganizationID, &row.ConnectionID, &row.BookingID,
		&row.GoogleEventID, &row.ETag, &row.MeetRequestID,
		&row.MeetStatus, &row.MeetURI, &row.SourceVersion,
		&row.SnapshotDigest, &row.Status, &row.LastErrorCode,
		&row.CreatedAt, &row.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ExternalEvent{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ExternalEvent{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.ExternalEvent{}, err
	}
	return repositoryhelpers.ExternalEventFromRow(row), nil
}

func (store *Store) SaveExternalEvent(
	ctx context.Context,
	event domain.ExternalEvent,
	operation, outcome, errorCode string,
	now time.Time,
) error {
	if !event.Valid() {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	tx, err := store.beginOrganization(ctx, event.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO app.external_calendar_events (
			org_id,connection_id,booking_id,google_event_id,etag,
			meet_request_id,meet_status,meet_uri,source_version,
			snapshot_digest,status,last_error_code,created_at,updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13
		)
		ON CONFLICT (org_id,connection_id,booking_id) DO UPDATE
		SET google_event_id=EXCLUDED.google_event_id,
		    etag=EXCLUDED.etag,
		    meet_request_id=EXCLUDED.meet_request_id,
		    meet_status=EXCLUDED.meet_status,
		    meet_uri=EXCLUDED.meet_uri,
		    source_version=EXCLUDED.source_version,
		    snapshot_digest=EXCLUDED.snapshot_digest,
		    status=EXCLUDED.status,
		    last_error_code=EXCLUDED.last_error_code,
		    updated_at=EXCLUDED.updated_at
		WHERE app.external_calendar_events.source_version <=
		      EXCLUDED.source_version`,
		event.OrganizationID, event.ConnectionID, event.BookingID,
		event.GoogleEventID, event.ETag, event.MeetRequestID,
		event.MeetStatus, event.MeetURI, event.SourceVersion,
		event.SnapshotDigest, event.Status, errorCode, now,
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO app.calendar_sync_attempts (
			id,org_id,connection_id,booking_id,operation,source_version,
			snapshot_digest,outcome,error_code,occurred_at
		) VALUES (gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		event.OrganizationID, event.ConnectionID, event.BookingID,
		operation, event.SourceVersion, event.SnapshotDigest,
		outcome, errorCode, now,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (store *Store) MarkConnectionReauthRequired(
	ctx context.Context,
	organizationID, connectionID string,
	now time.Time,
) error {
	tx, err := store.beginOrganization(ctx, organizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		UPDATE app.calendar_connections
		SET status='reauth_required',version=version+1,updated_at=$1
		WHERE org_id=$2 AND id=$3 AND status <> 'revoked'`,
		now, organizationID, connectionID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (store *Store) ListPendingConnections(
	ctx context.Context,
	limit int,
) ([]domain.Connection, error) {
	if limit < 1 {
		return nil, nil
	}
	organizations, err := store.organizationIDs(ctx)
	if err != nil {
		return nil, err
	}
	var result []domain.Connection
	for _, organizationID := range organizations {
		if len(result) >= limit {
			break
		}
		tx, beginErr := store.beginOrganization(ctx, organizationID)
		if beginErr != nil {
			return nil, beginErr
		}
		rows, queryErr := tx.Query(ctx, `
			SELECT id,org_id,actor_id,provider,status,calendar_id,time_zone,
			       scopes,free_busy_enabled,meet_enabled,''::bytea,
			       COALESCE(access_token_expires_at,'epoch'::timestamptz),
			       version,created_at,updated_at
			FROM app.calendar_connections
			WHERE org_id=$1 AND status='pending' AND token_envelope IS NOT NULL
			ORDER BY updated_at,id LIMIT $2`,
			organizationID, limit-len(result),
		)
		if queryErr != nil {
			_ = tx.Rollback(ctx)
			return nil, queryErr
		}
		for rows.Next() {
			row, scanErr := scanConnectionRows(rows)
			if scanErr != nil {
				rows.Close()
				_ = tx.Rollback(ctx)
				return nil, scanErr
			}
			result = append(
				result,
				repositoryhelpers.ConnectionFromRow(row),
			)
		}
		if rows.Err() != nil {
			rows.Close()
			_ = tx.Rollback(ctx)
			return nil, rows.Err()
		}
		rows.Close()
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, commitErr
		}
	}
	return result, nil
}

func (store *Store) ListReconcileEvents(
	ctx context.Context,
	limit int,
) ([]domain.ExternalEvent, error) {
	if limit < 1 {
		return nil, nil
	}
	organizations, err := store.organizationIDs(ctx)
	if err != nil {
		return nil, err
	}
	var result []domain.ExternalEvent
	for _, organizationID := range organizations {
		if len(result) >= limit {
			break
		}
		tx, beginErr := store.beginOrganization(ctx, organizationID)
		if beginErr != nil {
			return nil, beginErr
		}
		rows, queryErr := tx.Query(ctx, `
			SELECT org_id,connection_id,booking_id,google_event_id,etag,
			       meet_request_id,meet_status,meet_uri,source_version,
			       snapshot_digest,status,last_error_code,created_at,updated_at
			FROM app.external_calendar_events
			WHERE org_id=$1 AND status='reconcile'
			ORDER BY updated_at,connection_id,booking_id LIMIT $2`,
			organizationID, limit-len(result),
		)
		if queryErr != nil {
			_ = tx.Rollback(ctx)
			return nil, queryErr
		}
		for rows.Next() {
			row, scanErr := scanExternalEventRows(rows)
			if scanErr != nil {
				rows.Close()
				_ = tx.Rollback(ctx)
				return nil, scanErr
			}
			result = append(
				result,
				repositoryhelpers.ExternalEventFromRow(row),
			)
		}
		if rows.Err() != nil {
			rows.Close()
			_ = tx.Rollback(ctx)
			return nil, rows.Err()
		}
		rows.Close()
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, commitErr
		}
	}
	return result, nil
}

func (store *Store) organizationIDs(
	ctx context.Context,
) ([]string, error) {
	rows, err := store.Pool.Query(
		ctx,
		`SELECT id FROM app.organizations WHERE status <> 'suspended' ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var organizationID string
		if err := rows.Scan(&organizationID); err != nil {
			return nil, err
		}
		result = append(result, organizationID)
	}
	return result, rows.Err()
}

func (store *Store) beginOrganization(
	ctx context.Context,
	organizationID string,
) (pgx.Tx, error) {
	if store == nil || store.Pool == nil || organizationID == "" {
		return nil, fmt.Errorf("organization-scoped repository is required")
	}
	tx, err := store.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(
		ctx,
		"SELECT set_config('app.org_id',$1,true)",
		organizationID,
	); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

func scanConnection(row pgx.Row) (repositorymodels.ConnectionRow, error) {
	var value repositorymodels.ConnectionRow
	err := row.Scan(
		&value.ID, &value.OrganizationID, &value.ActorID, &value.Provider,
		&value.Status, &value.CalendarID, &value.TimeZone, &value.Scopes,
		&value.FreeBusyEnabled, &value.MeetEnabled, &value.TokenEnvelope,
		&value.AccessTokenExpiry, &value.Version, &value.CreatedAt,
		&value.UpdatedAt,
	)
	return value, err
}

func scanConnectionRows(rows pgx.Rows) (repositorymodels.ConnectionRow, error) {
	var value repositorymodels.ConnectionRow
	err := rows.Scan(
		&value.ID, &value.OrganizationID, &value.ActorID, &value.Provider,
		&value.Status, &value.CalendarID, &value.TimeZone, &value.Scopes,
		&value.FreeBusyEnabled, &value.MeetEnabled, &value.TokenEnvelope,
		&value.AccessTokenExpiry, &value.Version, &value.CreatedAt,
		&value.UpdatedAt,
	)
	return value, err
}

func scanExternalEventRows(
	rows pgx.Rows,
) (repositorymodels.ExternalEventRow, error) {
	var value repositorymodels.ExternalEventRow
	err := rows.Scan(
		&value.OrganizationID, &value.ConnectionID, &value.BookingID,
		&value.GoogleEventID, &value.ETag, &value.MeetRequestID,
		&value.MeetStatus, &value.MeetURI, &value.SourceVersion,
		&value.SnapshotDigest, &value.Status, &value.LastErrorCode,
		&value.CreatedAt, &value.UpdatedAt,
	)
	return value, err
}
