package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) GetAccountingFailure(ctx context.Context, organizationID, failureID string) (domain.AccountingFailure, error) {
	tx, err := beginTenantTransaction(ctx, s, organizationID)
	if err != nil {
		return domain.AccountingFailure{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	value, err := getAccountingFailureTx(ctx, tx, organizationID, failureID, false)
	if err != nil {
		return domain.AccountingFailure{}, err
	}
	return value, tx.Commit(ctx)
}

func getAccountingFailureTx(
	ctx context.Context,
	tx pgx.Tx,
	organizationID, failureID string,
	forUpdate bool,
) (domain.AccountingFailure, error) {
	lock := ""
	if forUpdate {
		lock = " FOR UPDATE"
	}
	var value domain.AccountingFailure
	err := tx.QueryRow(ctx, `
		SELECT id::text,org_id,original_event_id::text,source_kind,source_id,
		       command_kind,command_payload,command_digest,failed_effective_at,
		       status,failure_code,request_id,actor_ref,source_version,
		       snapshot_digest,correlation_id,created_at,updated_at
		FROM app.accounting_failures
		WHERE org_id=$1 AND id=$2`+lock,
		organizationID, failureID,
	).Scan(
		&value.ID, &value.OrganizationID, &value.OriginalEventID,
		&value.SourceKind, &value.SourceID, &value.CommandKind,
		&value.CommandPayload, &value.CommandDigest, &value.FailedEffectiveAt,
		&value.Status, &value.FailureCode, &value.Origin.RequestID,
		&value.Origin.ActorRef, &value.Origin.SourceVersion,
		&value.SnapshotDigest, &value.CorrelationID, &value.CreatedAt,
		&value.UpdatedAt,
	)
	if err != nil {
		return domain.AccountingFailure{}, err
	}
	value.Origin.CorrelationID = value.CorrelationID
	return value, nil
}

func (s *Store) GetAccountingAdjustment(
	ctx context.Context,
	organizationID, adjustmentID string,
) (domain.AccountingAdjustment, error) {
	tx, err := beginTenantTransaction(ctx, s, organizationID)
	if err != nil {
		return domain.AccountingAdjustment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var value domain.AccountingAdjustment
	err = tx.QueryRow(ctx, `
		SELECT id,org_id,failure_id::text,effective_at,reason,status,
		       request_id,actor_ref,source_version,snapshot_digest,
		       correlation_id,created_at,updated_at
		FROM app.accounting_adjustments
		WHERE org_id=$1 AND id=$2`,
		organizationID, adjustmentID,
	).Scan(
		&value.ID, &value.OrganizationID, &value.FailureID,
		&value.EffectiveAt, &value.Reason, &value.Status,
		&value.Origin.RequestID, &value.Origin.ActorRef,
		&value.Origin.SourceVersion, &value.SnapshotDigest,
		&value.CorrelationID, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return domain.AccountingAdjustment{}, err
	}
	value.Origin.CorrelationID = value.CorrelationID
	return value, tx.Commit(ctx)
}

func (s *Store) RequestAccountingAdjustmentIdempotent(
	ctx context.Context,
	command domain.IdempotencyCommand,
	failureID string,
	adjustment domain.AccountingAdjustment,
) (domain.AccountingAdjustment, error) {
	return executeIdempotent(ctx, s, command, func(tx pgx.Tx) (domain.AccountingAdjustment, error) {
		failure, err := getAccountingFailureTx(ctx, tx, command.OrganizationID, failureID, true)
		if err != nil {
			return domain.AccountingAdjustment{}, err
		}
		if failure.Status != "awaiting_adjustment" ||
			failure.FailureCode != domain.ErrPeriodLocked.Error() ||
			adjustment.ID == "" || adjustment.OrganizationID != failure.OrganizationID ||
			adjustment.FailureID != failure.ID || adjustment.EffectiveAt.IsZero() ||
			strings.TrimSpace(adjustment.Reason) == "" || len(adjustment.Reason) > 500 {
			return domain.AccountingAdjustment{}, fmt.Errorf("ACCOUNTING_ADJUSTMENT_NOT_ALLOWED")
		}
		now := s.Now().UTC()
		adjustment.Origin = originFromIdempotencyCommand(adjustment.Origin, command)
		adjustment.CorrelationID = adjustment.Origin.CorrelationID
		adjustment.SnapshotDigest = command.PayloadHash
		adjustment.Status = "pending"
		adjustment.CreatedAt, adjustment.UpdatedAt = now, now
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.accounting_adjustments (
				id,org_id,failure_id,effective_at,reason,status,
				request_id,actor_ref,source_version,snapshot_digest,
				correlation_id,created_at,updated_at
			) VALUES ($1,$2,$3,$4,$5,'pending',$6,$7,$8,$9,$10,$11,$11)`,
			adjustment.ID, adjustment.OrganizationID, adjustment.FailureID,
			adjustment.EffectiveAt.UTC(), strings.TrimSpace(adjustment.Reason),
			adjustment.Origin.RequestID, adjustment.Origin.ActorRef,
			adjustment.Origin.SourceVersion, adjustment.SnapshotDigest,
			adjustment.CorrelationID, now,
		); err != nil {
			return domain.AccountingAdjustment{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.accounting_failures
			SET status='adjustment_pending',updated_at=$3
			WHERE org_id=$1 AND id=$2 AND status='awaiting_adjustment'`,
			failure.OrganizationID, failure.ID, now,
		); err != nil {
			return domain.AccountingAdjustment{}, err
		}
		if err := setAccountingSourceFailureState(
			ctx, tx, failure, domain.AccountingAdjustmentPending, failure.ID,
		); err != nil {
			return domain.AccountingAdjustment{}, err
		}
		payload, err := json.Marshal(map[string]string{
			"adjustment_id": adjustment.ID,
			"failure_id":    failure.ID,
		})
		if err != nil {
			return domain.AccountingAdjustment{}, err
		}
		payloadDigest := sha256.Sum256(payload)
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.outbox (
				id,org_id,topic,payload,payload_hash,idempotency_key,
				request_id,actor_ref,source_version,snapshot_digest,
				correlation_id,available_at,created_at
			) VALUES (
				$1,$2,'AccountingAdjustmentRequested',$3,$4,$5,
				$6,$7,$8,$9,$10,$11,$11
			)`,
			uuid.New(), failure.OrganizationID, payload,
			hex.EncodeToString(payloadDigest[:]),
			internalIdempotencyKey(
				failure.OrganizationID, "accounting.adjust",
				adjustment.ID, adjustment.Origin.SourceVersion,
			),
			adjustment.Origin.RequestID, adjustment.Origin.ActorRef,
			adjustment.Origin.SourceVersion, adjustment.SnapshotDigest,
			adjustment.CorrelationID, now,
		); err != nil {
			return domain.AccountingAdjustment{}, err
		}
		return adjustment, nil
	})
}

func (s *Store) RecordAccountingPeriodLocked(
	ctx context.Context,
	event domain.Event,
	failure domain.AccountingFailure,
) error {
	if failure.ID == "" || failure.ID != event.ID ||
		failure.OrganizationID == "" || failure.OrganizationID != event.OrganizationID ||
		failure.OriginalEventID != event.ID ||
		failure.FailureCode != domain.ErrPeriodLocked.Error() ||
		failure.FailedEffectiveAt.IsZero() || len(failure.CommandPayload) == 0 ||
		failure.CommandDigest == "" || failure.SnapshotDigest == "" {
		return fmt.Errorf("INVALID_ACCOUNTING_FAILURE")
	}
	tx, err := beginTenantTransaction(ctx, s, failure.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := s.Now().UTC()
	tag, err := tx.Exec(ctx, `
		INSERT INTO app.accounting_failures (
			id,org_id,original_event_id,source_kind,source_id,command_kind,
			command_payload,command_digest,failed_effective_at,status,failure_code,
			request_id,actor_ref,source_version,snapshot_digest,correlation_id,
			created_at,updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,'awaiting_adjustment','PERIOD_LOCKED',
			$10,$11,$12,$13,$14,$15,$15
		)
		ON CONFLICT (org_id,original_event_id) DO NOTHING`,
		failure.ID, failure.OrganizationID, failure.OriginalEventID,
		failure.SourceKind, failure.SourceID, failure.CommandKind,
		failure.CommandPayload, failure.CommandDigest, failure.FailedEffectiveAt.UTC(),
		failure.Origin.RequestID, failure.Origin.ActorRef,
		failure.Origin.SourceVersion, failure.SnapshotDigest,
		failure.CorrelationID, now,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		existing, err := getAccountingFailureTx(ctx, tx, failure.OrganizationID, failure.ID, true)
		if err != nil {
			return err
		}
		if existing.SourceKind != failure.SourceKind ||
			existing.SourceID != failure.SourceID ||
			existing.CommandKind != failure.CommandKind ||
			existing.CommandDigest != failure.CommandDigest {
			return fmt.Errorf("ACCOUNTING_FAILURE_COMMAND_MISMATCH")
		}
	}
	if err := setAccountingSourceFailureState(
		ctx, tx, failure, domain.AccountingAdjustmentRequired, failure.ID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) MarkAccountingAdjustmentPeriodLocked(
	ctx context.Context,
	event domain.Event,
	failure domain.AccountingFailure,
	adjustment domain.AccountingAdjustment,
	commandPayload json.RawMessage,
	failedEffectiveAt time.Time,
) error {
	if event.OrganizationID != failure.OrganizationID ||
		adjustment.OrganizationID != failure.OrganizationID ||
		adjustment.FailureID != failure.ID ||
		adjustment.Status != "pending" || failedEffectiveAt.IsZero() ||
		len(commandPayload) == 0 {
		return fmt.Errorf("INVALID_ACCOUNTING_ADJUSTMENT")
	}
	tx, err := beginTenantTransaction(ctx, s, failure.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := s.Now().UTC()
	adjustmentUpdated, err := tx.Exec(ctx, `
		UPDATE app.accounting_adjustments
		SET status='period_locked',updated_at=$3
		WHERE org_id=$1 AND id=$2 AND failure_id=$4 AND status='pending'`,
		failure.OrganizationID, adjustment.ID, now, failure.ID,
	)
	if err != nil {
		return err
	}
	if adjustmentUpdated.RowsAffected() != 1 {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: accounting adjustment")
	}
	failureUpdated, err := tx.Exec(ctx, `
		UPDATE app.accounting_failures
		SET status='awaiting_adjustment',failed_effective_at=$3,updated_at=$4
		WHERE org_id=$1 AND id=$2 AND status='adjustment_pending'`,
		failure.OrganizationID, failure.ID, failedEffectiveAt.UTC(), now,
	)
	if err != nil {
		return err
	}
	if failureUpdated.RowsAffected() != 1 {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: accounting failure")
	}
	failure.FailedEffectiveAt = failedEffectiveAt.UTC()
	if err := setAccountingSourceFailureState(
		ctx, tx, failure, domain.AccountingAdjustmentRequired, failure.ID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func setAccountingSourceFailureState(
	ctx context.Context,
	tx pgx.Tx,
	failure domain.AccountingFailure,
	status, failureID string,
) error {
	var table, allowedStatuses string
	switch failure.SourceKind {
	case "sale":
		table = "app.sales"
		allowedStatuses = "'authorized_pending_posting','accounting_adjustment_required','accounting_adjustment_pending'"
	case "purchase":
		table = "app.purchases"
		allowedStatuses = "'confirmed','accounting_adjustment_required','accounting_adjustment_pending'"
	case "payment":
		table = "app.payments"
		allowedStatuses = "'confirmed','accounting_adjustment_required','accounting_adjustment_pending'"
	case "accounting_application":
		table = "app.accounting_application_commands"
		allowedStatuses = "'pending','accounting_adjustment_required','accounting_adjustment_pending'"
	case "accounting_reversal":
		table = "app.accounting_reversals"
		allowedStatuses = "'requested','accounting_adjustment_required','accounting_adjustment_pending'"
	default:
		return fmt.Errorf("INVALID_ACCOUNTING_FAILURE_SOURCE")
	}
	tag, err := tx.Exec(ctx, `
		UPDATE `+table+`
		SET status=$1,accounting_failure_id=$2,
		    accounting_failure_code='PERIOD_LOCKED',updated_at=now()
		WHERE org_id=$3 AND id=$4
		  AND (
		    accounting_failure_id IS NULL
		    OR accounting_failure_id=$2
		  )
		  AND status IN (`+allowedStatuses+`)`,
		status, failureID, failure.OrganizationID, failure.SourceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: accounting failure source")
	}
	return nil
}

func resolveAccountingFailureTx(
	ctx context.Context,
	tx pgx.Tx,
	organizationID, failureID string,
	now time.Time,
) error {
	if strings.TrimSpace(failureID) == "" {
		return nil
	}
	failureUpdated, err := tx.Exec(ctx, `
		UPDATE app.accounting_failures
		SET status='resolved',resolved_at=$3,updated_at=$3
		WHERE org_id=$1 AND id=$2 AND status='adjustment_pending'`,
		organizationID, failureID, now,
	)
	if err != nil {
		return err
	}
	if failureUpdated.RowsAffected() != 1 {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: accounting failure resolution")
	}
	adjustmentUpdated, err := tx.Exec(ctx, `
		UPDATE app.accounting_adjustments
		SET status='posted',updated_at=$3
		WHERE org_id=$1 AND failure_id=$2 AND status='pending'`,
		organizationID, failureID, now,
	)
	if err != nil {
		return err
	}
	if adjustmentUpdated.RowsAffected() != 1 {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: accounting adjustment resolution")
	}
	return nil
}

func (s *Store) ResumeAccountingReversalAfterAdjustment(
	ctx context.Context,
	failure domain.AccountingFailure,
	adjustment domain.AccountingAdjustment,
) error {
	if failure.CommandKind != "application_reversal" ||
		failure.SourceKind != "accounting_reversal" ||
		adjustment.FailureID != failure.ID {
		return fmt.Errorf("INVALID_ACCOUNTING_ADJUSTMENT")
	}
	tx, err := beginTenantTransaction(ctx, s, failure.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := s.Now().UTC()
	if err := resolveAccountingFailureTx(
		ctx, tx, failure.OrganizationID, failure.ID, now,
	); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE app.accounting_reversals
		SET status='requested',accounting_failure_id=NULL,
		    accounting_failure_code=NULL,updated_at=$3
		WHERE org_id=$1 AND id=$2
		  AND status='accounting_adjustment_pending'
		  AND accounting_failure_id=$4`,
		failure.OrganizationID, failure.SourceID, now, failure.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: accounting reversal resume")
	}
	payload, _ := json.Marshal(map[string]string{"reversal_id": failure.SourceID})
	digest := sha256.Sum256(payload)
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.outbox (
			id,org_id,topic,payload,payload_hash,idempotency_key,
			request_id,actor_ref,source_version,snapshot_digest,
			correlation_id,available_at,created_at
		) VALUES (
			$1,$2,'AccountingReversalRequested',$3,$4,$5,
			$6,$7,$8,$9,$10,$11,$11
		)
		ON CONFLICT (org_id,topic,idempotency_key) DO NOTHING`,
		uuid.New(), failure.OrganizationID, payload,
		hex.EncodeToString(digest[:]),
		internalIdempotencyKey(
			failure.OrganizationID, "accounting.reversal.resume",
			adjustment.ID, adjustment.Origin.SourceVersion,
		),
		adjustment.Origin.RequestID, adjustment.Origin.ActorRef,
		adjustment.Origin.SourceVersion, failure.SnapshotDigest,
		adjustment.CorrelationID, now,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func commandDigest(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
