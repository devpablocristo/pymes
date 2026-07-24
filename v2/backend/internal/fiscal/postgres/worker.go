package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repository *Repository) LeaseNext(
	ctx context.Context,
	workerID string,
	now, until time.Time,
) (fiscal.Lease, error) {
	// Worker repositories are constructed with NewTenant. The helper opens and
	// commits a short transaction with app.org_id bound before this query runs;
	// no lease transaction survives into an authority network call.
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || !until.After(now) || until.Sub(now) > 15*time.Minute {
		return fiscal.Lease{}, errors.New("invalid fiscal lease request")
	}
	token := workerID + ":" + uuid.NewString()
	var voucher fiscal.Voucher
	err := repository.withinWorkerTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		var voucherID, organizationID uuid.UUID
		if err := tx.QueryRow(txContext, `
			WITH candidate AS (
				SELECT voucher.org_id, voucher.id
				FROM fiscal.vouchers AS voucher
				WHERE voucher.org_id = app.current_org_id()
				  AND (
					voucher.status IN ('queued', 'uncertain')
					OR (
						voucher.status = 'processing'
						AND voucher.lease_until <= $1
					)
				  )
				  AND (
					voucher.lease_until IS NULL
					OR voucher.lease_until <= $1
				  )
				  AND NOT EXISTS (
					SELECT 1
					FROM fiscal.vouchers AS blocker
					WHERE blocker.org_id = voucher.org_id
					  AND blocker.environment = voucher.environment
					  AND blocker.point_of_sale_id = voucher.point_of_sale_id
					  AND blocker.voucher_type = voucher.voucher_type
					  AND blocker.id <> voucher.id
					  AND blocker.status IN ('processing', 'uncertain')
				  )
				ORDER BY
					CASE voucher.status
						WHEN 'processing' THEN 0
						WHEN 'uncertain' THEN 1
						ELSE 2
					END,
					voucher.created_at,
					voucher.id
				FOR UPDATE SKIP LOCKED
				LIMIT 1
			)
			UPDATE fiscal.vouchers AS voucher
			   SET status = 'processing',
			       lease_owner = $2,
			       lease_until = $3,
			       uncertain_at = CASE
					WHEN voucher.status = 'uncertain' THEN NULL
					ELSE voucher.uncertain_at
			       END,
			       version = voucher.version + 1
			  FROM candidate
			 WHERE voucher.org_id = candidate.org_id
			   AND voucher.id = candidate.id
			RETURNING voucher.org_id, voucher.id`,
			now.UTC(), token, until.UTC(),
		).Scan(&organizationID, &voucherID); err != nil {
			return err
		}
		var err error
		voucher, err = repository.getWithDB(txContext, tx, organizationID, voucherID)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiscal.Lease{}, fiscal.ErrNoWork
	}
	if isBusySeriesError(err) {
		return fiscal.Lease{}, fiscal.ErrNoWork
	}
	if err != nil {
		return fiscal.Lease{}, fmt.Errorf("lease fiscal voucher: %w", mapDatabaseError(err))
	}
	return fiscal.Lease{Voucher: voucher, Token: token, Until: until.UTC()}, nil
}

func (repository *Repository) RenewLease(
	ctx context.Context,
	voucherID uuid.UUID,
	token string,
	until time.Time,
) error {
	return repository.withinWorkerTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		tag, err := tx.Exec(txContext, `
			UPDATE fiscal.vouchers
			   SET lease_until = $3
			 WHERE org_id = app.current_org_id()
			   AND id = $1
			   AND status = 'processing'
			   AND lease_owner = $2
			   AND lease_until > now()`,
			voucherID, token, until.UTC(),
		)
		if err != nil {
			return fmt.Errorf("renew fiscal voucher lease: %w", mapDatabaseError(err))
		}
		if tag.RowsAffected() != 1 {
			return fiscal.ErrLeaseLost
		}
		return nil
	})
}

func (repository *Repository) ReleaseLease(
	ctx context.Context,
	voucherID uuid.UUID,
	token string,
	retryAt time.Time,
	cause string,
) error {
	message := truncate(strings.TrimSpace(cause), 500)
	return repository.withinWorkerTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		tag, err := tx.Exec(txContext, `
			UPDATE fiscal.vouchers
			   SET status = CASE
					WHEN voucher_number IS NULL THEN 'queued'
					ELSE 'uncertain'
			       END,
			       lease_owner = $3,
			       lease_until = $4,
			       last_error_code = 'worker_released',
			       last_error_detail_redacted = $5,
			       uncertain_at = CASE
					WHEN voucher_number IS NULL THEN NULL
					ELSE now()
			       END,
			       version = version + 1
			 WHERE org_id = app.current_org_id()
			   AND id = $1
			   AND status = 'processing'
			   AND lease_owner = $2`,
			voucherID, token, token, retryAt.UTC(), message,
		)
		if err != nil {
			return fmt.Errorf("release fiscal voucher lease: %w", mapDatabaseError(err))
		}
		if tag.RowsAffected() != 1 {
			return fiscal.ErrLeaseLost
		}
		return nil
	})
}

