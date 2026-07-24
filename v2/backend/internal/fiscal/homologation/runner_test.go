package homologation

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"html"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/artifacts"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsfev1"
	"github.com/google/uuid"
)

func TestRunnerUsesOnlyReadOnlyARCAOperationsAndPersistsRedactedEvidence(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	organizationID := uuid.MustParse("018fe915-aaba-77b0-a55c-68427afe1e77")
	privateKey, certificatePEM, fingerprint := homologationCertificate(t, now, "30000000007")
	repository := &memoryRepository{
		runID: uuid.MustParse("018fe915-aaba-77b0-a55c-68427afe1e78"),
		configuration: Configuration{
			LegalName:              "Pyme Homologación SRL",
			LegalAddress:           "CABA",
			ActivityStartDate:      time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
			ProfileVersion:         1,
			CUIT:                   "30000000007",
			IssuerVATCondition:     "responsable_inscripto",
			SettingsVersion:        1,
			CertificateReference:   "objects/certificate.pem",
			PrivateKeyReference:    "secret://test/key",
			CertificateFingerprint: fingerprint,
			CertificateValidFrom:   now.Add(-time.Hour),
			CertificateValidUntil:  now.Add(24 * time.Hour),
			PointsOfSale: []PointOfSale{
				{Code: 3, Name: "Homologación 3"},
				{Code: 7, Name: "Homologación 7"},
			},
		},
	}
	transport := &readOnlyTransport{now: now}
	kms := &memoryKMS{privateKey: privateKey}
	runner, err := NewRunner(
		repository,
		kms,
		&memoryObjects{
			key: "objects/certificate.pem",
			object: fiscal.ImmutableObject{
				Key: "objects/certificate.pem", ContentType: "application/x-pem-file",
				Body: certificatePEM,
			},
		},
		artifacts.NewRenderer(),
		transport,
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Run(context.Background(), Command{
		OrganizationID: organizationID,
		RequestedBy:    "test:accountant",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusSucceeded || result.FailureCount != 0 ||
		result.CheckCount != 30 || result.SuccessCount != 30 {
		t.Fatalf("result = %+v", result)
	}
	if transport.wsaaCalls != 1 {
		t.Fatalf("WSAA calls = %d, want 1", transport.wsaaCalls)
	}
	if transport.lastAuthorizedCalls != 18 {
		t.Fatalf(
			"FECompUltimoAutorizado calls = %d, want 18",
			transport.lastAuthorizedCalls,
		)
	}
	if transport.emissionAttempted {
		t.Fatal("homologation runner attempted an emissive ARCA operation")
	}
	if repository.completion.Status != StatusSucceeded ||
		repository.completion.CertificateFingerprint != fingerprint {
		t.Fatalf("completion = %+v", repository.completion)
	}

	serialized, err := json.Marshal(repository.completion)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"fixture-token", "fixture-sign", "PRIVATE KEY"} {
		if bytes.Contains(serialized, []byte(secret)) {
			t.Fatalf("persisted homologation evidence contains secret %q", secret)
		}
	}

	concepts := map[int]bool{}
	currencies := map[string]bool{}
	voucherTypes := map[int]bool{}
	associatedCount := 0
	localCount := 0
	for _, check := range repository.completion.Checks {
		if check.Kind != CheckLocalMatrix {
			continue
		}
		localCount++
		var evidence localMatrixEvidence
		if err := json.Unmarshal(check.Evidence, &evidence); err != nil {
			t.Fatal(err)
		}
		concepts[evidence.Concept] = true
		currencies[evidence.Currency] = true
		voucherTypes[evidence.VoucherType] = true
		if evidence.HasAssociation {
			associatedCount++
		}
		if !evidence.Deterministic || evidence.NetworkEmission ||
			evidence.ArtifactSHA256["pdf"] == "" ||
			evidence.ArtifactSHA256["qr"] == "" {
			t.Fatalf("incomplete local evidence = %+v", evidence)
		}
	}
	if localCount != 9 || len(voucherTypes) != 9 || len(concepts) != 3 ||
		!currencies[ar.CurrencyPES] || !currencies[ar.CurrencyDOL] ||
		associatedCount != 6 {
		t.Fatalf(
			"local matrix coverage: count=%d types=%v concepts=%v currencies=%v associations=%d",
			localCount, voucherTypes, concepts, currencies, associatedCount,
		)
	}
}

func TestRunnerFinalizesFailureWithoutLeakingTransportDetails(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	privateKey, certificatePEM, fingerprint := homologationCertificate(t, now, "30000000007")
	repository := &memoryRepository{
		runID: uuid.New(),
		configuration: Configuration{
			LegalName: "Test SA", LegalAddress: "CABA",
			ActivityStartDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			ProfileVersion:    1,
			CUIT:              "30000000007", IssuerVATCondition: "responsable_inscripto",
			SettingsVersion:      1,
			CertificateReference: "certificate", PrivateKeyReference: "secret://test/key",
			CertificateFingerprint: fingerprint,
			PointsOfSale:           []PointOfSale{{Code: 1, Name: "One"}},
		},
	}
	transport := &readOnlyTransport{
		now: now, lastAuthorizedError: errors.New("remote detail fixture-token fixture-sign"),
	}
	runner, err := NewRunner(
		repository,
		&memoryKMS{privateKey: privateKey},
		&memoryObjects{
			key:    "certificate",
			object: fiscal.ImmutableObject{Key: "certificate", Body: certificatePEM},
		},
		artifacts.NewRenderer(),
		transport,
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	result, runErr := runner.Run(context.Background(), Command{
		OrganizationID: uuid.New(), RequestedBy: "test",
	})
	if runErr == nil || result.Status != StatusFailed ||
		repository.completion.Status != StatusFailed {
		t.Fatalf("result=%+v err=%v completion=%+v", result, runErr, repository.completion)
	}
	if strings.Contains(runErr.Error(), "fixture-token") ||
		strings.Contains(runErr.Error(), "fixture-sign") {
		t.Fatal("remote secrets leaked into the command error")
	}
	if transport.emissionAttempted {
		t.Fatal("failure path attempted an emissive ARCA operation")
	}
	serialized, _ := json.Marshal(repository.completion)
	if bytes.Contains(serialized, []byte("fixture-token")) ||
		bytes.Contains(serialized, []byte("fixture-sign")) {
		t.Fatal("remote error details leaked into persisted evidence")
	}
}

type memoryRepository struct {
	runID         uuid.UUID
	configuration Configuration
	completion    Completion
}

func (repository *memoryRepository) Start(
	_ context.Context,
	_ uuid.UUID,
	_ string,
	_ time.Time,
) (uuid.UUID, error) {
	return repository.runID, nil
}

func (repository *memoryRepository) LoadConfiguration(
	_ context.Context,
	_ uuid.UUID,
	_ time.Time,
) (Configuration, error) {
	return repository.configuration, nil
}

func (repository *memoryRepository) Complete(
	_ context.Context,
	_, _ uuid.UUID,
	completion Completion,
) error {
	repository.completion = completion
	return nil
}

type memoryObjects struct {
	key    string
	object fiscal.ImmutableObject
}

func (objects *memoryObjects) PutImmutable(context.Context, fiscal.ImmutableObject) error {
	return errors.New("not implemented")
}

func (objects *memoryObjects) Get(
	_ context.Context,
	key string,
) (fiscal.ImmutableObject, error) {
	if key != objects.key {
		return fiscal.ImmutableObject{}, fiscal.ErrNotFound
	}
	return objects.object, nil
}

type memoryKMS struct {
	privateKey *rsa.PrivateKey
}

func (kms *memoryKMS) Encrypt(
	_ context.Context,
	_ string,
	plaintext, _ []byte,
) ([]byte, error) {
	return append([]byte(nil), plaintext...), nil
}

func (kms *memoryKMS) Decrypt(
	_ context.Context,
	_ string,
	ciphertext, _ []byte,
) ([]byte, error) {
	return append([]byte(nil), ciphertext...), nil
}

func (kms *memoryKMS) PublicKey(context.Context, string) (crypto.PublicKey, error) {
	return &kms.privateKey.PublicKey, nil
}

func (kms *memoryKMS) SignDigest(
	_ context.Context,
	_ string,
	digest []byte,
	hash crypto.Hash,
) ([]byte, error) {
	return rsa.SignPKCS1v15(rand.Reader, kms.privateKey, hash, digest)
}

type readOnlyTransport struct {
	now                 time.Time
	wsaaCalls           int
	lastAuthorizedCalls int
	emissionAttempted   bool
	lastAuthorizedError error
}

func (transport *readOnlyTransport) Call(
	_ context.Context,
	request ar.SOAPRequest,
) ([]byte, error) {
	if strings.Contains(request.Endpoint, "wsaahomo") {
		transport.wsaaCalls++
		inner := fmt.Sprintf(
			`<loginTicketResponse version="1.0"><header><expirationTime>%s</expirationTime></header><credentials><token>fixture-token</token><sign>fixture-sign</sign></credentials></loginTicketResponse>`,
			transport.now.Add(time.Hour).Format(time.RFC3339),
		)
		return []byte(fmt.Sprintf(
			`<Envelope><Body><loginCmsResponse><loginCmsReturn>%s</loginCmsReturn></loginCmsResponse></Body></Envelope>`,
			html.EscapeString(inner),
		)), nil
	}
	if request.Action == wsfev1.Namespace+"FECompUltimoAutorizado" {
		transport.lastAuthorizedCalls++
		if bytes.Contains(request.Envelope, []byte("FECAESolicitar")) {
			transport.emissionAttempted = true
			return nil, errors.New("emissive envelope sent to read-only operation")
		}
		if transport.lastAuthorizedError != nil {
			return nil, transport.lastAuthorizedError
		}
		return []byte(
			`<Envelope><Body><FECompUltimoAutorizadoResponse><FECompUltimoAutorizadoResult><CbteNro>42</CbteNro></FECompUltimoAutorizadoResult></FECompUltimoAutorizadoResponse></Body></Envelope>`,
		), nil
	}
	transport.emissionAttempted = true
	return nil, fmt.Errorf("unexpected ARCA SOAP action %q", request.Action)
}

func homologationCertificate(
	t *testing.T,
	now time.Time,
	cuit string,
) (*rsa.PrivateKey, []byte, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Homologation test",
			SerialNumber: cuit,
		},
		NotBefore: now.Add(-time.Hour),
		NotAfter:  now.Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(
		rand.Reader, template, template, &privateKey.PublicKey, privateKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(der)
	return privateKey, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: der,
	}), hex.EncodeToString(sum[:])
}
