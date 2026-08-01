package commerce

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	publicapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/handler/dto"
	"github.com/devpablocristo/pymes/v3/backend/internal/fakeservice"
)

func TestFiscalOnboardingTraversesBFFWithoutExposingPrivateKeyMaterial(t *testing.T) {
	t.Parallel()

	fiscalHandler, err := fakeservice.HandlerForKind("fiscal")
	if err != nil {
		t.Fatal(err)
	}
	fiscalServer := httptest.NewServer(fiscalHandler)
	t.Cleanup(fiscalServer.Close)

	organizationID := "org_fiscal_bff"
	bff := NewHTTPServer(
		Commands{
			FiscalCredentials: HTTPFiscalClient{
				BaseURL: fiscalServer.URL,
				Client:  fiscalServer.Client(),
			},
		},
		organizationAuthStub{organizationID: organizationID},
	).Handler()

	csrRecorder := httptest.NewRecorder()
	csrRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/organizations/"+organizationID+"/fiscal/credentials/csr",
		strings.NewReader(`{"cuit":"30712345678","legal_name":"Pyme SA","common_name":"pyme-homologacion","environment":"homologation"}`),
	)
	csrRequest.Header.Set("Content-Type", "application/json")
	csrRequest.Header.Set("Idempotency-Key", "csr-bff-1")
	bff.ServeHTTP(csrRecorder, csrRequest)
	if csrRecorder.Code != http.StatusCreated {
		t.Fatalf("CSR status=%d body=%s", csrRecorder.Code, csrRecorder.Body.String())
	}
	if strings.Contains(strings.ToLower(csrRecorder.Body.String()), "private") {
		t.Fatalf("CSR response exposed private material: %s", csrRecorder.Body.String())
	}
	var csr publicapi.FiscalCredentialCSRResult
	if err := json.Unmarshal(csrRecorder.Body.Bytes(), &csr); err != nil {
		t.Fatal(err)
	}
	if csr.CsrPem == nil || !strings.Contains(*csr.CsrPem, "BEGIN CERTIFICATE REQUEST") {
		t.Fatalf("invalid public CSR: %#v", csr)
	}
	credentialID := csr.Credential.Id.String()

	certificateBody, err := json.Marshal(publicapi.FiscalCertificateUpload{
		CertificatePem:  optionalString("-----BEGIN CERTIFICATE-----\nFAKE\n-----END CERTIFICATE-----\n"),
		ExpectedVersion: csr.Credential.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	certificateRecorder := httptest.NewRecorder()
	certificateRequest := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/organizations/"+organizationID+"/fiscal/credentials/"+credentialID,
		bytes.NewReader(certificateBody),
	)
	certificateRequest.Header.Set("Content-Type", "application/json")
	bff.ServeHTTP(certificateRecorder, certificateRequest)
	if certificateRecorder.Code != http.StatusOK {
		t.Fatalf("certificate status=%d body=%s", certificateRecorder.Code, certificateRecorder.Body.String())
	}
	if strings.Contains(certificateRecorder.Body.String(), "certificate_pem") {
		t.Fatalf("certificate response reflected write-only PEM: %s", certificateRecorder.Body.String())
	}

	pointPath := "/api/v1/organizations/" + organizationID +
		"/fiscal/credentials/" + credentialID + "/points-of-sale/1"
	configureRecorder := httptest.NewRecorder()
	configureRequest := httptest.NewRequest(
		http.MethodPut,
		pointPath,
		strings.NewReader(`{"enabled":false}`),
	)
	configureRequest.Header.Set("Content-Type", "application/json")
	bff.ServeHTTP(configureRecorder, configureRequest)
	if configureRecorder.Code != http.StatusOK {
		t.Fatalf("configure status=%d body=%s", configureRecorder.Code, configureRecorder.Body.String())
	}

	validateRecorder := httptest.NewRecorder()
	validateRequest := httptest.NewRequest(
		http.MethodPost,
		pointPath+"/validate",
		strings.NewReader(`{"enabled":true}`),
	)
	validateRequest.Header.Set("Content-Type", "application/json")
	bff.ServeHTTP(validateRecorder, validateRequest)
	if validateRecorder.Code != http.StatusOK {
		t.Fatalf("validate status=%d body=%s", validateRecorder.Code, validateRecorder.Body.String())
	}
	var point publicapi.FiscalPointOfSale
	if err := json.Unmarshal(validateRecorder.Body.Bytes(), &point); err != nil {
		t.Fatal(err)
	}
	if !point.Enabled || point.ValidatedAt == nil ||
		point.OrganizationId != organizationID ||
		point.CredentialId.String() != credentialID {
		t.Fatalf("invalid validated point of sale: %#v", point)
	}

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/organizations/"+organizationID+"/fiscal/credentials/"+credentialID,
		nil,
	)
	bff.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get credential status=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
}
