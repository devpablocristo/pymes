package identity

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	clerk "github.com/devpablocristo/platform/sdks/clerk/go"
	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
)

type verifierStub func(context.Context, string) (clerk.SessionClaims, error)

func (f verifierStub) VerifySession(ctx context.Context, token string) (clerk.SessionClaims, error) {
	return f(ctx, token)
}

type membershipStub func(context.Context, string, string) (identitydomain.Principal, error)

func (f membershipStub) ResolveClerkMembership(ctx context.Context, organizationID, userID string) (identitydomain.Principal, error) {
	return f(ctx, organizationID, userID)
}

func TestClerkAuthenticatorRejectsMissingConfigurationAndBearer(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest("GET", "/", nil)
	if _, err := (ClerkAuthenticator{}).Principal(request); err == nil {
		t.Fatal("expected missing configuration to fail")
	}
	auth := ClerkAuthenticator{
		Verifier: verifierStub(func(context.Context, string) (clerk.SessionClaims, error) {
			t.Fatal("verifier must not run without a bearer token")
			return clerk.SessionClaims{}, nil
		}),
		Memberships: membershipStub(func(context.Context, string, string) (identitydomain.Principal, error) {
			t.Fatal("membership resolver must not run without a bearer token")
			return identitydomain.Principal{}, nil
		}),
	}
	if _, err := auth.Principal(request); err == nil {
		t.Fatal("expected missing bearer token to fail")
	}
}

func TestClerkAuthenticatorVerifiesTokenAndResolvesLocalMembership(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer session-token")
	auth := ClerkAuthenticator{
		Verifier: verifierStub(func(_ context.Context, token string) (clerk.SessionClaims, error) {
			if token != "session-token" {
				t.Fatalf("unexpected token %q", token)
			}
			return clerk.SessionClaims{
				OrganizationID: "org_clerk", Subject: "user_clerk",
				SessionID: "session_clerk",
			}, nil
		}),
		Memberships: membershipStub(func(_ context.Context, organizationID, userID string) (identitydomain.Principal, error) {
			if organizationID != "org_clerk" || userID != "user_clerk" {
				t.Fatalf("unexpected Clerk identity %q/%q", organizationID, userID)
			}
			return identitydomain.Principal{
				OrganizationID: "local-organization", ActorID: "user_clerk", Role: identitydomain.RoleAdmin,
				Permissions: []string{"org:members:read"}, OrganizationStatus: "ready", MembershipStatus: "active",
			}, nil
		}),
	}
	principal, err := auth.Principal(request)
	if err != nil {
		t.Fatal(err)
	}
	if principal.OrganizationID != "local-organization" || principal.ActorID != "user_clerk" ||
		principal.Role != identitydomain.RoleAdmin || len(principal.Permissions) != 1 ||
		principal.SessionID != "session_clerk" ||
		principal.OrganizationStatus != "ready" || principal.MembershipStatus != "active" {
		t.Fatalf("unexpected local principal %+v", principal)
	}
}

func TestClerkAuthenticatorRejectsInvalidOrIncompleteSession(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer session-token")
	sentinel := errors.New("invalid session")
	auth := ClerkAuthenticator{
		Verifier: verifierStub(func(context.Context, string) (clerk.SessionClaims, error) {
			return clerk.SessionClaims{}, sentinel
		}),
		Memberships: membershipStub(func(context.Context, string, string) (identitydomain.Principal, error) {
			t.Fatal("membership resolver must not run for an invalid session")
			return identitydomain.Principal{}, nil
		}),
	}
	if _, err := auth.Principal(request); !errors.Is(err, sentinel) {
		t.Fatalf("expected verifier error, got %v", err)
	}
	auth.Verifier = verifierStub(func(context.Context, string) (clerk.SessionClaims, error) {
		return clerk.SessionClaims{Subject: "user_clerk"}, nil
	})
	if _, err := auth.Principal(request); err == nil {
		t.Fatal("expected session without active organization to fail")
	}
}
