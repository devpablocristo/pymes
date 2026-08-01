package helpers

import (
	"errors"

	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
	organizationmodels "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/organization/models"
	schedulingdomain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
)

func FromOrganization(value organizationdomain.Organization) organizationmodels.Organization {
	return organizationmodels.Organization{
		ID: value.ID, Slug: value.Slug, Status: string(value.Status),
	}
}

func MapError(err error) error {
	if errors.Is(err, organizationdomain.ErrUnknown) {
		return NotAvailable()
	}
	return err
}

func NotAvailable() error {
	return schedulingdomain.NewError(
		schedulingdomain.CodeNotFound,
		"organization is not available",
	)
}
