package commerce

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	publicapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/handler/dto"
	identityaccess "github.com/devpablocristo/pymes/v3/backend/internal/identity"
)

func TestFiscalOnboardingAgainstRealAdapter(t *testing.T) {
	baseURL := os.Getenv("PYMES_FISCAL_TEST_URL")
	if baseURL == "" {
		t.Skip("PYMES_FISCAL_TEST_URL is required")
	}
	tokens, err := identityaccess.TokenSourceFromRuntime(
		"api:fiscal-onboarding-contract-test",
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tokens.Close(); err != nil {
			t.Errorf("close internal token source: %v", err)
		}
	})

	organizationID := "org_fiscal_onboarding_contract"
	bff := NewHTTPServer(
		Commands{
			FiscalCredentials: HTTPFiscalClient{
				BaseURL: baseURL,
				Tokens:  tokens,
			},
			Features: commerceFeatureGate{enabled: true},
		},
		organizationAuthStub{organizationID: organizationID},
	).Handler()

	csrRecorder := serveFiscalOnboardingRequest(
		t,
		bff,
		http.MethodPost,
		"/api/v1/organizations/"+organizationID+"/fiscal/credentials/csr",
		`{"cuit":"20123456786","legal_name":"Pyme Contract SA","common_name":"pymes-contract-homologation","environment":"homologation"}`,
		"fiscal-onboarding-csr-contract",
		http.StatusCreated,
	)
	if strings.Contains(strings.ToLower(csrRecorder.Body.String()), "private") {
		t.Fatalf("CSR response exposed private material: %s", csrRecorder.Body.String())
	}
	var csr publicapi.FiscalCredentialCSRResult
	if err := json.Unmarshal(csrRecorder.Body.Bytes(), &csr); err != nil {
		t.Fatal(err)
	}
	credentialID := csr.Credential.Id
	if !regexp.MustCompile(`^fcred_[A-Za-z0-9_-]{8,80}$`).MatchString(credentialID) {
		t.Fatalf("credential ID drifted from the opaque contract: %q", credentialID)
	}
	if csr.CsrPem == nil {
		t.Fatal("Fiscal Adapter did not return a CSR")
	}

	certificatePEM := signFiscalContractCSR(
		t,
		*csr.CsrPem,
	)
	certificateBody, err := json.Marshal(publicapi.FiscalCertificateUpload{
		CertificatePem:  &certificatePEM,
		ExpectedVersion: csr.Credential.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	credentialPath := "/api/v1/organizations/" + organizationID +
		"/fiscal/credentials/" + credentialID
	certificateRecorder := serveFiscalOnboardingRequest(
		t,
		bff,
		http.MethodPut,
		credentialPath,
		string(certificateBody),
		"",
		http.StatusOK,
	)
	if strings.Contains(certificateRecorder.Body.String(), "certificate_pem") {
		t.Fatalf("certificate response reflected write-only PEM: %s", certificateRecorder.Body.String())
	}
	var uploaded publicapi.FiscalCredential
	if err := json.Unmarshal(certificateRecorder.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.Id != credentialID || uploaded.Status != "ready" {
		t.Fatalf("credential identity/status drifted after certificate upload: %#v", uploaded)
	}

	pointPath := credentialPath + "/points-of-sale/1"
	configuredRecorder := serveFiscalOnboardingRequest(
		t,
		bff,
		http.MethodPut,
		pointPath,
		`{"enabled":false}`,
		"",
		http.StatusOK,
	)
	var configured publicapi.FiscalPointOfSale
	if err := json.Unmarshal(configuredRecorder.Body.Bytes(), &configured); err != nil {
		t.Fatal(err)
	}
	if configured.CredentialId != credentialID || configured.Enabled {
		t.Fatalf("configured point of sale drifted from credential: %#v", configured)
	}

	validatedRecorder := serveFiscalOnboardingRequest(
		t,
		bff,
		http.MethodPost,
		pointPath+"/validate",
		`{"enabled":true}`,
		"",
		http.StatusOK,
	)
	var validated publicapi.FiscalPointOfSale
	if err := json.Unmarshal(validatedRecorder.Body.Bytes(), &validated); err != nil {
		t.Fatal(err)
	}
	if validated.CredentialId != credentialID ||
		validated.OrganizationId != organizationID ||
		!validated.Enabled ||
		validated.ValidatedAt == nil {
		t.Fatalf("validated point of sale drifted from the public contract: %#v", validated)
	}

	getRecorder := serveFiscalOnboardingRequest(
		t,
		bff,
		http.MethodGet,
		credentialPath,
		"",
		"",
		http.StatusOK,
	)
	var fetched publicapi.FiscalCredential
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.Id != credentialID || fetched.OrganizationId != organizationID {
		t.Fatalf("fetched credential drifted from onboarding identity: %#v", fetched)
	}
}

func serveFiscalOnboardingRequest(
	t *testing.T,
	handler http.Handler,
	method,
	path,
	body,
	idempotencyKey string,
	expectedStatus int,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("X-Request-ID", "request-fiscal-onboarding-contract")
	request.Header.Set("X-Correlation-ID", "correlation-fiscal-onboarding-contract")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != expectedStatus {
		t.Fatalf(
			"%s %s status=%d expected=%d body=%s",
			method,
			path,
			recorder.Code,
			expectedStatus,
			recorder.Body.String(),
		)
	}
	return recorder
}

func signFiscalContractCSR(t *testing.T, csrPEM string) string {
	t.Helper()
	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		t.Fatal("Fiscal Adapter returned an invalid CSR PEM")
	}
	request, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := request.CheckSignature(); err != nil {
		t.Fatalf("CSR signature: %v", err)
	}
	issuerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	issuerTemplate := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Pymes Fiscal Contract Test CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	issuerDER, err := x509.CreateCertificate(
		rand.Reader,
		&issuerTemplate,
		&issuerTemplate,
		&issuerKey.PublicKey,
		issuerKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := x509.ParseCertificate(issuerDER)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      request.Subject,
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(12 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(
		rand.Reader,
		&leafTemplate,
		issuer,
		request.PublicKey,
		issuerKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	var certificate bytes.Buffer
	if err := pem.Encode(
		&certificate,
		&pem.Block{Type: "CERTIFICATE", Bytes: leafDER},
	); err != nil {
		t.Fatal(err)
	}
	return certificate.String()
}
