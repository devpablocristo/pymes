package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	"github.com/jackc/pgx/v5"
)

var ErrIdempotencyConflict = domain.ErrIdempotencyKeyReused

func beginTenantTransaction(ctx context.Context, store *Store, organizationID string) (pgx.Tx, error) {
	if store == nil || store.Pool == nil {
		return nil, fmt.Errorf("database is not configured")
	}
	tx, err := store.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	var organizationStatus string
	if err = tx.QueryRow(ctx, `SELECT status FROM app.organizations WHERE id=$1`, organizationID).Scan(&organizationStatus); err != nil || organizationStatus != "ready" {
		_ = tx.Rollback(ctx)
		return nil, domain.ErrOrganizationNotReady
	}
	return tx, nil
}

func executeIdempotent[T any](
	ctx context.Context,
	store *Store,
	command domain.IdempotencyCommand,
	action func(pgx.Tx) (T, error),
) (T, error) {
	var zero T
	decodedHash, hashErr := hex.DecodeString(command.PayloadHash)
	trimmedKey := strings.TrimSpace(command.Key)
	if trimmedKey == "" || trimmedKey != command.Key || len(command.Key) > 255 || command.OrganizationID == "" ||
		command.Operation == "" || command.SourceID == "" || command.SourceVersion < 1 ||
		hashErr != nil || len(decodedHash) != sha256.Size {
		return zero, fmt.Errorf("VALIDATION_ERROR")
	}
	tx, err := beginTenantTransaction(ctx, store, command.OrganizationID)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	replayed, response, err := reserveIdempotency(ctx, tx, command)
	if err != nil {
		return zero, err
	}
	if replayed {
		var result T
		if err := json.Unmarshal(response, &result); err != nil {
			return zero, fmt.Errorf("invalid idempotency response: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return zero, err
		}
		return result, nil
	}

	result, err := action(tx)
	if err != nil {
		return zero, err
	}
	response, err = json.Marshal(result)
	if err != nil {
		return zero, err
	}
	completedAt := store.Now().UTC()
	tag, err := tx.Exec(ctx, `
		UPDATE app.idempotency_records
		SET response=$6,completed_at=$7
		WHERE org_id=$1 AND operation=$2 AND source_id=$3 AND source_version=$4
		  AND idempotency_key=$5 AND completed_at IS NULL`,
		command.OrganizationID, command.Operation, command.SourceID, command.SourceVersion,
		command.Key, response, completedAt)
	if err != nil {
		return zero, err
	}
	if tag.RowsAffected() != 1 {
		return zero, fmt.Errorf("idempotency completion lost")
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}
	return result, nil
}

func reserveIdempotency(
	ctx context.Context,
	tx pgx.Tx,
	command domain.IdempotencyCommand,
) (bool, json.RawMessage, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO app.idempotency_records
			(org_id,operation,source_id,source_version,payload_hash,idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT DO NOTHING`,
		command.OrganizationID, command.Operation, command.SourceID, command.SourceVersion,
		command.PayloadHash, command.Key)
	if err != nil {
		return false, nil, err
	}
	if tag.RowsAffected() == 1 {
		return false, nil, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT operation,source_id,source_version,payload_hash,idempotency_key,response,completed_at
		FROM app.idempotency_records
		WHERE org_id=$1
		  AND (
		    (operation=$2 AND source_id=$3 AND source_version=$4)
		    OR (operation=$2 AND idempotency_key=$5)
		  )
		FOR UPDATE`,
		command.OrganizationID, command.Operation, command.SourceID, command.SourceVersion, command.Key)
	if err != nil {
		return false, nil, err
	}
	defer rows.Close()

	type record struct {
		operation, sourceID, payloadHash, key string
		sourceVersion                         int
		response                              json.RawMessage
		completedAt                           *time.Time
	}
	var records []record
	for rows.Next() {
		var value record
		if err := rows.Scan(
			&value.operation, &value.sourceID, &value.sourceVersion, &value.payloadHash,
			&value.key, &value.response, &value.completedAt,
		); err != nil {
			return false, nil, err
		}
		records = append(records, value)
	}
	if err := rows.Err(); err != nil {
		return false, nil, err
	}
	if len(records) != 1 {
		return false, nil, domain.ErrIdempotencyKeyReused
	}
	existing := records[0]
	if existing.operation != command.Operation ||
		existing.sourceID != command.SourceID ||
		existing.sourceVersion != command.SourceVersion ||
		existing.key != command.Key ||
		existing.payloadHash != command.PayloadHash {
		return false, nil, domain.ErrIdempotencyKeyReused
	}
	if existing.completedAt == nil || len(existing.response) == 0 {
		return false, nil, errors.New("IDEMPOTENCY_IN_PROGRESS")
	}
	return true, existing.response, nil
}
