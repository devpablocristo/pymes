// Package helpers contains envelope validation for the fiscal fake.
package helpers

import (
	"crypto/sha256"
	"encoding/base64"
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

func CredentialID(parts ...string) string {
	digest := sha256.New()
	for _, part := range parts {
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(part))
	}
	return "fcred_" + base64.RawURLEncoding.EncodeToString(digest.Sum(nil)[:18])
}
