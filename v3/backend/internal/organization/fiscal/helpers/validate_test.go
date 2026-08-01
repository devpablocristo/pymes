package helpers

import (
	"testing"

	"github.com/devpablocristo/pymes/v3/backend/internal/organization/fiscal/models"
)

func TestValidateTargetRejectsEmptyOrganization(t *testing.T) {
	if err := ValidateTarget(models.ProvisioningTarget{}); err == nil {
		t.Fatal("expected empty organization to be rejected")
	}
}
