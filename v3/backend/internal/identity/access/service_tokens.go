package access

import (
	"encoding/base64"
	"fmt"
	"os"

	"context"

	"github.com/google/uuid"
)

// TokenSourceFromRuntime builds the service credential issuer from a seed
// mounted by the deployment's KMS/secret provider. No development fallback is
// permitted unless the caller explicitly selects its local fake mode.
type TokenSource interface {
	Token(context.Context, string, string) (string, error)
}
type IssuerTokenSource struct {
	Issuer  *ServiceIssuer
	Subject string
}

func (s IssuerTokenSource) Token(_ context.Context, audience, organizationID string) (string, error) {
	if s.Issuer == nil || s.Subject == "" {
		return "", fmt.Errorf("internal token issuer is not configured")
	}
	id := uuid.NewString()
	return s.Issuer.Mint(audience, s.Subject, organizationID, "", id, id, []string{"service"})
}

func TokenSourceFromRuntime(subject string) (TokenSource, error) {
	encoded := os.Getenv("PYMES_INTERNAL_SIGNING_SEED_B64")
	issuer, keyID := os.Getenv("PYMES_INTERNAL_ISSUER"), os.Getenv("PYMES_INTERNAL_KEY_ID")
	seed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode internal signing seed: %w", err)
	}
	signer, err := NewServiceIssuerFromSeed(issuer, keyID, seed, nil)
	if err != nil {
		return nil, err
	}
	return IssuerTokenSource{Issuer: signer, Subject: subject}, nil
}
