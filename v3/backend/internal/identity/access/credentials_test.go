package access

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceIssuerMintsCompleteFiveMinuteCredential(t *testing.T) {
	t.Parallel()
	now := time.Unix(2_000_000_000, 0).UTC()
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = byte(index + 1)
	}
	issuer, err := NewServiceIssuerFromSeed("pymes-v3", "local-test-1", seed, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	token, err := issuer.MintCredential(context.Background(), CredentialRequest{
		Audience:         "accounting",
		Subject:          "worker:outbox",
		OrgID:            "org_acme",
		ActorID:          "user_owner",
		DelegatedActorID: "user_delegate",
		Roles:            []string{"service"},
		RequestID:        "request-1",
		CorrelationID:    "correlation-1",
		TokenID:          "token-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := VerifyInternalCredential(token, issuer.PublicKey(), now, "pymes-v3", "accounting", "org_acme")
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "worker:outbox" || claims.ActorID != "user_owner" ||
		claims.DelegatedActorID != "user_delegate" || claims.RequestID != "request-1" ||
		claims.CorrelationID != "correlation-1" || claims.TokenID != "token-1" ||
		claims.ExpiresAt-claims.IssuedAt != 300 {
		t.Fatalf("claims = %#v", claims)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "service" {
		t.Fatalf("roles = %#v", claims.Roles)
	}
}

func TestServiceIssuerLegacyMintKeepsCompatibility(t *testing.T) {
	t.Parallel()
	issuer, err := NewServiceIssuer("pymes-v3", "ephemeral-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := issuer.Mint("fiscal", "worker:test", "org_1", "", "request-1", "token-1", []string{"service"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := VerifyInternalCredential(token, issuer.PublicKey(), time.Now(), "pymes-v3", "fiscal", "org_1")
	if err != nil {
		t.Fatal(err)
	}
	if claims.CorrelationID != claims.RequestID {
		t.Fatalf("legacy correlation=%q request=%q", claims.CorrelationID, claims.RequestID)
	}
}

func TestServiceIssuerRejectsCancelledContextAndInvalidRoles(t *testing.T) {
	t.Parallel()
	issuer, err := NewServiceIssuer("pymes-v3", "ephemeral-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	request := CredentialRequest{
		Audience: "fiscal", Subject: "worker:test", OrgID: "org_1",
		Roles: []string{"service"}, RequestID: "request-1", CorrelationID: "correlation-1", TokenID: "token-1",
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := issuer.MintCredential(ctx, request); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled mint err = %v", err)
	}
	request.Roles = []string{""}
	if _, err := issuer.MintCredential(context.Background(), request); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("invalid role err = %v", err)
	}
}

func TestJWKSJSONPublishesVerificationOnlyEd25519Key(t *testing.T) {
	t.Parallel()
	issuer, err := NewServiceIssuer("pymes-v3", "ephemeral-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := issuer.JWKSJSON()
	if err != nil {
		t.Fatal(err)
	}
	var jwks struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal([]byte(encoded), &jwks); err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 1 || jwks.Keys[0]["kid"] != "ephemeral-test" ||
		jwks.Keys[0]["kty"] != "OKP" || jwks.Keys[0]["crv"] != "Ed25519" ||
		jwks.Keys[0]["alg"] != "EdDSA" || jwks.Keys[0]["use"] != "sig" {
		t.Fatalf("jwks = %s", encoded)
	}
	x, _ := jwks.Keys[0]["x"].(string)
	public, err := base64.RawURLEncoding.DecodeString(x)
	if err != nil || !strings.EqualFold(base64.RawURLEncoding.EncodeToString(public), x) ||
		!ed25519.PublicKey(public).Equal(issuer.PublicKey()) {
		t.Fatalf("invalid public coordinate %q: %v", x, err)
	}
}
