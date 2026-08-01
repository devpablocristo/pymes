package helpers

import (
	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
	identitymodels "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/identity/models"
	schedulingdomain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
)

func FromIdentityPrincipal(value identitydomain.Principal) identitymodels.Principal {
	return identitymodels.Principal{
		OrganizationID:     value.OrganizationID,
		ActorID:            value.ActorID,
		Role:               string(value.Role),
		Permissions:        append([]string(nil), value.Permissions...),
		OrganizationStatus: value.OrganizationStatus,
		MembershipStatus:   value.MembershipStatus,
	}
}

func ToSchedulingPrincipal(value identitymodels.Principal) schedulingdomain.Principal {
	return schedulingdomain.Principal{
		OrganizationID:     value.OrganizationID,
		ActorID:            value.ActorID,
		Role:               value.Role,
		Permissions:        append([]string(nil), value.Permissions...),
		OrganizationStatus: value.OrganizationStatus,
		MembershipStatus:   value.MembershipStatus,
	}
}
