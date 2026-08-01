package helpers

import "testing"

func TestParseKindRejectsUnknownService(t *testing.T) {
	if _, err := ParseKind("unknown"); err == nil {
		t.Fatal("expected unknown service to be rejected")
	}
}
