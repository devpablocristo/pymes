// Package services — Ola C step 8.
//
// ArchivePolicy for services. See pricelists/policy.go for the canonical
// pattern documentation. ArchivedAtColumn remains "deleted_at" in the
// services table (rename to archived_at is a future coordinated migration).
package services

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

const ResourceTypeService = "service"

var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypeService,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
