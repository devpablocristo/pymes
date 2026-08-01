// architecture:adapter external
package scheduling

import (
	"context"

	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
	organizationhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/organization/helpers"
)

// PublicOrganizationLookup is the consumer-owned boundary towards the
// Organization use case. No repository is shared across contexts.
type PublicOrganizationLookup interface {
	ResolvePublicBySlug(context.Context, string) (organizationdomain.Organization, error)
}

type OrganizationDirectoryAdapter struct {
	lookup PublicOrganizationLookup
}

func NewOrganizationDirectoryAdapter(
	lookup PublicOrganizationLookup,
) *OrganizationDirectoryAdapter {
	return &OrganizationDirectoryAdapter{lookup: lookup}
}

func (a *OrganizationDirectoryAdapter) ResolvePublicOrganization(
	ctx context.Context,
	slug string,
) (string, error) {
	value, err := a.lookup.ResolvePublicBySlug(ctx, slug)
	if err != nil {
		return "", organizationhelpers.MapError(err)
	}
	result := organizationhelpers.FromOrganization(value)
	if result.ID == "" || result.Status != "ready" {
		return "", organizationhelpers.NotAvailable()
	}
	return result.ID, nil
}
