// Package employees — Ola C step 5.
//
// ArchivePolicy for the "employee" resource. Once the module's Usecases is
// constructed via WithLifecycle, soft-delete / restore / hard-delete are
// routed through platform/lifecycle/go.Service which enforces this policy
// and emits canonical audit entries via pymesAuditPort.
package employees

import (
	lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
)

// ResourceTypeEmployee is the opaque string used in lifecycle audit and policy.
const ResourceTypeEmployee = "employee"

// Policy is the lifecycle.ArchivePolicy for employees.
//
//   - AllowArchive=true:    employees can be soft-deleted.
//   - AllowHardDelete=true: hard-delete permitted (admin override).
//   - RequireReason=false:  no reason required to archive.
//   - RetentionDays=0:      archived rows retained forever.
//
// ValidateRelations is intentionally nil. A follow-up can reject archive
// when the employee is referenced by an active payroll cycle / scheduling
// shift / open work order. For the piloto we keep it permissive.
var Policy = &lifecycle.ArchivePolicy{
	ResourceType:    ResourceTypeEmployee,
	AllowArchive:    true,
	AllowHardDelete: true,
	RequireReason:   false,
	RetentionDays:   0,
}
