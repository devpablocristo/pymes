// Ola C step 3 piloto — wire helper for pricelists.
//
// Builds a platform/lifecycle/go.Service scoped to the "price_list" resource
// type. Other CRUDAR modules will add their own builders here (or a shared
// service grouping multiple resource types) as they opt in.
//
// Audit is currently a no-op until the LifecycleAdapter shim from
// platform/kernels/activity/go is wired against the pymes audit subsystem.
// That follow-up is tracked in pymes/docs/CRUDAR_REFACTOR.md.
package wire

import (
	"context"
	"log"

	lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
	"github.com/devpablocristo/pymes/core/backend/internal/pricelists"
	"gorm.io/gorm"
)

// noopAuditPort is the temporary AuditPort used until the activity-audit
// integration is wired. It implements lifecycle.AuditPort by discarding the
// entry (logs a single warning per process to make the gap visible).
type noopAuditPort struct{ warned bool }

func (a *noopAuditPort) Append(_ context.Context, _ lifecycle.ArchiveAudit) error {
	if !a.warned {
		log.Println("warn: lifecycle.AuditPort is noop — wire activity-audit adapter to enable audit trail")
		a.warned = true
	}
	return nil
}

// buildPriceListsLifecycleService wires a Service that handles archive /
// restore / hard-delete for the "price_list" resource type. Returns nil
// (legacy path) on any construction error so the system keeps booting.
func buildPriceListsLifecycleService(gdb *gorm.DB) *lifecycle.Service {
	if gdb == nil {
		return nil
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		log.Printf("warn: pricelists lifecycle wire skipped (gdb.DB(): %v)", err)
		return nil
	}

	// price_lists table uses `deleted_at` for the soft-delete column (pymes
	// has not yet renamed to `archived_at`). Parametrize via SoftDeleterConfig.
	sd, err := lifecycle.NewSoftDeleter(sqlDB, lifecycle.SoftDeleterConfig{
		Table:            "price_lists",
		IDColumn:         "id",
		TenantColumn:     "org_id",
		ArchivedAtColumn: "deleted_at",
	})
	if err != nil {
		log.Printf("warn: pricelists lifecycle wire skipped (NewSoftDeleter: %v)", err)
		return nil
	}

	registry := lifecycle.NewStaticPolicyRegistry(pricelists.Policy)
	svc, err := lifecycle.NewServiceWithRepos(
		map[string]lifecycle.RepositoryPort{
			pricelists.ResourceTypePriceList: sd,
		},
		&noopAuditPort{},
		registry,
	)
	if err != nil {
		log.Printf("warn: pricelists lifecycle wire skipped (NewServiceWithRepos: %v)", err)
		return nil
	}
	return svc
}

