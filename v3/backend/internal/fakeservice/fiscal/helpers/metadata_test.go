package helpers

import (
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
