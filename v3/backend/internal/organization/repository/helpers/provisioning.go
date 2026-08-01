// Package helpers contains SQL-safe mappings for organization persistence.
package helpers

import (
	"errors"

	"github.com/devpablocristo/pymes/v3/backend/internal/organization/repository/models"
)

// ProvisioningColumn validates the bounded service/status vocabulary before a
// column name is selected for the query.
func ProvisioningColumn(target models.ProvisioningTarget) (string, error) {
	if target.Status != "pending" && target.Status != "ready" && target.Status != "failed" {
		return "", errors.New("unknown provisioning status")
	}
	switch target.Service {
	case "accounting":
		return "accounting_status", nil
	case "fiscal":
		return "fiscal_status", nil
	default:
		return "", errors.New("unknown provisioning service")
	}
}
