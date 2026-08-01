package helpers

import (
	"testing"

	"github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/accounting/models"
)

func TestMetadataMatchesRejectsBodyMismatch(t *testing.T) {
	if MetadataMatches(models.Metadata{PathOrganizationID: "a", BodyOrganizationID: "b"}) {
		t.Fatal("expected metadata mismatch")
	}
}
