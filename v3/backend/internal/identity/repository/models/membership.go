// Package models contains PostgreSQL records owned by the identity repository.
package models

// Membership mirrors the selected membership projection without leaking SQL
// details into identity use cases.
type Membership struct {
	Role               string
	PermissionsJSON    string
	Status             string
	OrganizationName   string
	OrganizationSlug   string
	OrganizationStatus string
}
