// Package models contains request records owned by the accounting fake.
package models

type Metadata struct {
	PathOrganizationID   string
	HeaderIdempotencyKey string
	HeaderCorrelationID  string
	BodyOrganizationID   string
	BodyIdempotencyKey   string
	BodyCorrelationID    string
}
