package helpers

import (
	"crypto/ed25519"
	"testing"
)

func TestStableKeyIDIsDeterministic(t *testing.T) {
	public := make(ed25519.PublicKey, ed25519.PublicKeySize)
	if StableKeyID(public) != StableKeyID(public) {
		t.Fatal("expected stable key ID")
	}
}
