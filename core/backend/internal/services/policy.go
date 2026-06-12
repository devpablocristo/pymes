// Package services — Ola C step 8.
//
// ArchivePolicy for services. See pricelists/policy.go for the canonical
// pattern documentation. The services table uses archived_at as the canonical
// archive column after migration 0020.
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
