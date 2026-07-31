package access

import (
	"context"
	"errors"
	"net/http"
	"strings"

	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
)

// ClerkVerifier is deliberately narrow so Clerk verification can be tested
// without a network dependency. The concrete SDK checks signature, issuer,
// audience, authorized party, session state and active organization.
type ClerkVerifier interface {
	VerifySession(context.Context, string) (clerk.SessionClaims, error)
}
type MembershipResolver interface {
	ResolveClerkMembership(context.Context, string, string) (string, error)
}

type ClerkAuthenticator struct {
	Memberships MembershipResolver
	Verifier    ClerkVerifier
}

func (a ClerkAuthenticator) OrganizationID(request *http.Request) (string, error) {
	if a.Memberships == nil || a.Verifier == nil {
		return "", errors.New("clerk verifier unavailable")
	}
	header := strings.TrimSpace(request.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return "", errors.New("bearer token required")
	}
	claims, err := a.Verifier.VerifySession(request.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
	if err != nil {
		return "", err
	}
	if claims.OrganizationID == "" || claims.Subject == "" {
		return "", errors.New("Clerk session has no active organization or subject")
	}
	return a.Memberships.ResolveClerkMembership(request.Context(), claims.OrganizationID, claims.Subject)
}
