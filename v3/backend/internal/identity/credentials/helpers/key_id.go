// Package helpers contains deterministic identity encodings for credentials.
package helpers

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
)

// StableKeyID derives the public JWKS identifier used for KMS keys.
func StableKeyID(public ed25519.PublicKey) string {
	digest := sha256.Sum256(public)
	return "ed25519-" + base64.RawURLEncoding.EncodeToString(digest[:])
}
