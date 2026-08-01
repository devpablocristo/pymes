// architecture:adapter external
package scheduling

import (
	"net/http"

	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
	identityhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/identity/helpers"
)

// IdentityPrincipalProvider is consumer-owned. The Clerk implementation stays
// behind the Identity context and only its verified local principal reaches
// this anti-corruption adapter.
type IdentityPrincipalProvider interface {
	Principal(*http.Request) (identitydomain.Principal, error)
}

type IdentityAuthenticator struct {
	provider IdentityPrincipalProvider
}

func NewIdentityAuthenticator(provider IdentityPrincipalProvider) *IdentityAuthenticator {
	return &IdentityAuthenticator{provider: provider}
}

func (a *IdentityAuthenticator) Principal(request *http.Request) (Principal, error) {
	value, err := a.provider.Principal(request)
	if err != nil {
		return Principal{}, err
	}
	return identityhelpers.ToSchedulingPrincipal(
		identityhelpers.FromIdentityPrincipal(value),
	), nil
}
