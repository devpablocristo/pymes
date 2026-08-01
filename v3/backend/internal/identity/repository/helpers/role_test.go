package helpers

import "testing"

func TestLocalRoleFallsBackToMember(t *testing.T) {
	if got := LocalRole("org:custom"); got != "member" {
		t.Fatalf("got %q", got)
	}
}
