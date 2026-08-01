// architecture:adapter external
package identity

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"strings"
	"sync"

	tokenhelpers "github.com/devpablocristo/pymes/v3/backend/internal/identity/service_tokens/helpers"
	tokenmodels "github.com/devpablocristo/pymes/v3/backend/internal/identity/service_tokens/models"
)

type IssuerTokenSource struct {
	Issuer  *ServiceIssuer
	Subject string

	closer    io.Closer
	closeOnce sync.Once
	closeErr  error
}

func (s *IssuerTokenSource) Token(ctx context.Context, audience, organizationID string) (string, error) {
	if s == nil || s.Issuer == nil || s.Subject == "" {
		return "", fmt.Errorf("internal token issuer is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	requestID := uuid.NewString()
	correlationID := requestID
	if metadata, ok := RequestMetadataFromContext(ctx); ok {
		requestID = metadata.RequestID
		correlationID = metadata.CorrelationID
	}
	actorID, delegatedActorID := "", ""
	if principal, ok := PrincipalFromContext(ctx); ok && principal.CanRead(organizationID) {
		actorID = principal.ActorID
		if delegated, exists := DelegatedActorFromContext(ctx); exists {
			delegatedActorID = delegated
		}
	}
	return s.Issuer.MintCredential(ctx, CredentialRequest{
		Audience:         audience,
		Subject:          s.Subject,
		OrgID:            organizationID,
		ActorID:          actorID,
		DelegatedActorID: delegatedActorID,
		Roles:            []string{"service"},
		RequestID:        requestID,
		CorrelationID:    correlationID,
		TokenID:          uuid.NewString(),
	})
}

func (s *IssuerTokenSource) Close() error {
	if s == nil || s.closer == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.closeErr = s.closer.Close()
	})
	return s.closeErr
}

// TokenSourceFromRuntime is retained for local tools and tests. Long-running
// workloads should use TokenSourceFromRuntimeContext and close the result.
func TokenSourceFromRuntime(subject string) (*IssuerTokenSource, error) {
	return TokenSourceFromRuntimeContext(context.Background(), subject)
}

func TokenSourceFromRuntimeContext(ctx context.Context, subject string) (*IssuerTokenSource, error) {
	return tokenSourceFromRuntime(ctx, subject, os.Getenv, cloudKMSIssuer)
}

type cloudKMSIssuerFactory func(context.Context, string, string, []string) (*ServiceIssuer, io.Closer, error)

func cloudKMSIssuer(
	ctx context.Context,
	issuer string,
	keyVersion string,
	overlapKeyVersions []string,
) (*ServiceIssuer, io.Closer, error) {
	serviceIssuer, client, err := NewCloudKMSServiceIssuer(ctx, issuer, keyVersion, overlapKeyVersions, nil)
	if err != nil {
		return nil, nil, err
	}
	return serviceIssuer, client, nil
}

func tokenSourceFromRuntime(
	ctx context.Context,
	subject string,
	getenv func(string) string,
	newCloudKMSIssuer cloudKMSIssuerFactory,
) (*IssuerTokenSource, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(subject) == "" {
		return nil, fmt.Errorf("internal workload subject is required")
	}
	if getenv == nil {
		return nil, fmt.Errorf("environment reader is required")
	}
	settings := tokenmodels.RuntimeSettings{
		Environment: strings.ToLower(strings.TrimSpace(getenv("PYMES_ENVIRONMENT"))),
		Issuer:      strings.TrimSpace(getenv("PYMES_INTERNAL_ISSUER")),
	}
	environment := settings.Environment
	if environment == "" {
		environment = "development"
	}
	issuer := settings.Issuer
	if issuer == "" {
		return nil, fmt.Errorf("PYMES_INTERNAL_ISSUER is required")
	}

	switch environment {
	case "production":
		if strings.TrimSpace(getenv("PYMES_INTERNAL_SIGNING_SEED_B64")) != "" {
			return nil, fmt.Errorf("PYMES_INTERNAL_SIGNING_SEED_B64 is forbidden in production")
		}
		keyVersion := strings.TrimSpace(getenv("PYMES_INTERNAL_KMS_KEY_VERSION"))
		if !cryptoKeyVersionPattern.MatchString(keyVersion) {
			return nil, fmt.Errorf("PYMES_INTERNAL_KMS_KEY_VERSION must be an explicit CryptoKeyVersion resource")
		}
		if newCloudKMSIssuer == nil {
			return nil, fmt.Errorf("Cloud KMS issuer factory is required")
		}
		serviceIssuer, closer, err := newCloudKMSIssuer(
			ctx,
			issuer,
			keyVersion,
			tokenhelpers.CSVValues(getenv("PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS")),
		)
		if err != nil {
			return nil, err
		}
		return &IssuerTokenSource{Issuer: serviceIssuer, Subject: subject, closer: closer}, nil

	case "development", "test":
		if strings.TrimSpace(getenv("PYMES_INTERNAL_KMS_KEY_VERSION")) != "" {
			return nil, fmt.Errorf("Cloud KMS signing is reserved for production")
		}
		encoded := strings.TrimSpace(getenv("PYMES_INTERNAL_SIGNING_SEED_B64"))
		seed, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode internal signing seed: %w", err)
		}
		signer, err := NewServiceIssuerFromSeed(
			issuer,
			strings.TrimSpace(getenv("PYMES_INTERNAL_KEY_ID")),
			seed,
			nil,
		)
		if err != nil {
			return nil, err
		}
		return &IssuerTokenSource{Issuer: signer, Subject: subject}, nil

	default:
		return nil, fmt.Errorf("PYMES_ENVIRONMENT must be development, test, or production")
	}
}

func csvValues(value string) []string {
	return tokenhelpers.CSVValues(value)
}
