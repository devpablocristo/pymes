package domain

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRoutedOAuthStateRoundTripsOrganizationHint(t *testing.T) {
	state, err := RoutedOAuthState(
		"org_local_123",
		bytes.Repeat([]byte{0x42}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(state, "org_local_123") {
		t.Fatalf("organization leaked in clear text: %q", state)
	}
	organizationID, err := OrganizationFromOAuthState(state)
	if err != nil {
		t.Fatal(err)
	}
	if organizationID != "org_local_123" {
		t.Fatalf("organization = %q", organizationID)
	}
}

func TestOrganizationFromOAuthStateRejectsMalformedHints(t *testing.T) {
	for _, state := range []string{
		"",
		"opaque",
		".entropy",
		"organization.",
		"***.***",
	} {
		if _, err := OrganizationFromOAuthState(state); !errors.Is(
			err, ErrOAuthStateInvalid,
		) {
			t.Fatalf("state %q error = %v", state, err)
		}
	}
}
