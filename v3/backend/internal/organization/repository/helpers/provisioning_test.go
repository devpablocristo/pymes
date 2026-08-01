package helpers

import (
	"testing"

	"github.com/devpablocristo/pymes/v3/backend/internal/organization/repository/models"
)

func TestProvisioningColumnRejectsUnknownService(t *testing.T) {
	if _, err := ProvisioningColumn(models.ProvisioningTarget{Service: "other", Status: "ready"}); err == nil {
		t.Fatal("expected unknown service to be rejected")
	}
}
