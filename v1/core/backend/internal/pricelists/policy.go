// Package pricelists — Ola C step 2 piloto.
//
// This file declares the lifecycle.ArchivePolicy and a constructor for a
// dedicated lifecycle.Service for price_list resources. It is the first
// pymes-core module to formally adopt platform/lifecycle/go.
//
// Wiring (bootstrap.go) — see docs/CRUDAR_REFACTOR.md for the canonical
// pattern. For now this is opt-in: the existing Usecases continues to work
// (the legacy SoftDelete/Restore/HardDelete via repository directly are
// untouched); a follow-up commit will switch the Usecases to delegate to
// Service.
package pricelists

import (
	lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
)

// ResourceTypePriceList is the opaque string used in lifecycle audit and policy.
const ResourceTypePriceList = "price_list"

// Policy is the lifecycle.ArchivePolicy for pricelists.
//
//   - AllowArchive=true:    pricelists can be soft-deleted.
//   - AllowHardDelete=true: hard-delete is permitted (admin override).
//   - RequireReason=false:  archiving does not require a reason.
//   - RetentionDays=0:      archived rows are retained forever (no purge).
//
// ValidateRelations is intentionally nil for the piloto. A follow-up can wire
// it to reject archive when the price list is referenced by an open quote or
// active product line.
var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypePriceList,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
