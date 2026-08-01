// Package helpers contains validation for fiscal provisioning requests.
package helpers

import (
	"fmt"
	"strings"

	"github.com/devpablocristo/pymes/v3/backend/internal/organization/fiscal/models"
)

func ValidateTarget(target models.ProvisioningTarget) error {
	if strings.TrimSpace(target.OrganizationID) == "" {
		return fmt.Errorf("deferred fiscal provision: organization ID is required")
	}
	return nil
}
