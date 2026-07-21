// Package customers — Ola C step 9.
//
// ArchivePolicy for customers. The Customers and Suppliers modules share
// the `parties` table (with a type discriminator) and their SoftDelete /
// Restore / HardDelete run multi-table transactions (party_roles,
// party_persons, accounts). Therefore we cannot use the generic
// lifecycle.SoftDeleter — the wire site supplies a custom RepositoryPort
// adapter that delegates to the existing Repository.
package customers

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

const ResourceTypeCustomer = "customer"

var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypeCustomer,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
