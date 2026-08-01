package helpers

import "testing"

func TestTokenExpiryRejectsMalformedToken(t *testing.T) {
	if _, err := TokenExpiry("bad"); err == nil {
		t.Fatal("expected malformed token to be rejected")
	}
}
