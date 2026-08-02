// architecture:adapter external
package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	credentialhelpers "github.com/devpablocristo/pymes/v3/backend/internal/identity/credentials/helpers"
	credentialmodels "github.com/devpablocristo/pymes/v3/backend/internal/identity/credentials/models"
)

var (
	ErrInvalidCredential = errors.New("invalid internal credential")
	ErrCredentialExpired = errors.New("internal credential expired")
	ErrClaimMismatch     = errors.New("internal credential claim mismatch")
)

type InternalCredential = credentialmodels.InternalCredential

type CredentialRequest = credentialmodels.CredentialRequest

type credentialSigner interface {
	Sign(context.Context, []byte) ([]byte, error)
	PublicKey() ed25519.PublicKey
	KeyID() string
}

type localEd25519Signer struct {
	keyID   string
	private ed25519.PrivateKey
	public  ed25519.PublicKey
}

func (s localEd25519Signer) Sign(ctx context.Context, data []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return ed25519.Sign(s.private, data), nil
}

func (s localEd25519Signer) PublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), s.public...)
}

func (s localEd25519Signer) KeyID() string { return s.keyID }

type VerificationKey = credentialmodels.VerificationKey

type ServiceIssuer struct {
	issuer           string
	signer           credentialSigner
	verificationKeys []VerificationKey
	now              func() time.Time
}

func NewServiceIssuer(issuer, keyID string, now func() time.Time) (*ServiceIssuer, error) {
	if issuer == "" || keyID == "" {
		return nil, fmt.Errorf("%w: issuer and key ID are required", ErrInvalidCredential)
	}
	if now == nil {
		now = time.Now
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	return newServiceIssuer(issuer, localEd25519Signer{keyID: keyID, private: private, public: public}, nil, now)
}

func NewServiceIssuerFromSeed(issuer, keyID string, seed []byte, now func() time.Time) (*ServiceIssuer, error) {
	if issuer == "" || keyID == "" || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%w: issuer, key ID and seed are required", ErrInvalidCredential)
	}
	if now == nil {
		now = time.Now
	}
	private := ed25519.NewKeyFromSeed(seed)
	return newServiceIssuer(issuer, localEd25519Signer{keyID: keyID, private: private, public: private.Public().(ed25519.PublicKey)}, nil, now)
}

func newServiceIssuer(issuer string, signer credentialSigner, overlap []VerificationKey, now func() time.Time) (*ServiceIssuer, error) {
	if strings.TrimSpace(issuer) == "" || signer == nil || strings.TrimSpace(signer.KeyID()) == "" {
		return nil, fmt.Errorf("%w: issuer and signer are required", ErrInvalidCredential)
	}
	public := signer.PublicKey()
	if len(public) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: signer must expose an Ed25519 public key", ErrInvalidCredential)
	}
	if now == nil {
		now = time.Now
	}
	keys := []VerificationKey{{KeyID: signer.KeyID(), PublicKey: public}}
	seen := map[string]bool{signer.KeyID(): true}
	for _, key := range overlap {
		if strings.TrimSpace(key.KeyID) == "" || len(key.PublicKey) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("%w: invalid verification key", ErrInvalidCredential)
		}
		if seen[key.KeyID] {
			continue
		}
		seen[key.KeyID] = true
		keys = append(keys, VerificationKey{KeyID: key.KeyID, PublicKey: append(ed25519.PublicKey(nil), key.PublicKey...)})
	}
	return &ServiceIssuer{issuer: issuer, signer: signer, verificationKeys: keys, now: now}, nil
}

func (i *ServiceIssuer) PublicKey() ed25519.PublicKey {
	if i == nil || i.signer == nil {
		return nil
	}
	return i.signer.PublicKey()
}

func (i *ServiceIssuer) KeyID() string {
	if i == nil || i.signer == nil {
		return ""
	}
	return i.signer.KeyID()
}

