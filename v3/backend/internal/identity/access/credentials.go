package access

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidCredential = errors.New("invalid internal credential")
	ErrCredentialExpired = errors.New("internal credential expired")
	ErrClaimMismatch     = errors.New("internal credential claim mismatch")
)

type InternalCredential struct {
	Issuer    string   `json:"iss"`
	Audience  string   `json:"aud"`
	Subject   string   `json:"sub"`
	OrgID     string   `json:"org_id"`
	ActorID   string   `json:"actor_id,omitempty"`
	Roles     []string `json:"roles"`
	RequestID string   `json:"request_id"`
	TokenID   string   `json:"jti"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
	KeyID     string   `json:"kid"`
}
type ServiceIssuer struct {
	issuer  string
	keyID   string
	private ed25519.PrivateKey
	public  ed25519.PublicKey
	now     func() time.Time
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
	return &ServiceIssuer{issuer: issuer, keyID: keyID, private: private, public: public, now: now}, nil
}
func NewServiceIssuerFromSeed(issuer, keyID string, seed []byte, now func() time.Time) (*ServiceIssuer, error) {
	if issuer == "" || keyID == "" || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%w: issuer, key ID and seed are required", ErrInvalidCredential)
	}
	if now == nil {
		now = time.Now
	}
	private := ed25519.NewKeyFromSeed(seed)
	return &ServiceIssuer{issuer: issuer, keyID: keyID, private: private, public: private.Public().(ed25519.PublicKey), now: now}, nil
}
func (i *ServiceIssuer) PublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), i.public...)
}
func (i *ServiceIssuer) Mint(audience, subject, orgID, actorID, requestID, tokenID string, roles []string) (string, error) {
	if audience == "" || subject == "" || orgID == "" || requestID == "" || tokenID == "" {
		return "", fmt.Errorf("%w: required claims are missing", ErrInvalidCredential)
	}
	now := i.now().UTC()
	claims := InternalCredential{Issuer: i.issuer, Audience: audience, Subject: subject, OrgID: orgID, ActorID: actorID, Roles: roles, RequestID: requestID, TokenID: tokenID, IssuedAt: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(), KeyID: i.keyID}
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": i.keyID})
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(i.private, []byte(signingInput))), nil
}
func VerifyInternalCredential(token string, public ed25519.PublicKey, now time.Time, expectedIssuer, expectedAudience, expectedOrgID string) (InternalCredential, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return InternalCredential{}, ErrInvalidCredential
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(public, []byte(parts[0]+"."+parts[1]), signature) {
		return InternalCredential{}, ErrInvalidCredential
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return InternalCredential{}, ErrInvalidCredential
	}
	var claims InternalCredential
	if err = json.Unmarshal(payload, &claims); err != nil {
		return InternalCredential{}, ErrInvalidCredential
	}
	if claims.Issuer != expectedIssuer || claims.Audience != expectedAudience || claims.OrgID != expectedOrgID || claims.OrgID == "" {
		return InternalCredential{}, ErrClaimMismatch
	}
	if now.UTC().Unix() >= claims.ExpiresAt {
		return InternalCredential{}, ErrCredentialExpired
	}
	return claims, nil
}
