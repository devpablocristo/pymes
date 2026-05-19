// Package purchases — Ola C step 8.
//
// ArchivePolicy for purchases. The FSM and the archive lifecycle are
// orthogonal flows; this policy governs only archive, restore and
// hard-delete. State transitions continue to be enforced by
// fsm.MapDomainError inside UpdateStatus.
//
// ArchivedAtColumn remains "deleted_at" in the purchases table (rename to
// archived_at is a future coordinated migration).
package purchases

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

const ResourceTypePurchase = "purchase"

var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypePurchase,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
