// Package models contains persistence-only records for the commerce repository.
package models

// ServiceResponseMetadata is the immutable identity stored with a private
// service response before it is applied to commerce state.
type ServiceResponseMetadata struct {
	OrganizationID string
	Service        string
	Operation      string
	RequestID      string
	IdempotencyKey string
	SourceVersion  int
	SnapshotDigest string
	CorrelationID  string
}
