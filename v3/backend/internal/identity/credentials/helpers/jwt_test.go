package helpers

import (
	"crypto/ed25519"
	"testing"
)

func TestDecodeAndVerifyRejectsInvalidTokenShape(t *testing.T) {
	t.Parallel()
	if _, _, err := DecodeAndVerify("invalid", make(ed25519.PublicKey, ed25519.PublicKeySize)); err == nil {
		t.Fatal("expected invalid JWT shape to fail")
	}
}
