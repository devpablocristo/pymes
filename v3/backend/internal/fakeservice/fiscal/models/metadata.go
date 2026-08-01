// Package models contains request records owned by the fiscal fake.
package models

import fiscalapi "github.com/devpablocristo/pymes/v3/backend/internal/commerce/fiscal/models"

type Metadata struct {
	PathOrganizationID   string
	HeaderIdempotencyKey string
	HeaderCorrelationID  string
	BodyOrganizationID   string
	BodyIdempotencyKey   string
	BodyCorrelationID    string
}

type CredentialReplay struct {
	Request fiscalapi.CSRRequest
	Result  fiscalapi.CSRResult
}
