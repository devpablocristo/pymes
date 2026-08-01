// Package organization contains outbound organization adapters.
// architecture:adapter external
package organization

import (
	"context"
	"fmt"

	fiscalhelpers "github.com/devpablocristo/pymes/v3/backend/internal/organization/fiscal/helpers"
	fiscalmodels "github.com/devpablocristo/pymes/v3/backend/internal/organization/fiscal/models"
	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
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
	target := fiscalmodels.ProvisioningTarget{OrganizationID: organization.ID}
	if err := fiscalhelpers.ValidateTarget(target); err != nil {
		return err
	}
	return nil
}
