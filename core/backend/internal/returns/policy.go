// Package returns — Ola C step 5.
//
// ArchivePolicy for returns. The restore method is exposed as
// RestoreArchived (legacy convention in this package); the lifecycle.Service
// integration in WithLifecycle uses the standard Restore semantics.
package returns

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

const ResourceTypeReturn = "return"

var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypeReturn,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
