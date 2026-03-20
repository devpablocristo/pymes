package wire

import (
	"reflect"
	"slices"
	"testing"

	saasusers "github.com/devpablocristo/saas-core/users"
)

func TestSaaSDefaultAPIKeyScopesAlignWithSaaSCore(t *testing.T) {
	t.Parallel()

	got := saasDefaultAPIKeyScopes()
	if !reflect.DeepEqual(got, saasusers.DefaultAPIKeyScopes) {
		t.Fatalf("saasDefaultAPIKeyScopes() = %#v, want %#v", got, saasusers.DefaultAPIKeyScopes)
	}
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
	original := saasusers.DefaultAPIKeyScopes[0]
	got[0] = "changed"
	if saasusers.DefaultAPIKeyScopes[0] != original {
		t.Fatalf("saasDefaultAPIKeyScopes() must return a copy; saas-core defaults were mutated to %q", saasusers.DefaultAPIKeyScopes[0])
	}
}
