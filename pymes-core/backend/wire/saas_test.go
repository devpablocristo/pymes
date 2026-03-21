package wire

import (
	"slices"
	"testing"
)

func TestSaaSDefaultAPIKeyScopesBaseline(t *testing.T) {
	t.Parallel()

	got := saasDefaultAPIKeyScopes()
	if slices.Contains(got, "admin:full") {
		t.Fatalf("saasDefaultAPIKeyScopes() must not contain deprecated scope admin:full: %#v", got)
	}
	for _, required := range []string{"admin:console:read", "admin:console:write"} {
		if !slices.Contains(got, required) {
			t.Fatalf("saasDefaultAPIKeyScopes() missing required scope %q: %#v", required, got)
		}
	}
}

func TestSaaSDefaultAPIKeyScopesReturnsCopy(t *testing.T) {
	t.Parallel()

	got := saasDefaultAPIKeyScopes()
	if len(got) == 0 {
		t.Fatal("saasDefaultAPIKeyScopes() returned no scopes")
	}
	original := got[0]
	got[0] = "changed"
	if saasDefaultAPIKeyScopes()[0] != original {
		t.Fatalf("saasDefaultAPIKeyScopes() must return a copy; defaults were mutated")
	}
}
