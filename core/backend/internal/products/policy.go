// Package products — Ola C step 8.
//
// ArchivePolicy for products. See pricelists/policy.go for the canonical
// pattern documentation. ArchivedAtColumn remains "deleted_at" in the
// product table (rename to archived_at is a future coordinated migration).
package products

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

const ResourceTypeProduct = "product"

var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypeProduct,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
