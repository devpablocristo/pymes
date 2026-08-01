// Package models contains key material owned by the KMS signer adapter.
package models

import "crypto/ed25519"

type PublicKeyMaterial struct {
	KeyID     string
	PublicKey ed25519.PublicKey
}
