package wire

import "testing"

func TestClerkEmailFromClaims(t *testing.T) {
	t.Parallel()
	if got := clerkEmailFromClaims(map[string]any{"email": "  a@b.co  "}); got != "a@b.co" {
		t.Fatalf("email: got %q", got)
	}
	if got := clerkEmailFromClaims(map[string]any{
		"email_addresses": []any{
			map[string]any{"email_address": "x@y.com"},
		},
	}); got != "x@y.com" {
		t.Fatalf("email_addresses: got %q", got)
	}
}

func TestClerkDisplayNameFromClaims(t *testing.T) {
	t.Parallel()
	if got := clerkDisplayNameFromClaims(map[string]any{"name": "Ana"}); got != "Ana" {
		t.Fatalf("name: got %q", got)
	}
	if got := clerkDisplayNameFromClaims(map[string]any{"first_name": "Ana", "last_name": "López"}); got != "Ana López" {
		t.Fatalf("first+last: got %q", got)
	}
}

func TestNormalizeIssuerURL(t *testing.T) {
	t.Parallel()
	if normalizeIssuerURL("https://clerk.example/") != normalizeIssuerURL("https://clerk.example") {
		t.Fatal("issuer normalize mismatch")
	}
}