func (repository *Repository) AssignNumber(
	ctx context.Context,
	voucherID uuid.UUID,
	leaseToken string,
	number int64,
	_ time.Time,
) (fiscal.Voucher, error) {
	if number <= 0 {
		return fiscal.Voucher{}, errors.New("fiscal voucher number must be positive")
	}
	var voucher fiscal.Voucher
	err := repository.withinWorkerTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		var organizationID uuid.UUID
		var existing int64
		if err := tx.QueryRow(txContext, `
			SELECT org_id, coalesce(voucher_number, 0)
			FROM fiscal.vouchers
			WHERE org_id = app.current_org_id()
			  AND id = $1
			  AND status IN ('processing', 'uncertain')
			  AND lease_owner = $2
			  AND lease_until > now()
			FOR UPDATE`,
			voucherID, leaseToken,
		).Scan(&organizationID, &existing); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fiscal.ErrLeaseLost
			}
			return err
		}
		if existing != 0 {
			if existing != number {
				return fiscal.ErrSequenceConflict
			}
			var err error
			voucher, err = repository.getWithDB(txContext, tx, organizationID, voucherID)
			return err
		}
		if _, err := tx.Exec(txContext, `
			SELECT fiscal.reserve_voucher_number($1, $2, $3)`,
			organizationID, voucherID, number,
		); err != nil {
			return mapDatabaseError(err)
		}
		var err error
		voucher, err = repository.getWithDB(txContext, tx, organizationID, voucherID)
		return err
	})
	if err != nil {
		return fiscal.Voucher{}, fmt.Errorf("assign fiscal voucher number: %w", err)
	}
	return voucher, nil
}

func (repository *Repository) MarkProcessing(
	ctx context.Context,
	voucherID uuid.UUID,
	leaseToken string,
	_ time.Time,
) (fiscal.Voucher, error) {
	var voucher fiscal.Voucher
	err := repository.withinWorkerTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		var organizationID uuid.UUID
		if err := tx.QueryRow(txContext, `
			UPDATE fiscal.vouchers
			   SET status = CASE
					WHEN status = 'uncertain' THEN 'processing'
					ELSE status
			       END,
			       uncertain_at = CASE
					WHEN status = 'uncertain' THEN NULL
					ELSE uncertain_at
			       END,
			       version = CASE
					WHEN status = 'uncertain' THEN version + 1
					ELSE version
			       END
			 WHERE org_id = app.current_org_id()
			   AND id = $1
			   AND status IN ('processing', 'uncertain')
			   AND lease_owner = $2
			   AND lease_until > now()
			RETURNING org_id`,
			voucherID, leaseToken,
		).Scan(&organizationID); err != nil {
			return err
		}
		var err error
		voucher, err = repository.getWithDB(txContext, tx, organizationID, voucherID)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fiscal.Voucher{}, fiscal.ErrLeaseLost
	}
	if err != nil {
		return fiscal.Voucher{}, fmt.Errorf("mark fiscal voucher processing: %w", mapDatabaseError(err))
	}
	return voucher, nil
}

func (repository *Repository) MarkUncertain(
	ctx context.Context,
	voucherID uuid.UUID,
	leaseToken string,
	failure fiscal.Failure,
) error {
	return repository.withinWorkerTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		tag, err := tx.Exec(txContext, `
			UPDATE fiscal.vouchers
			   SET status = 'uncertain',
			       lease_owner = NULL,
			       lease_until = NULL,
			       last_error_code = $3,
			       last_error_detail_redacted = $4,
			       uncertain_at = $5,
			       version = CASE
					WHEN status = 'uncertain' THEN version
					ELSE version + 1
			       END
			 WHERE org_id = app.current_org_id()
			   AND id = $1
			   AND status IN ('processing', 'uncertain')
			   AND lease_owner = $2
			   AND lease_until > now()`,
			voucherID, leaseToken, truncate(failure.Code, 120),
			truncate(failure.Message, 500), failure.OccurredAt.UTC(),
		)
		if err != nil {
			return fmt.Errorf("mark fiscal voucher uncertain: %w", mapDatabaseError(err))
		}
		if tag.RowsAffected() != 1 {
			return fiscal.ErrLeaseLost
		}
		return nil
	})
}

