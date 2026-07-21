// Package recurring — Ola C step 5.
//
// ArchivePolicy for recurring expenses. See pricelists/policy.go for the
// canonical pattern documentation.
package recurring

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

const ResourceTypeRecurringExpense = "recurring_expense"

var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypeRecurringExpense,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
