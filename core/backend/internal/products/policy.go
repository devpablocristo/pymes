// Package products — Ola C step 8.
//
// ArchivePolicy for products. See pricelists/policy.go for the canonical
// pattern documentation. The product table uses archived_at as the canonical
// archive column after migration 0020.
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