func (repository *Repository) MarkRejected(
	ctx context.Context,
	voucherID uuid.UUID,
	leaseToken string,
	authorization fiscal.Authorization,
) error {
	if err := authorization.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(authorization)
	if err != nil {
		return fmt.Errorf("encode rejected fiscal authorization: %w", err)
	}
	return repository.withinWorkerTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		tag, err := tx.Exec(txContext, `
			UPDATE fiscal.vouchers
			   SET status = 'rejected',
			       voucher_number = $3,
			       arca_result = $4,
			       response_sha256 = nullif($5, ''),
			       response_storage_ref = nullif($6, ''),
			       lease_owner = NULL,
			       lease_until = NULL,
			       last_error_code = NULL,
			       last_error_detail_redacted = NULL,
			       rejected_at = $7,
			       uncertain_at = NULL,
			       version = version + 1
			 WHERE org_id = app.current_org_id()
			   AND id = $1
			   AND status IN ('processing', 'uncertain')
			   AND lease_owner = $2
			   AND lease_until > now()`,
			voucherID, leaseToken, authorization.Number, string(raw),
			authorization.ResponseHash, authorization.ResponseObject,
			authorization.ProcessedAt.UTC(),
		)
		if err != nil {
			return fmt.Errorf("mark fiscal voucher rejected: %w", mapDatabaseError(err))
		}
		if tag.RowsAffected() != 1 {
			return fiscal.ErrLeaseLost
		}
		return nil
	})
}

