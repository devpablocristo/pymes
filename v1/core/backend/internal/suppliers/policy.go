// Package suppliers — Ola C step 9.
//
// ArchivePolicy for suppliers. See customers/policy.go for the multi-table
// adapter rationale.
package suppliers

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

const ResourceTypeSupplier = "supplier"

var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypeSupplier,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
