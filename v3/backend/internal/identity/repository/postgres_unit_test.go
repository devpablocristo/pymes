package repository

import "testing"

func TestLocalRoleFailsClosedToMember(t *testing.T) {
	for input, want := range map[string]string{"org:owner": "owner", "org:admin": "admin", "org:viewer": "viewer", "org:billing": "member", "": "member"} {
		if got := localRole(input); got != want {
			t.Fatalf("%q got %q want %q", input, got, want)
		}
	}
}
