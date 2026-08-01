// Package helpers contains accounting-adapter protocol helpers.
package helpers

import (
	"crypto/sha256"
	"encoding/hex"
)

// ProvisioningIdempotencyKey preserves one accounting schema per organization.
func ProvisioningIdempotencyKey(organizationID string) string {
	digest := sha256.Sum256([]byte(organizationID))
	return "provision-org-v1:" + hex.EncodeToString(digest[:])
}
