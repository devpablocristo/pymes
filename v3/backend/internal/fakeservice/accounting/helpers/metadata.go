// Package helpers contains envelope validation for the accounting fake.
package helpers

import "github.com/devpablocristo/pymes/v3/backend/internal/fakeservice/accounting/models"

func MetadataMatches(value models.Metadata) bool {
	return value.PathOrganizationID == value.BodyOrganizationID &&
		value.HeaderIdempotencyKey == value.BodyIdempotencyKey &&
		value.HeaderCorrelationID == value.BodyCorrelationID
}
