// Package quotes — Ola C step 7.
//
// ArchivePolicy for quotes. The FSM (draft/sent/accepted/rejected/expired)
// and the archive lifecycle are orthogonal flows; this policy governs only
// archive, restore and hard-delete. State transitions continue to be
// enforced by fsm.MapDomainError inside UpdateStatus.
//
// The legacy method names on Usecases are Archive/Restore/HardDelete (not
// SoftDelete). When wired via WithLifecycle, Archive forwards to
// lifecycle.Service.SoftDelete preserving the API surface.
package quotes

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

const ResourceTypeQuote = "quote"

var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypeQuote,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
