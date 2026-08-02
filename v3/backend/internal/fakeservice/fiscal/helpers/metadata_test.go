package helpers

import (
	"regexp"
	"testing"

	"github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/fiscal/models"
)

func TestMetadataMatchesRejectsHeaderMismatch(t *testing.T) {
	if MetadataMatches(models.Metadata{
		PathOrganizationID: "org", BodyOrganizationID: "org",
		HeaderIdempotencyKey: "a", BodyIdempotencyKey: "b",
	}) {
		t.Fatal("expected metadata mismatch")
	}
}

func TestCredentialIDUsesTheOpaqueFiscalContract(t *testing.T) {
	first := CredentialID("org", "homologation", "30712345678", "csr-key")
	second := CredentialID("org", "homologation", "30712345678", "csr-key")
	if first != second {
		t.Fatalf("credential ID is not deterministic: %q != %q", first, second)
	}
	if !regexp.MustCompile(`^fcred_[A-Za-z0-9_-]{8,80}$`).MatchString(first) {
		t.Fatalf("credential ID does not satisfy the public contract: %q", first)
	}
}
