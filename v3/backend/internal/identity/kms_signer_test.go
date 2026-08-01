package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"hash/crc32"
	"testing"
	"time"

	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	testKMSVersion1 = "projects/pymes-test/locations/us-central1/keyRings/pymes-v3-test/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/1"
	testKMSVersion2 = "projects/pymes-test/locations/us-central1/keyRings/pymes-v3-test/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/2"
)

type fakeKMSMaterial struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
	pem     string
}

type fakeKMSClient struct {
	materials  map[string]fakeKMSMaterial
	signCalls  []*kmspb.AsymmetricSignRequest
	mutateGet  func(*kmspb.PublicKey)
	mutateSign func(*kmspb.AsymmetricSignResponse)
}

func (f *fakeKMSClient) GetPublicKey(_ context.Context, request *kmspb.GetPublicKeyRequest, _ ...gax.CallOption) (*kmspb.PublicKey, error) {
	material := f.materials[request.GetName()]
	response := &kmspb.PublicKey{
		Name:      request.GetName(),
		Pem:       material.pem,
		PemCrc32C: wrapperspb.Int64(int64(crc32.Checksum([]byte(material.pem), castagnoliTable))),
		Algorithm: kmspb.CryptoKeyVersion_EC_SIGN_ED25519,
	}
	if f.mutateGet != nil {
		f.mutateGet(response)
	}
	return response, nil
}

func (f *fakeKMSClient) AsymmetricSign(_ context.Context, request *kmspb.AsymmetricSignRequest, _ ...gax.CallOption) (*kmspb.AsymmetricSignResponse, error) {
	f.signCalls = append(f.signCalls, &kmspb.AsymmetricSignRequest{
		Name:       request.GetName(),
		Data:       append([]byte(nil), request.GetData()...),
		DataCrc32C: wrapperspb.Int64(request.GetDataCrc32C().GetValue()),
	})
	signature := ed25519.Sign(f.materials[request.GetName()].private, request.GetData())
	response := &kmspb.AsymmetricSignResponse{
		Name:               request.GetName(),
		Signature:          signature,
		SignatureCrc32C:    wrapperspb.Int64(int64(crc32.Checksum(signature, castagnoliTable))),
		VerifiedDataCrc32C: true,
	}
	if f.mutateSign != nil {
		f.mutateSign(response)
	}
	return response, nil
}

func TestKMSServiceIssuerVerifiesIntegrityAndPublishesRotationOverlap(t *testing.T) {
	t.Parallel()
	client := newFakeKMSClient(testKMSVersion1, testKMSVersion2)
	now := time.Unix(2_000_000_000, 0)
	issuer, err := NewKMSServiceIssuer(
		context.Background(),
		"pymes-v3",
		testKMSVersion1,
		[]string{testKMSVersion2, testKMSVersion1},
		client,
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	if issuer.KeyID() != stableKMSKeyID(client.materials[testKMSVersion1].public) {
		t.Fatalf("kid = %q", issuer.KeyID())
	}
	token, err := issuer.MintCredential(context.Background(), CredentialRequest{
		Audience: "accounting", Subject: "worker:outbox", OrgID: "org_acme",
		Roles: []string{"service"}, RequestID: "request-1", CorrelationID: "correlation-1", TokenID: "token-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyInternalCredential(token, issuer.PublicKey(), now, "pymes-v3", "accounting", "org_acme"); err != nil {
		t.Fatal(err)
	}
	if len(client.signCalls) != 2 {
		t.Fatalf("sign calls = %d, want startup verification plus JWT", len(client.signCalls))
	}
	for _, request := range client.signCalls {
		if request.GetName() != testKMSVersion1 || request.GetDigest() != nil ||
			request.GetDataCrc32C() == nil ||
			uint32(request.GetDataCrc32C().GetValue()) != crc32.Checksum(request.GetData(), castagnoliTable) {
			t.Fatalf("KMS request = %#v", request)
		}
	}
	encoded, err := issuer.JWKSJSON()
	if err != nil {
		t.Fatal(err)
	}
	var jwks struct {
		Keys []struct {
			KeyID string `json:"kid"`
		} `json:"keys"`
	}
	if err := json.Unmarshal([]byte(encoded), &jwks); err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 2 || jwks.Keys[0].KeyID != stableKMSKeyID(client.materials[testKMSVersion1].public) ||
		jwks.Keys[1].KeyID != stableKMSKeyID(client.materials[testKMSVersion2].public) {
		t.Fatalf("rotation JWKS = %s", encoded)
	}
}

func TestKMSServiceIssuerFailsClosedOnVersionAlgorithmAndIntegrity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		version    string
		mutateGet  func(*kmspb.PublicKey)
		mutateSign func(*kmspb.AsymmetricSignResponse)
	}{
		{name: "version alias", version: "projects/p/locations/l/keyRings/r/cryptoKeys/k/cryptoKeyVersions/latest"},
		{name: "wrong algorithm", version: testKMSVersion1, mutateGet: func(key *kmspb.PublicKey) {
			key.Algorithm = kmspb.CryptoKeyVersion_EC_SIGN_P256_SHA256
		}},
		{name: "public key checksum", version: testKMSVersion1, mutateGet: func(key *kmspb.PublicKey) {
			key.PemCrc32C = wrapperspb.Int64(1)
		}},
		{name: "request checksum not verified", version: testKMSVersion1, mutateSign: func(response *kmspb.AsymmetricSignResponse) {
			response.VerifiedDataCrc32C = false
		}},
		{name: "signature checksum", version: testKMSVersion1, mutateSign: func(response *kmspb.AsymmetricSignResponse) {
			response.SignatureCrc32C = wrapperspb.Int64(1)
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client := newFakeKMSClient(testKMSVersion1)
			client.mutateGet = test.mutateGet
			client.mutateSign = test.mutateSign
			if _, err := NewKMSServiceIssuer(context.Background(), "pymes-v3", test.version, nil, client, nil); err == nil {
				t.Fatal("expected KMS issuer validation failure")
			}
		})
	}
}

func newFakeKMSClient(versions ...string) *fakeKMSClient {
	client := &fakeKMSClient{materials: make(map[string]fakeKMSMaterial, len(versions))}
	for index, version := range versions {
		seed := make([]byte, ed25519.SeedSize)
		for position := range seed {
			seed[position] = byte(index*ed25519.SeedSize + position + 1)
		}
		private := ed25519.NewKeyFromSeed(seed)
		public := private.Public().(ed25519.PublicKey)
		der, err := x509.MarshalPKIXPublicKey(public)
		if err != nil {
			panic(err)
		}
		client.materials[version] = fakeKMSMaterial{
			private: private,
			public:  public,
			pem:     string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})),
		}
	}
	return client
}
