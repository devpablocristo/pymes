// Package companion contains outbound organization adapters.
package companion

import (
	"context"
	"fmt"
	"strings"

	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/domain"
)

// DeferredFiscalProvisioner makes the current mock-only fiscal boundary
// explicit. The real ARCA credential provisioning adapter will replace it.
type DeferredFiscalProvisioner struct{}

func (DeferredFiscalProvisioner) ProvisionOrganization(
	ctx context.Context,
	organization organizationdomain.Organization,
) error {
	if ctx == nil {
		return fmt.Errorf("deferred fiscal provision: context is required")
	}
	if strings.TrimSpace(organization.ID) == "" {
		return fmt.Errorf("deferred fiscal provision: organization ID is required")
	}
	return nil
}
