// Package models contains records owned by the Clerk adapter.
package models

// SessionIdentity is the minimum verified Clerk session needed locally.
type SessionIdentity struct {
	OrganizationID string
	Subject        string
}
