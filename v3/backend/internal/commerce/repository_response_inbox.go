package commerce

import (
	"context"
	"fmt"
	"strings"
	"time"

	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/commerce/repository/helpers"
	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/commerce/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	"github.com/jackc/pgx/v5"
)

type serviceResponseMetadata = repositorymodels.ServiceResponseMetadata

func recordServiceResponse(
	ctx context.Context,
	tx pgx.Tx,
	metadata serviceResponseMetadata,
	response any,
	now time.Time,
) error {
	body, payloadHash, err := repositoryhelpers.ServiceResponsePayload(response)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO app.service_response_inbox (
			org_id,service,operation,request_id,idempotency_key,
			source_version,snapshot_digest,correlation_id,payload_hash,
			response,received_at,applied_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)
		ON CONFLICT DO NOTHING`,
		metadata.OrganizationID,
		metadata.Service,
		metadata.Operation,
		metadata.RequestID,
		metadata.IdempotencyKey,
		metadata.SourceVersion,
		strings.ToLower(metadata.SnapshotDigest),
		metadata.CorrelationID,
		payloadHash,
		body,
		now.UTC(),
	)
	if err != nil {
		return fmt.Errorf("record service response: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	var (
		requestID      string
		idempotencyKey string
		sourceVersion  int
		snapshotDigest string
		correlationID  string
		existingHash   string
	)
	err = tx.QueryRow(ctx, `
		SELECT request_id,idempotency_key,source_version,snapshot_digest,
		       correlation_id,payload_hash
		FROM app.service_response_inbox
		WHERE org_id=$1
		  AND service=$2
		  AND (
		    request_id=$3
		    OR (operation=$4 AND idempotency_key=$5)
		  )
		ORDER BY (request_id=$3) DESC
		LIMIT 1`,
		metadata.OrganizationID,
		metadata.Service,
		metadata.RequestID,
		metadata.Operation,
		metadata.IdempotencyKey,
	).Scan(
		&requestID,
		&idempotencyKey,
		&sourceVersion,
		&snapshotDigest,
		&correlationID,
		&existingHash,
	)
	if err != nil {
		return fmt.Errorf("read service response replay: %w", err)
	}
	if requestID != metadata.RequestID ||
		idempotencyKey != metadata.IdempotencyKey ||
		sourceVersion != metadata.SourceVersion ||
		!strings.EqualFold(snapshotDigest, metadata.SnapshotDigest) ||
		correlationID != metadata.CorrelationID ||
		!strings.EqualFold(existingHash, payloadHash) {
		return domain.ErrIdempotencyKeyReused
	}
	return nil
}

func fiscalResponseMetadata(result domain.FiscalResult) (serviceResponseMetadata, error) {
	operation := ""
	switch {
	case strings.HasPrefix(result.RequestID, "fiscal-authorize:"):
		operation = "authorize"
	case strings.HasPrefix(result.RequestID, "fiscal-consult:"):
		operation = "consult"
	default:
		return serviceResponseMetadata{}, fmt.Errorf("INVALID_SERVICE_RESPONSE: unknown fiscal request")
	}
	return serviceResponseMetadata{
		OrganizationID: result.OrganizationID,
		Service:        "fiscal",
		Operation:      operation,
		RequestID:      result.RequestID,
		IdempotencyKey: result.IdempotencyKey,
		SourceVersion:  result.SourceVersion,
		SnapshotDigest: result.SnapshotDigest,
		CorrelationID:  result.CorrelationID,
	}, nil
}

func accountingResponseMetadata(operation string, result domain.AccountingEvent) serviceResponseMetadata {
	return serviceResponseMetadata{
		OrganizationID: result.OrganizationID,
		Service:        "accounting",
		Operation:      operation,
		RequestID:      result.CommandID,
		IdempotencyKey: result.IdempotencyKey,
		SourceVersion:  result.SourceVersion,
		SnapshotDigest: result.SnapshotDigest,
		CorrelationID:  result.CorrelationID,
	}
}

func serviceResponsePayload(response any) ([]byte, string, error) {
	return repositoryhelpers.ServiceResponsePayload(response)
}
