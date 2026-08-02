// architecture:adapter external
package identity

import (
	"context"
	"errors"
	"net/http"

	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	clerkhelpers "github.com/devpablocristo/pymes/v3/backend/internal/identity/clerk/helpers"
	clerkmodels "github.com/devpablocristo/pymes/v3/backend/internal/identity/clerk/models"
	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
)

// ClerkVerifier is deliberately narrow so Clerk verification can be tested
// without a network dependency. The concrete SDK checks signature, issuer,
// audience, authorized party, session state and active organization.
type ClerkVerifier interface {
	VerifySession(context.Context, string) (clerk.SessionClaims, error)
}
type MembershipResolver interface {
	ResolveClerkMembership(context.Context, string, string) (identitydomain.Principal, error)
}

type ClerkAuthenticator struct {
	Memberships MembershipResolver
	Verifier    ClerkVerifier
}

func (a ClerkAuthenticator) Principal(request *http.Request) (identitydomain.Principal, error) {
	if a.Memberships == nil || a.Verifier == nil {
		return identitydomain.Principal{}, errors.New("clerk verifier unavailable")
	}
	token, err := clerkhelpers.SessionToken(request)
	if err != nil {
		return identitydomain.Principal{}, err
	}
	claims, err := a.Verifier.VerifySession(request.Context(), token)
	if err != nil {
		return identitydomain.Principal{}, err
	}
	identity := clerkmodels.SessionIdentity{
		OrganizationID: claims.OrganizationID,
		Subject:        claims.Subject,
	}
	if identity.OrganizationID == "" || identity.Subject == "" ||
		claims.SessionID == "" {
		return identitydomain.Principal{}, errors.New("Clerk session has no active organization or subject")
	}
	principal, err := a.Memberships.ResolveClerkMembership(
		request.Context(), identity.OrganizationID, identity.Subject,
	)
	if err != nil {
		return identitydomain.Principal{}, err
	}
	principal.SessionID = claims.SessionID
	return principal, nil
}
