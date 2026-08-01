package helpers

import (
	"testing"
	"time"

	fiscalapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/fiscal/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

func TestCredentialMappingsKeepTenantMetadataAndNeverInventPrivateMaterial(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	input := fiscalapi.CSRResult{
		Credential: fiscalapi.FiscalCredential{
			CommonName:     "tenant",
			CreatedAt:      now,
			Cuit:           "30712345678",
			Environment:    fiscalapi.FiscalCredentialEnvironmentHomologation,
			Id:             "credential",
			LegalName:      "Tenant SA",
			OrganizationId: "org",
			Status:         fiscalapi.FiscalCredentialStatusPendingCertificate,
			UpdatedAt:      now,
			Version:        1,
		},
		CsrPem: "public-csr",
	}
	result := CredentialCSRResult(input)
	if result.Credential.OrganizationID != "org" ||
		result.Credential.Environment != domain.FiscalEnvironmentHomologation ||
		result.CSRPEM != "public-csr" {
		t.Fatalf("unexpected mapping: %#v", result)
	}
}
