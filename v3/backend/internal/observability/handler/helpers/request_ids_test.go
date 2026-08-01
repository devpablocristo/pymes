package helpers

import "testing"

func TestHeaderOrDefaultRejectsWhitespace(t *testing.T) {
	if got := HeaderOrDefault("not valid", "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
}
