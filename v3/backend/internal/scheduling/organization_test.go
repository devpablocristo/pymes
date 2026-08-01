package scheduling

import (
	"context"
	"testing"

	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
	schedulingdomain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
)

type publicOrganizationLookupFake struct {
	organization organizationdomain.Organization
	err          error
}

func (f publicOrganizationLookupFake) ResolvePublicBySlug(
	context.Context,
	string,
) (organizationdomain.Organization, error) {
	return f.organization, f.err
}

func TestOrganizationAdapterOnlyPublishesReadyOrganizations(t *testing.T) {
	adapter := NewOrganizationDirectoryAdapter(publicOrganizationLookupFake{
		organization: organizationdomain.Organization{
			ID: "org-ready", Slug: "ready", Status: organizationdomain.Ready,
		},
	})
	id, err := adapter.ResolvePublicOrganization(context.Background(), "ready")
	if err != nil || id != "org-ready" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	blocked := NewOrganizationDirectoryAdapter(publicOrganizationLookupFake{
		organization: organizationdomain.Organization{
			ID: "org-suspended", Slug: "suspended", Status: organizationdomain.Suspended,
		},
	})
	if _, err := blocked.ResolvePublicOrganization(
		context.Background(),
		"suspended",
	); schedulingdomain.ErrorCodeOf(err) != schedulingdomain.CodeNotFound {
		t.Fatalf("suspended organization error=%v", err)
	}
}
