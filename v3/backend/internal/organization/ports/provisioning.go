// Package ports defines the organization application's outbound boundaries.
package ports

import (
	"context"

	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/domain"
)

type Directory interface {
	SyncClerk(context.Context, string, organizationdomain.Organization) error
	SetProvisioningStatus(context.Context, string, string, string, string) error
	SetStatus(context.Context, string, organizationdomain.Status) error
}

type Provisioner interface {
	ProvisionOrganization(context.Context, organizationdomain.Organization) error
}
