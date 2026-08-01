package identity

import (
	"context"
	"testing"

	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
)

func TestPrincipalContextRoundTrip(t *testing.T) {
	t.Parallel()
	principal := identitydomain.Principal{
		OrganizationID: "org_1", ActorID: "user_1", Role: identitydomain.RoleOwner,
		OrganizationStatus: "ready", MembershipStatus: "active",
	}
	ctx := WithPrincipal(context.Background(), principal)
	if actual, ok := PrincipalFromContext(ctx); !ok || actual.OrganizationID != principal.OrganizationID || actual.ActorID != principal.ActorID {
		t.Fatalf("context principal=%+v ok=%v", actual, ok)
	}
	ctx = WithDelegatedActor(ctx, "user_delegate")
	if actual, ok := DelegatedActorFromContext(ctx); !ok || actual != "user_delegate" {
		t.Fatalf("delegated actor=%q ok=%v", actual, ok)
	}
	ctx = WithRequestMetadata(ctx, RequestMetadata{RequestID: "request-1", CorrelationID: "correlation-1"})
	if actual, ok := RequestMetadataFromContext(ctx); !ok ||
		actual.RequestID != "request-1" || actual.CorrelationID != "correlation-1" {
		t.Fatalf("request metadata=%#v ok=%v", actual, ok)
	}
}
