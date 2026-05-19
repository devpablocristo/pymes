// Ola C steps 3-5 — shared platform/lifecycle/go.Service wiring for pymes.
//
// A single lifecycle.Service is constructed at boot from the union of all
// LifecycleEntry registrations declared in this file. Each CRUDAR module
// that opts in registers its ArchivePolicy + table layout via
// register*Lifecycle helpers; the Service is then injected into every
// module's Usecases through WithLifecycle.
//
// Audit is routed through pymesAuditPort, which wraps the canonical
// pymes audit.Usecases (hash chain v2 + audit_log table).
//
// See docs/CRUDAR_REFACTOR.md for the migration plan per module.
package wire

import (
	"context"
	"log"

	lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
	"github.com/devpablocristo/pymes/core/backend/internal/audit"
	auditdomain "github.com/devpablocristo/pymes/core/backend/internal/audit/usecases/domain"
	"github.com/devpablocristo/pymes/core/backend/internal/cashflow"
	"github.com/devpablocristo/pymes/core/backend/internal/customers"
	"github.com/devpablocristo/pymes/core/backend/internal/employees"
	"github.com/devpablocristo/pymes/core/backend/internal/invoices"
	"github.com/devpablocristo/pymes/core/backend/internal/payments"
	"github.com/devpablocristo/pymes/core/backend/internal/pricelists"
	"github.com/devpablocristo/pymes/core/backend/internal/products"
	"github.com/devpablocristo/pymes/core/backend/internal/purchases"
	"github.com/devpablocristo/pymes/core/backend/internal/quotes"
	"github.com/devpablocristo/pymes/core/backend/internal/recurring"
	"github.com/devpablocristo/pymes/core/backend/internal/returns"
	pymesservices "github.com/devpablocristo/pymes/core/backend/internal/services"
	"github.com/devpablocristo/pymes/core/backend/internal/suppliers"
	"gorm.io/gorm"
)

// LifecycleEntry is the per-resource registration consumed by
// buildPymesLifecycleService.
//
//   - Policy: the lifecycle.ArchivePolicy for this resource.
//   - Config: the table layout (table, id col, tenant col, archived col).
//             Used to construct a generic lifecycle.SoftDeleter when Repo
//             is not provided.
//   - Repo:   an explicit lifecycle.RepositoryPort. When set, it overrides
//             Config — useful for modules whose SoftDelete/Restore/HardDelete
//             span multiple tables or contain custom transactional logic
//             (e.g. customers + suppliers, which share the `parties` table
//             and update party_roles, party_persons, accounts in one tx).
//
// Exactly one of {Repo set, Config populated} is expected. If both are
// provided, Repo wins.
type LifecycleEntry struct {
	Policy *lifecycle.ArchivePolicy
	Config lifecycle.SoftDeleterConfig
	Repo   lifecycle.RepositoryPort
}

// pymesAuditPort implements lifecycle.AuditPort by translating ArchiveAudit
// entries into pymes audit.LogInput and delegating to audit.Usecases.
type pymesAuditPort struct {
	uc *audit.Usecases
}

// pymesArchiveActionFor maps the agnostic lifecycle.Action constants to the
// pymes audit naming convention "<resourceType>.<verb>" (e.g. price_list.archived,
// employee.restored). This preserves consistency with existing audit entries
// produced by modules that called audit.Log directly before the refactor.
func pymesArchiveActionFor(resourceType string, action lifecycle.Action) string {
	verb := string(action)
	switch action {
	case lifecycle.ActionArchive:
		verb = "archived"
	case lifecycle.ActionRestore:
		verb = "restored"
	case lifecycle.ActionHardDelete:
		verb = "hard_deleted"
	}
	return resourceType + "." + verb
}

func (a *pymesAuditPort) Append(ctx context.Context, e lifecycle.ArchiveAudit) error {
	if a == nil || a.uc == nil {
		return nil
	}
	payload := map[string]any{}
	if e.Reason != nil {
		payload["reason"] = *e.Reason
	}
	if e.BatchID != nil {
		payload["batch_id"] = e.BatchID.String()
	}
	if e.RetentionExpires != nil {
		payload["retention_expires"] = e.RetentionExpires.UTC().Format("2006-01-02T15:04:05Z")
	}
	a.uc.LogWithActor(ctx, auditdomain.LogInput{
		OrgID: e.TenantID,
		Actor: auditdomain.ActorRef{
			Raw:   e.Actor,
			Type:  "user",
			Label: e.Actor,
		},
		Action:       pymesArchiveActionFor(e.ResourceType, e.Action),
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID.String(),
		Payload:      payload,
	})
	return nil
}

var _ lifecycle.AuditPort = (*pymesAuditPort)(nil)

// noopAuditPort is the fallback AuditPort used only when auditUC is nil.
type noopAuditPort struct{}

func (a *noopAuditPort) Append(_ context.Context, _ lifecycle.ArchiveAudit) error { return nil }