func (repository *Repository) FinalizeAuthorized(
	ctx context.Context,
	finalization fiscal.Finalization,
) error {
	if err := finalization.Authorization.Validate(); err != nil {
		return err
	}
	if finalization.Authorization.Decision != fiscal.DecisionAuthorized {
		return errors.New("authorized finalization requires an authorized decision")
	}
	return repository.withinWorkerTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		var organizationID uuid.UUID
		var snapshotHash string
		if err := tx.QueryRow(txContext, `
			SELECT voucher.org_id, snapshot.snapshot_sha256
			FROM fiscal.vouchers AS voucher
			JOIN fiscal.voucher_snapshots AS snapshot
			  ON snapshot.org_id = voucher.org_id
			 AND snapshot.voucher_id = voucher.id
			WHERE voucher.org_id = app.current_org_id()
			  AND voucher.id = $1
			  AND voucher.status IN ('processing', 'uncertain')
			  AND voucher.lease_owner = $2
			  AND voucher.lease_until > now()
			FOR UPDATE OF voucher`,
			finalization.VoucherID, finalization.LeaseToken,
		).Scan(&organizationID, &snapshotHash); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fiscal.ErrLeaseLost
			}
			return fmt.Errorf("lock fiscal voucher for finalization: %w", err)
		}
		if finalization.Posting.OrganizationID != uuid.Nil &&
			finalization.Posting.OrganizationID != organizationID {
			return errors.New("fiscal posting organization does not match voucher tenant")
		}
		if finalization.Posting.SnapshotHash != "" &&
			finalization.Posting.SnapshotHash != snapshotHash {
			return errors.New("fiscal posting snapshot hash does not match immutable snapshot")
		}
		if finalization.Posting.VoucherID != finalization.VoucherID {
			return errors.New("fiscal posting voucher does not match finalization")
		}
		if err := finalization.Posting.Source.Validate(); err != nil {
			return fmt.Errorf("validate fiscal posting source: %w", err)
		}
		if !finalization.Posting.Operation.Valid() {
			return errors.New("fiscal posting operation is invalid")
		}
		if finalization.Posting.AuthorityCode != "" &&
			finalization.Posting.AuthorityCode != finalization.Authorization.Code {
			return errors.New("fiscal posting authority code does not match authorization")
		}
		storedAuthorization := finalization.Authorization
		raw, err := json.Marshal(storedAuthorization)
		if err != nil {
			return fmt.Errorf("encode fiscal authorization: %w", err)
		}
		responseHash := storedAuthorization.ResponseHash
		if responseHash == "" {
			responseHash = hashBytes(raw)
		}
		storedAuthorization.ResponseHash = responseHash
		raw, err = json.Marshal(storedAuthorization)
		if err != nil {
			return fmt.Errorf("encode normalized fiscal authorization: %w", err)
		}
		expiresOn, err := time.Parse("2006-01-02", storedAuthorization.ExpiresOn)
		if err != nil {
			return fmt.Errorf("parse fiscal authorization expiration: %w", err)
		}
		tag, err := tx.Exec(txContext, `
			UPDATE fiscal.vouchers
			   SET status = 'authorized',
			       voucher_number = $3,
			       authorization_code = $4,
			       authorization_expires_at = $5,
			       arca_result = $6,
			       response_sha256 = $7,
			       response_storage_ref = nullif($8, ''),
			       lease_owner = NULL,
			       lease_until = NULL,
			       last_error_code = NULL,
			       last_error_detail_redacted = NULL,
			       authorized_at = $9,
			       rejected_at = NULL,
			       uncertain_at = NULL,
			       version = version + 1
			 WHERE org_id = $1
			   AND id = $2
			   AND status IN ('processing', 'uncertain')`,
			organizationID, finalization.VoucherID,
			storedAuthorization.Number,
			storedAuthorization.Code,
			expiresOn.UTC(),
			string(raw),
			responseHash,
			storedAuthorization.ResponseObject,
			storedAuthorization.ProcessedAt.UTC(),
		)
		if err != nil {
			return fmt.Errorf("authorize fiscal voucher: %w", mapDatabaseError(err))
		}
		if tag.RowsAffected() != 1 {
			return fiscal.ErrLeaseLost
		}
		for _, artifact := range finalization.Artifacts {
			artifactType, err := databaseArtifactType(artifact.Kind)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(txContext, `
				INSERT INTO fiscal.voucher_artifacts (
					org_id, voucher_id, artifact_type, artifact_version,
					storage_ref, content_type, sha256
				)
				VALUES ($1, $2, $3, 1, $4, $5, $6)
				ON CONFLICT (
					org_id, voucher_id, artifact_type, artifact_version
				) DO NOTHING`,
				organizationID, finalization.VoucherID, artifactType,
				artifact.Key, artifact.ContentType, artifact.SHA256,
			); err != nil {
				return fmt.Errorf("persist fiscal voucher artifact %q: %w", artifact.Kind, mapDatabaseError(err))
			}
		}
		if _, err := tx.Exec(txContext, `
			INSERT INTO fiscal.accounting_posting_intents (
				org_id, voucher_id, source_type, source_id, operation,
				snapshot_sha256, authority_code
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			organizationID,
			finalization.VoucherID,
			finalization.Posting.Source.Kind,
			finalization.Posting.Source.ID.String(),
			string(finalization.Posting.Operation),
			snapshotHash,
			storedAuthorization.Code,
		); err != nil {
			return fmt.Errorf("persist fiscal accounting posting intent: %w", mapDatabaseError(err))
		}
		return nil
	})
}

func (repository *Repository) WithinSerial(
	ctx context.Context,
	key fiscal.SerialKey,
	work func(context.Context) error,
) error {
	if key.OrganizationID == uuid.Nil ||
		(key.Environment != "homologation" && key.Environment != "production") ||
		key.PointOfSale <= 0 ||
		key.AuthorityType <= 0 {
		return errors.New("valid fiscal serial key is required")
	}
	if work == nil {
		return errors.New("fiscal serial work is required")
	}
	return repository.withinTransaction(ctx, func(txContext context.Context, tx DBTX) error {
		if err := bindOrganization(txContext, tx, key.OrganizationID); err != nil {
			return err
		}
		pointOfSaleID, err := resolvePointOfSale(
			txContext, tx, key.OrganizationID, key.Environment, key.PointOfSale,
		)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(txContext, `
			SELECT fiscal.lock_voucher_series($1, $2, $3, $4)`,
			key.OrganizationID, key.Environment, pointOfSaleID, key.AuthorityType,
		); err != nil {
			return fmt.Errorf("lock fiscal voucher series: %w", err)
		}
		return work(txContext)
	})
}

func databaseArtifactType(kind string) (string, error) {
	switch strings.TrimSpace(kind) {
	case "pdf":
		return "pdf", nil
	case "qr":
		return "qr", nil
	case "authority_request", "arca_request":
		return "authority_request", nil
	case "authority_response", "arca_response":
		return "authority_response", nil
	default:
		return "", fmt.Errorf("unsupported fiscal artifact kind %q", kind)
	}
}

func truncate(value string, length int) string {
	value = strings.TrimSpace(value)
	if len(value) <= length {
		return value
	}
	return value[:length]
}
