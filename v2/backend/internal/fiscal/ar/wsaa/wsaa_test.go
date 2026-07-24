package wsaa

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"
)

type testKMS struct {
	key *rsa.PrivateKey
}

func (kms testKMS) Encrypt(context.Context, string, []byte, []byte) ([]byte, error) {
	panic("not used")
}
func (kms testKMS) Decrypt(context.Context, string, []byte, []byte) ([]byte, error) {
	panic("not used")
}
func (kms testKMS) PublicKey(context.Context, string) (crypto.PublicKey, error) {
	return &kms.key.PublicKey, nil
}
func (kms testKMS) SignDigest(
	_ context.Context,
	_ string,
	digest []byte,
	hash crypto.Hash,
) ([]byte, error) {
	return rsa.SignPKCS1v15(rand.Reader, kms.key, hash, digest)
}

func certificateFixture(t *testing.T) ([]byte, *rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject: pkix.Name{
			CommonName:   "Pymes test",
			SerialNumber: "CUIT 30-00000000-7",
		},
		NotBefore: now.Add(-time.Hour),
		NotAfter:  now.Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}), key, certificate
}

func TestBuildTRAEscapesAndUsesShortWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("ART", -3*60*60))
	tra, err := BuildTRA(ServiceWSFE, now)
	if err != nil {
		t.Fatal(err)
	}
	text := string(tra)
	for _, expected := range []string{
		"<uniqueId>1784905200</uniqueId>",
		"<generationTime>2026-07-24T11:50:00-03:00</generationTime>",
		"<expirationTime>2026-07-24T12:10:00-03:00</expirationTime>",
		"<service>wsfe</service>",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("TRA does not contain %q: %s", expected, text)
		}
	}
	if _, err := BuildTRA("wsfe</service>", now); err == nil {
		t.Fatal("expected invalid service to be rejected")
	}
}

func TestSignTRAWithKMSProducesVerifiableCMS(t *testing.T) {
	t.Parallel()

	certificatePEM, key, certificate := certificateFixture(t)
	tra := []byte("<loginTicketRequest/>")
	encoded, err := SignTRAWithKMS(context.Background(), tra, certificatePEM, "key", testKMS{key: key})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var outer contentInfo
	if _, err := asn1.Unmarshal(raw, &outer); err != nil {
		t.Fatalf("parse outer CMS: %v", err)
	}
	if !outer.ContentType.Equal(oidSignedData) {
		t.Fatalf("content type = %v", outer.ContentType)
	}
	var signed signedData
	if _, err := asn1.Unmarshal(outer.Content.Bytes, &signed); err != nil {
		t.Fatalf("parse SignedData: %v", err)
	}
	if len(signed.SignerInfos) != 1 {
		t.Fatalf("signer count = %d", len(signed.SignerInfos))
	}
	digest := sha256.Sum256(tra)
	if err := rsa.VerifyPKCS1v15(
		certificate.PublicKey.(*rsa.PublicKey),
		crypto.SHA256,
		digest[:],
		signed.SignerInfos[0].Signature,
	); err != nil {
		t.Fatalf("verify CMS signature: %v", err)
	}
}

func TestParseLoginResponseFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/login_response.xml")
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := ParseLoginResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Token != "fixture-token" || ticket.Sign != "fixture-sign" {
		t.Fatalf("unexpected ticket credentials")
	}
	if got, want := ticket.ExpiresAt.Format(time.RFC3339), "2026-07-25T06:00:00Z"; got != want {
		t.Fatalf("expiration = %s, want %s", got, want)
	}
}
