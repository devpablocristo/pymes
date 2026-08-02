package scheduling

import (
	"net/http"
	"testing"

	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
)

type identityProviderFake struct {
	principal identitydomain.Principal
}

func (f identityProviderFake) Principal(*http.Request) (identitydomain.Principal, error) {
	return f.principal, nil
}

func TestIdentityAdapterTranslatesVerifiedLocalPrincipal(t *testing.T) {
	adapter := NewIdentityAuthenticator(identityProviderFake{principal: identitydomain.Principal{
		OrganizationID:     "org-a",
		ActorID:            "user-a",
		Role:               identitydomain.RoleMember,
		Permissions:        []string{"scheduling:read"},
		OrganizationStatus: "ready",
		MembershipStatus:   "active",
	}})
	principal, err := adapter.Principal(&http.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if !principal.Allows("org-a", "scheduling:read") ||
		principal.Allows("org-b", "scheduling:read") {
		t.Fatalf("translated principal broke tenant authorization: %+v", principal)
	}
}
