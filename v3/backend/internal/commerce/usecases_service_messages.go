package commerce

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

func internalIdempotencyKey(organizationID, operation, sourceID string, sourceVersion int) string {
	identity := fmt.Sprintf("%s\x00%s\x00%s\x00%d", organizationID, operation, sourceID, sourceVersion)
	digest := sha256.Sum256([]byte(identity))
	return "pymes-v3:" + hex.EncodeToString(digest[:])
}

func commandSnapshotDigest(value any) string {
	body, _ := json.Marshal(value)
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func validateFiscalResult(request domain.FiscalRequest, result domain.FiscalResult) error {
	if result.RequestID != request.RequestID ||
		result.OrganizationID != request.OrganizationID ||
		result.IdempotencyKey != request.IdempotencyKey ||
		result.SourceVersion != request.SourceVersion ||
		result.SnapshotDigest != request.SnapshotDigest ||
		result.CorrelationID != request.CorrelationID {
		return fmt.Errorf("INVALID_SERVICE_RESPONSE: fiscal metadata mismatch")
	}
	return nil
}

func validateAccountingEvent(
	result domain.AccountingEvent,
	commandID, organizationID, idempotencyKey string,
	sourceVersion int,
	snapshotDigest, correlationID string,
) error {
	if result.EventID == "" ||
		result.CommandID != commandID ||
		result.OrganizationID != organizationID ||
		result.IdempotencyKey != idempotencyKey ||
		result.SourceVersion != sourceVersion ||
		result.SnapshotDigest != snapshotDigest ||
		result.CorrelationID != correlationID {
		return fmt.Errorf("INVALID_SERVICE_RESPONSE: accounting metadata mismatch")
	}
	return nil
}
