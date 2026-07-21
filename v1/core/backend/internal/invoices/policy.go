// Package invoices — Ola C step 7.
//
// ArchivePolicy for invoices. The FSM (draft/sent/paid/voided…) and the
// archive lifecycle are orthogonal flows; this policy governs only archive,
// restore and hard-delete. State transitions continue to be enforced by
// fsm.MapDomainError inside UpdateStatus.
package invoices

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

const ResourceTypeInvoice = "invoice"

var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypeInvoice,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
