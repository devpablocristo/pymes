// Package payments — Ola C step 5.
//
// ArchivePolicy for payments. See pricelists/policy.go for the canonical
// pattern documentation.
package payments

import lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"

const ResourceTypePayment = "payment"

var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypePayment,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
