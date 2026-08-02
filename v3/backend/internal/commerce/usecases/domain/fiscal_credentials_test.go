package domain

import "testing"

func TestFiscalCredentialInputsValidateTenantOnboardingData(t *testing.T) {
	t.Parallel()

	valid := FiscalCredentialCSRInput{
		CUIT:        "30712345678",
		LegalName:   "Pyme SA",
		CommonName:  "pyme-homologacion",
		Environment: FiscalEnvironmentHomologation,
	}
	if !valid.Valid() {
		t.Fatal("expected valid CSR input")
	}
	for _, invalid := range []FiscalCredentialCSRInput{
		{},
		{CUIT: "3071234567x", LegalName: valid.LegalName, CommonName: valid.CommonName, Environment: valid.Environment},
		{CUIT: valid.CUIT, LegalName: valid.LegalName, CommonName: valid.CommonName, Environment: "shared"},
	} {
		if invalid.Valid() {
			t.Fatalf("unexpected valid CSR input: %#v", invalid)
		}
	}
	if !(FiscalCertificateUpload{CertificatePEM: "certificate", ExpectedVersion: 1}).Valid() {
		t.Fatal("expected valid certificate upload")
	}
}