func (i *ServiceIssuer) Mint(audience, subject, orgID, actorID, requestID, tokenID string, roles []string) (string, error) {
	return i.MintCredential(context.Background(), CredentialRequest{
		Audience:      audience,
		Subject:       subject,
		OrgID:         orgID,
		ActorID:       actorID,
		Roles:         roles,
		RequestID:     requestID,
		CorrelationID: requestID,
		TokenID:       tokenID,
	})
}

func (i *ServiceIssuer) MintCredential(ctx context.Context, request CredentialRequest) (string, error) {
	if i == nil || i.signer == nil {
		return "", fmt.Errorf("%w: signer is not configured", ErrInvalidCredential)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if request.Audience == "" || request.Subject == "" || request.OrgID == "" ||
		request.RequestID == "" || request.CorrelationID == "" || request.TokenID == "" ||
		len(request.Roles) == 0 {
		return "", fmt.Errorf("%w: required claims are missing", ErrInvalidCredential)
	}
	for _, role := range request.Roles {
		if strings.TrimSpace(role) == "" {
			return "", fmt.Errorf("%w: roles cannot contain empty values", ErrInvalidCredential)
		}
	}
	now := i.now().UTC()
	claims := InternalCredential{
		Issuer:           i.issuer,
		Audience:         request.Audience,
		Subject:          request.Subject,
		OrgID:            request.OrgID,
		ActorID:          request.ActorID,
		DelegatedActorID: request.DelegatedActorID,
		Roles:            append([]string(nil), request.Roles...),
		RequestID:        request.RequestID,
		CorrelationID:    request.CorrelationID,
		TokenID:          request.TokenID,
		IssuedAt:         now.Unix(),
		ExpiresAt:        now.Add(5 * time.Minute).Unix(),
		KeyID:            i.signer.KeyID(),
	}
	signingInput, err := credentialhelpers.SigningInput(i.signer.KeyID(), claims)
	if err != nil {
		return "", err
	}
	signature, err := i.signer.Sign(ctx, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("sign internal credential: %w", err)
	}
	if len(signature) != ed25519.SignatureSize || !ed25519.Verify(i.signer.PublicKey(), []byte(signingInput), signature) {
		return "", fmt.Errorf("%w: signer returned an unverifiable signature", ErrInvalidCredential)
	}
	return credentialhelpers.SignedToken(signingInput, signature), nil
}

func VerifyInternalCredential(token string, public ed25519.PublicKey, now time.Time, expectedIssuer, expectedAudience, expectedOrgID string) (InternalCredential, error) {
	header, claims, err := credentialhelpers.DecodeAndVerify(token, public)
	if err != nil {
		return InternalCredential{}, ErrInvalidCredential
	}
	if claims.Issuer != expectedIssuer || claims.Audience != expectedAudience ||
		claims.OrgID != expectedOrgID || claims.OrgID == "" {
		return InternalCredential{}, ErrClaimMismatch
	}
	if claims.KeyID != header.KeyID || claims.Subject == "" || claims.RequestID == "" ||
		claims.CorrelationID == "" || claims.TokenID == "" || len(claims.Roles) == 0 ||
		claims.IssuedAt <= 0 || claims.ExpiresAt <= claims.IssuedAt ||
		claims.ExpiresAt-claims.IssuedAt > int64((5*time.Minute)/time.Second) {
		return InternalCredential{}, ErrInvalidCredential
	}
	if now.UTC().Unix() >= claims.ExpiresAt {
		return InternalCredential{}, ErrCredentialExpired
	}
	return claims, nil
}

func (i *ServiceIssuer) JWKSJSON() (string, error) {
	if i == nil {
		return "", fmt.Errorf("%w: issuer is not configured", ErrInvalidCredential)
	}
	return JWKSJSON(i.verificationKeys)
}

func JWKSJSON(keys []VerificationKey) (string, error) {
	encoded, err := credentialhelpers.JWKSJSON(keys)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidCredential, err)
	}
	return encoded, nil
}
