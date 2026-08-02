// architecture:adapter external
package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"hash/crc32"
	"regexp"
	"strings"
	"time"

	kms "cloud.google.com/go/kms/apiv1"
	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	credentialhelpers "github.com/devpablocristo/pymes/v3/backend/internal/identity/credentials/helpers"
	kmshelpers "github.com/devpablocristo/pymes/v3/backend/internal/identity/kms_signer/helpers"
	kmsmodels "github.com/devpablocristo/pymes/v3/backend/internal/identity/kms_signer/models"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var cryptoKeyVersionPattern = regexp.MustCompile(
	`^projects/[^/]+/locations/[^/]+/keyRings/[^/]+/cryptoKeys/[^/]+/cryptoKeyVersions/[1-9][0-9]*$`,
)

var castagnoliTable = crc32.MakeTable(crc32.Castagnoli)

type KMSClient interface {
	GetPublicKey(context.Context, *kmspb.GetPublicKeyRequest, ...gax.CallOption) (*kmspb.PublicKey, error)
	AsymmetricSign(context.Context, *kmspb.AsymmetricSignRequest, ...gax.CallOption) (*kmspb.AsymmetricSignResponse, error)
}

type kmsEd25519Signer struct {
	client     KMSClient
	keyVersion string
	keyID      string
	public     ed25519.PublicKey
}

func (s *kmsEd25519Signer) Sign(ctx context.Context, data []byte) ([]byte, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("KMS signer is not configured")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("KMS signing data is required")
	}
	response, err := s.client.AsymmetricSign(ctx, &kmspb.AsymmetricSignRequest{
		Name:       s.keyVersion,
		Data:       data,
		DataCrc32C: wrapperspb.Int64(int64(crc32.Checksum(data, castagnoliTable))),
	})
	if err != nil {
		return nil, fmt.Errorf("KMS asymmetric sign: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("KMS asymmetric sign returned an empty response")
	}
	if response.GetName() != s.keyVersion {
		return nil, fmt.Errorf("KMS asymmetric sign returned an unexpected key version")
	}
	if !response.GetVerifiedDataCrc32C() {
		return nil, fmt.Errorf("KMS did not verify the signing request CRC32C")
	}
	if !kmshelpers.ValidCRC32C(response.GetSignatureCrc32C(), response.GetSignature()) {
		return nil, fmt.Errorf("KMS signature CRC32C mismatch")
	}
	signature := append([]byte(nil), response.GetSignature()...)
	if len(signature) != ed25519.SignatureSize || !ed25519.Verify(s.public, data, signature) {
		return nil, fmt.Errorf("KMS returned an invalid Ed25519 signature")
	}
	return signature, nil
}

func (s *kmsEd25519Signer) PublicKey() ed25519.PublicKey {
	if s == nil {
		return nil
	}
	return append(ed25519.PublicKey(nil), s.public...)
}

func (s *kmsEd25519Signer) KeyID() string {
	if s == nil {
		return ""
	}
	return s.keyID
}

func NewKMSServiceIssuer(
	ctx context.Context,
	issuer string,
	keyVersion string,
	overlapKeyVersions []string,
	client KMSClient,
	now func() time.Time,
) (*ServiceIssuer, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		return nil, fmt.Errorf("%w: KMS client is required", ErrInvalidCredential)
	}
	current, err := loadKMSVerificationKey(ctx, client, keyVersion)
	if err != nil {
		return nil, err
	}
	overlap, err := LoadKMSVerificationKeys(ctx, client, overlapKeyVersions)
	if err != nil {
		return nil, err
	}
	signer := &kmsEd25519Signer{
		client:     client,
		keyVersion: keyVersion,
		keyID:      current.KeyID,
		public:     current.PublicKey,
	}
	challenge := sha256.Sum256([]byte("pymes-v3-internal-identity-startup:" + keyVersion))
	if _, err := signer.Sign(ctx, challenge[:]); err != nil {
		return nil, fmt.Errorf("verify KMS signing key at startup: %w", err)
	}
	return newServiceIssuer(issuer, signer, overlap, now)
}

func NewCloudKMSServiceIssuer(
	ctx context.Context,
	issuer string,
	keyVersion string,
	overlapKeyVersions []string,
	now func() time.Time,
) (*ServiceIssuer, *kms.KeyManagementClient, error) {
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create KMS client: %w", err)
	}
	issuerService, err := NewKMSServiceIssuer(ctx, issuer, keyVersion, overlapKeyVersions, client, now)
	if err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	return issuerService, client, nil
}

func LoadKMSVerificationKeys(ctx context.Context, client KMSClient, keyVersions []string) ([]VerificationKey, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		return nil, fmt.Errorf("%w: KMS client is required", ErrInvalidCredential)
	}
	keys := make([]VerificationKey, 0, len(keyVersions))
	seen := make(map[string]bool, len(keyVersions))
	for _, keyVersion := range keyVersions {
		keyVersion = strings.TrimSpace(keyVersion)
		if keyVersion == "" {
			continue
		}
		key, err := loadKMSVerificationKey(ctx, client, keyVersion)
		if err != nil {
			return nil, err
		}
		if seen[key.KeyID] {
			continue
		}
		seen[key.KeyID] = true
		keys = append(keys, key)
	}
	return keys, nil
}

func loadKMSVerificationKey(ctx context.Context, client KMSClient, keyVersion string) (VerificationKey, error) {
	if !cryptoKeyVersionPattern.MatchString(keyVersion) {
		return VerificationKey{}, fmt.Errorf(
			"%w: KMS key must be an explicit CryptoKeyVersion resource",
			ErrInvalidCredential,
		)
	}
	response, err := client.GetPublicKey(ctx, &kmspb.GetPublicKeyRequest{Name: keyVersion})
	if err != nil {
		return VerificationKey{}, fmt.Errorf("get KMS public key: %w", err)
	}
	if response == nil {
		return VerificationKey{}, fmt.Errorf("KMS returned an empty public key response")
	}
	if response.GetName() != keyVersion {
		return VerificationKey{}, fmt.Errorf("KMS returned an unexpected public key version")
	}
	if response.GetAlgorithm() != kmspb.CryptoKeyVersion_EC_SIGN_ED25519 {
		return VerificationKey{}, fmt.Errorf("KMS key algorithm must be EC_SIGN_ED25519")
	}
	if !kmshelpers.ValidCRC32C(response.GetPemCrc32C(), []byte(response.GetPem())) {
		return VerificationKey{}, fmt.Errorf("KMS public key CRC32C mismatch")
	}
	block, remainder := pem.Decode([]byte(response.GetPem()))
	if block == nil || block.Type != "PUBLIC KEY" || strings.TrimSpace(string(remainder)) != "" {
		return VerificationKey{}, fmt.Errorf("KMS public key must be a single PKIX PEM block")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return VerificationKey{}, fmt.Errorf("parse KMS public key: %w", err)
	}
	public, ok := parsed.(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return VerificationKey{}, fmt.Errorf("KMS public key is not Ed25519")
	}
	material := kmsmodels.PublicKeyMaterial{
		KeyID:     credentialhelpers.StableKeyID(public),
		PublicKey: append(ed25519.PublicKey(nil), public...),
	}
	return VerificationKey{KeyID: material.KeyID, PublicKey: material.PublicKey}, nil
}

func validCRC32C(expected *wrapperspb.Int64Value, data []byte) bool {
	return kmshelpers.ValidCRC32C(expected, data)
}
