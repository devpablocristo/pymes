package helpers

import "testing"

func TestRequireEventOrganizationRejectsCrossTenantDelivery(t *testing.T) {
	if err := RequireEventOrganization("event", "org-a", "org-b"); err == nil {
		t.Fatal("expected cross-tenant delivery to be rejected")
	}
}
