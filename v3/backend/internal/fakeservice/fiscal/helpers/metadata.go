// Package helpers contains envelope validation for the fiscal fake.
package helpers

import "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/fiscal/models"

func MetadataMatches(value models.Metadata) bool {
	return value.PathOrganizationID == value.BodyOrganizationID &&
		value.HeaderIdempotencyKey == value.BodyIdempotencyKey &&
		value.HeaderCorrelationID == value.BodyCorrelationID
}