// buildPymesLifecycleService wires a single shared lifecycle.Service that
// handles archive / restore / hard-delete for every resource type
// registered in `entries`. Returns nil (legacy path everywhere) on any
// construction error so the system keeps booting.
func buildPymesLifecycleService(
	gdb *gorm.DB,
	auditUC *audit.Usecases,
	entries map[string]LifecycleEntry,
) *lifecycle.Service {
	if gdb == nil || len(entries) == 0 {
		return nil
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		log.Printf("warn: pymes lifecycle wire skipped (gdb.DB(): %v)", err)
		return nil
	}

	repos := make(map[string]lifecycle.RepositoryPort, len(entries))
	policies := make([]*lifecycle.ArchivePolicy, 0, len(entries))
	for rt, entry := range entries {
		if entry.Repo != nil {
			repos[rt] = entry.Repo
		} else {
			sd, sdErr := lifecycle.NewSoftDeleter(sqlDB, entry.Config)
			if sdErr != nil {
				log.Printf("warn: pymes lifecycle wire skipped for %q (NewSoftDeleter: %v)", rt, sdErr)
				return nil
			}
			repos[rt] = sd
		}
		policies = append(policies, entry.Policy)
	}

	var auditPort lifecycle.AuditPort
	if auditUC != nil {
		auditPort = &pymesAuditPort{uc: auditUC}
	} else {
		log.Println("warn: pymes lifecycle wired without audit (auditUC=nil); audit trail disabled")
		auditPort = &noopAuditPort{}
	}

	registry := lifecycle.NewStaticPolicyRegistry(policies...)
	svc, err := lifecycle.NewServiceWithRepos(repos, auditPort, registry)
	if err != nil {
		log.Printf("warn: pymes lifecycle wire skipped (NewServiceWithRepos: %v)", err)
		return nil
	}
	return svc
}

// pymesLifecycleRegistrations is the single source of truth listing every
// CRUDAR module that has opted into platform/lifecycle/go.Service.
//
// Add an entry here when a module is refactored. The wire site in
// bootstrap.go then uses this map to inject the same Service into every
// module's Usecases.
func pymesLifecycleRegistrations() map[string]LifecycleEntry {
	return map[string]LifecycleEntry{
		pricelists.ResourceTypePriceList: {
			Policy: pricelists.Policy,
			Config: lifecycle.SoftDeleterConfig{
				Table: "price_lists", IDColumn: "id",
				TenantColumn: "org_id", ArchivedAtColumn: "deleted_at",
			},
		},
		employees.ResourceTypeEmployee: {
			Policy: employees.Policy,
			Config: lifecycle.SoftDeleterConfig{
				Table: "employees", IDColumn: "id",
				TenantColumn: "org_id", ArchivedAtColumn: "deleted_at",
			},
		},
		recurring.ResourceTypeRecurringExpense: {
			Policy: recurring.Policy,
			Config: lifecycle.SoftDeleterConfig{
				Table: "recurring_expenses", IDColumn: "id",
				TenantColumn: "org_id", ArchivedAtColumn: "deleted_at",
			},
		},
		cashflow.ResourceTypeCashMovement: {
			Policy: cashflow.Policy,
			Config: lifecycle.SoftDeleterConfig{
				Table: "cash_movements", IDColumn: "id",
				TenantColumn: "org_id", ArchivedAtColumn: "deleted_at",
			},
		},
		payments.ResourceTypePayment: {
			Policy: payments.Policy,
			Config: lifecycle.SoftDeleterConfig{
				Table: "payments", IDColumn: "id",
				TenantColumn: "org_id", ArchivedAtColumn: "deleted_at",
			},
		},
		returns.ResourceTypeReturn: {
			Policy: returns.Policy,
			Config: lifecycle.SoftDeleterConfig{
				Table: "returns", IDColumn: "id",
				TenantColumn: "org_id", ArchivedAtColumn: "deleted_at",
			},
		},
		invoices.ResourceTypeInvoice: {
			Policy: invoices.Policy,
			Config: lifecycle.SoftDeleterConfig{
				Table: "invoices", IDColumn: "id",
				TenantColumn: "org_id", ArchivedAtColumn: "deleted_at",
			},
		},
		quotes.ResourceTypeQuote: {
			Policy: quotes.Policy,
			Config: lifecycle.SoftDeleterConfig{
				Table: "quotes", IDColumn: "id",
				TenantColumn: "org_id", ArchivedAtColumn: "deleted_at",
			},
		},
		products.ResourceTypeProduct: {
			Policy: products.Policy,
			Config: lifecycle.SoftDeleterConfig{
				Table: "products", IDColumn: "id",
				TenantColumn: "org_id", ArchivedAtColumn: "deleted_at",
			},
		},
		pymesservices.ResourceTypeService: {
			Policy: pymesservices.Policy,
			Config: lifecycle.SoftDeleterConfig{
				Table: "services", IDColumn: "id",
				TenantColumn: "org_id", ArchivedAtColumn: "deleted_at",
			},
		},
		purchases.ResourceTypePurchase: {
			Policy: purchases.Policy,
			Config: lifecycle.SoftDeleterConfig{
				Table: "purchases", IDColumn: "id",
				TenantColumn: "org_id", ArchivedAtColumn: "deleted_at",
			},
		},
	}
}

// pymesLifecyclePartiesRegistrations declares the customers + suppliers
// entries that need custom RepositoryPort adapters (multi-table transactions
// on the parties table). They are kept separate from the table-driven map
// in pymesLifecycleRegistrations so the type signatures stay clean.
func pymesLifecyclePartiesRegistrations(
	gdb *gorm.DB,
	customersRepo *customers.Repository,
	suppliersRepo *suppliers.Repository,
) map[string]LifecycleEntry {
	if gdb == nil || customersRepo == nil || suppliersRepo == nil {
		return nil
	}
	return map[string]LifecycleEntry{
		customers.ResourceTypeCustomer: {
			Policy: customers.Policy,
			Repo:   &customersLifecycleRepo{repo: customersRepo, db: gdb},
		},
		suppliers.ResourceTypeSupplier: {
			Policy: suppliers.Policy,
			Repo:   &suppliersLifecycleRepo{repo: suppliersRepo, db: gdb},
		},
	}
}

