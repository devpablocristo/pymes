// Package cashflow — Ola C step 5.
//
// ArchivePolicy for cash movements. See pricelists/policy.go for the
// canonical pattern documentation.
package cashflow

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

// ResourceTypeCashMovement uses "cashflow" (not "cash_movement") to match the
// legacy pymes audit naming where actions are "cashflow.archived",
// "cashflow.restored", etc. (cashflow is the domain name; cash_movement is
// the storage table). The ResourceType string is opaque to platform/lifecycle
// (§ Invariante I1), so consumers choose the vocabulary.
const ResourceTypeCashMovement = "cashflow"

var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypeCashMovement,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
