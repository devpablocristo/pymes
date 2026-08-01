// Package helpers contains envelope validation for the fiscal fake.
package helpers

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/fiscal/models"
)

func MetadataMatches(value models.Metadata) bool {
	return value.PathOrganizationID == value.BodyOrganizationID &&
		value.HeaderIdempotencyKey == value.BodyIdempotencyKey &&
		value.HeaderCorrelationID == value.BodyCorrelationID
}

func ScopedKey(organizationID string, parts ...string) string {
	key := organizationID
	for _, part := range parts {
		key += "\x00" + part
	}
	return key
}

func CertificateFingerprint(certificatePEM string) string {
	sum := sha256.Sum256([]byte(certificatePEM))
	return hex.EncodeToString(sum[:])
}
