package helpers

import (
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

func TestPublicFiscalCredentialContainsMetadataButNoPrivateKey(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	value, err := PublicFiscalCredential(domain.FiscalCredential{
		ID:             "8c17eac2-679b-4e2c-a4f4-7c4fc7a8cfc8",
		OrganizationID: "org",
		CUIT:           "30712345678",
		LegalName:      "Pyme SA",
		CommonName:     "pyme",
		Environment:    domain.FiscalEnvironmentHomologation,
		Status:         domain.FiscalCredentialReady,
		Version:        2,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		t.Fatalf("map credential: %v", err)
	}
	if value.OrganizationId != "org" || value.Cuit != "30712345678" || value.Version != 2 {
		t.Fatalf("unexpected public credential: %#v", value)
	}
}
