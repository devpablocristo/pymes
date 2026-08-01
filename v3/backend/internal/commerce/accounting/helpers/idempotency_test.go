package helpers

import "testing"

func TestProvisioningIdempotencyKeyIsStable(t *testing.T) {
	if ProvisioningIdempotencyKey("org") != ProvisioningIdempotencyKey("org") {
		t.Fatal("expected stable key")
	}
}
