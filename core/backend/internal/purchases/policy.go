// Package purchases — Ola C step 8.
//
// ArchivePolicy for purchases. The FSM and the archive lifecycle are
// orthogonal flows; this policy governs only archive, restore and
// hard-delete. State transitions continue to be enforced by
// fsm.MapDomainError inside UpdateStatus.
//
// The purchases table uses archived_at as the canonical archive column after
// migration 0020.
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
