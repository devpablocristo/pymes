package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

func NullableText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func NormalizeOrigin(
	origin domain.OriginMetadata,
	fallbackCorrelationID, operation, sourceID string,
) domain.OriginMetadata {
	origin.RequestID = strings.TrimSpace(origin.RequestID)
	origin.CorrelationID = strings.TrimSpace(origin.CorrelationID)
	origin.ActorRef = strings.TrimSpace(origin.ActorRef)
	if origin.SourceVersion < 1 {
		origin.SourceVersion = 1
	}
	if origin.RequestID == "" {
		identity := fmt.Sprintf("%s\x00%s", strings.TrimSpace(operation), strings.TrimSpace(sourceID))
		digest := sha256.Sum256([]byte(identity))
		origin.RequestID = "internal:" + hex.EncodeToString(digest[:16])
	}
	if origin.CorrelationID == "" {
		origin.CorrelationID = strings.TrimSpace(fallbackCorrelationID)
	}
	if origin.CorrelationID == "" {
		origin.CorrelationID = origin.RequestID
	}
	if origin.ActorRef == "" {
		origin.ActorRef = "system:internal"
	}
	return origin
}

func OriginFromIdempotencyCommand(
	current domain.OriginMetadata,
	command domain.IdempotencyCommand,
) domain.OriginMetadata {
	if strings.TrimSpace(command.RequestID) != "" {
		current.RequestID = command.RequestID
	}
	if strings.TrimSpace(command.CorrelationID) != "" {
		current.CorrelationID = command.CorrelationID
	}
	if strings.TrimSpace(command.ActorRef) != "" {
		current.ActorRef = command.ActorRef
	}
	if command.SourceVersion > 0 {
		current.SourceVersion = command.SourceVersion
	}
	return NormalizeOrigin(
		current,
		command.CorrelationID,
		command.Operation,
		command.SourceID,
	)
}

func OriginSourceVersion(origin domain.OriginMetadata) int {
	if origin.SourceVersion > 0 {
		return origin.SourceVersion
	}
	return 1
}

func IdempotencyKey(organizationID, operation, sourceID string, sourceVersion int) string {
	identity := fmt.Sprintf("%s\x00%s\x00%s\x00%d", organizationID, operation, sourceID, sourceVersion)
	digest := sha256.Sum256([]byte(identity))
	return "pymes-v3:" + hex.EncodeToString(digest[:])
}

func Min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
