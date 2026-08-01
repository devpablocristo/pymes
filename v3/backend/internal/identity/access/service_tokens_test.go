package access

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"io"
	"testing"
	"time"

	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/domain"
	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases"
)

func TestIssuerTokenSourcePropagatesRequestAndActorMetadata(t *testing.T) {
	t.Parallel()
	now := time.Unix(2_000_000_000, 0)
	issuer, err := NewServiceIssuer("pymes-v3", "local-test", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	source := &IssuerTokenSource{Issuer: issuer, Subject: "worker:outbox"}
	ctx := identityusecases.WithRequestMetadata(context.Background(), identityusecases.RequestMetadata{
		RequestID: "request-1", CorrelationID: "correlation-1",
	})
	ctx = identityusecases.WithPrincipal(ctx, identitydomain.Principal{
		OrganizationID: "org_acme", ActorID: "user_owner", Role: identitydomain.RoleOwner,
		OrganizationStatus: "ready", MembershipStatus: "active",
	})
	ctx = identityusecases.WithDelegatedActor(ctx, "user_delegate")
	token, err := source.Token(ctx, "accounting", "org_acme")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := VerifyInternalCredential(token, issuer.PublicKey(), now, "pymes-v3", "accounting", "org_acme")
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "worker:outbox" || claims.RequestID != "request-1" ||
		claims.CorrelationID != "correlation-1" || claims.ActorID != "user_owner" ||
		claims.DelegatedActorID != "user_delegate" || claims.TokenID == "" {
		t.Fatalf("claims = %#v", claims)
	}
}

func TestIssuerTokenSourceDoesNotPropagateCrossTenantActor(t *testing.T) {
	t.Parallel()
	now := time.Unix(2_000_000_000, 0)
	issuer, err := NewServiceIssuer("pymes-v3", "local-test", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	source := &IssuerTokenSource{Issuer: issuer, Subject: "worker:outbox"}
	ctx := identityusecases.WithPrincipal(context.Background(), identitydomain.Principal{
		OrganizationID: "org_other", ActorID: "user_other", Role: identitydomain.RoleOwner,
		OrganizationStatus: "ready", MembershipStatus: "active",
	})
	token, err := source.Token(ctx, "accounting", "org_acme")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := VerifyInternalCredential(token, issuer.PublicKey(), now, "pymes-v3", "accounting", "org_acme")
	if err != nil {
		t.Fatal(err)
	}
	if claims.ActorID != "" || claims.DelegatedActorID != "" {
		t.Fatalf("cross-tenant actor leaked into claims: %#v", claims)
	}
}

func TestRuntimeTokenSourceUsesSeedOnlyOutsideProduction(t *testing.T) {
	t.Parallel()
	seed := make([]byte, ed25519.SeedSize)
	values := map[string]string{
		"PYMES_ENVIRONMENT":               "test",
		"PYMES_INTERNAL_ISSUER":           "pymes-v3",
		"PYMES_INTERNAL_KEY_ID":           "local-test",
		"PYMES_INTERNAL_SIGNING_SEED_B64": base64.StdEncoding.EncodeToString(seed),
	}
	source, err := tokenSourceFromRuntime(
		context.Background(),
		"worker:test",
		func(key string) string { return values[key] },
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if source.Issuer.KeyID() != "local-test" {
		t.Fatalf("kid = %q", source.Issuer.KeyID())
	}
}

func TestRuntimeTokenSourceRequiresKMSAndForbidsProductionSeed(t *testing.T) {
	t.Parallel()
	base := map[string]string{
		"PYMES_ENVIRONMENT":              "production",
		"PYMES_INTERNAL_ISSUER":          "pymes-v3",
		"PYMES_INTERNAL_KMS_KEY_VERSION": testKMSVersion1,
	}
	t.Run("seed fails closed", func(t *testing.T) {
		values := cloneStrings(base)
		values["PYMES_INTERNAL_SIGNING_SEED_B64"] = "forbidden"
		called := false
		_, err := tokenSourceFromRuntime(context.Background(), "worker:test", func(key string) string { return values[key] },
			func(context.Context, string, string, []string) (*ServiceIssuer, io.Closer, error) {
				called = true
				return nil, nil, nil
			})
		if err == nil || called {
			t.Fatalf("err=%v factoryCalled=%v", err, called)
		}
	})
	t.Run("explicit version and overlap", func(t *testing.T) {
		values := cloneStrings(base)
		values["PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS"] = testKMSVersion2
		closer := &trackingCloser{}
		source, err := tokenSourceFromRuntime(context.Background(), "worker:test", func(key string) string { return values[key] },
			func(_ context.Context, issuer, version string, overlap []string) (*ServiceIssuer, io.Closer, error) {
				if issuer != "pymes-v3" || version != testKMSVersion1 || len(overlap) != 1 || overlap[0] != testKMSVersion2 {
					t.Fatalf("factory issuer=%q version=%q overlap=%#v", issuer, version, overlap)
				}
				local, localErr := NewServiceIssuer("pymes-v3", "kms-test", nil)
				return local, closer, localErr
			})
		if err != nil {
			t.Fatal(err)
		}
		if err := source.Close(); err != nil {
			t.Fatal(err)
		}
		if err := source.Close(); err != nil {
			t.Fatal(err)
		}
		if closer.calls != 1 {
			t.Fatalf("close calls = %d", closer.calls)
		}
	})
	t.Run("alias rejected", func(t *testing.T) {
		values := cloneStrings(base)
		values["PYMES_INTERNAL_KMS_KEY_VERSION"] = "projects/p/locations/l/keyRings/r/cryptoKeys/k/cryptoKeyVersions/latest"
		if _, err := tokenSourceFromRuntime(context.Background(), "worker:test", func(key string) string { return values[key] }, nil); err == nil {
			t.Fatal("expected explicit KMS version validation error")
		}
	})
}

type trackingCloser struct{ calls int }

func (c *trackingCloser) Close() error {
	c.calls++
	return nil
}

func cloneStrings(source map[string]string) map[string]string {
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}
