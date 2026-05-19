// Ola C step 4 — wire helper for pricelists lifecycle.Service with real audit.
//
// Builds a platform/lifecycle/go.Service scoped to the "price_list" resource
// type and routes its audit through the pymes audit.Usecases (hash chain v2 +
// audit_log table).
//
// Other CRUDAR modules will add their own builders here (or a shared service
// grouping multiple resource types) as they opt in. See docs/CRUDAR_REFACTOR.md.
package wire

import (
	"context"
	"log"

	lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
	"github.com/devpablocristo/pymes/core/backend/internal/audit"
	auditdomain "github.com/devpablocristo/pymes/core/backend/internal/audit/usecases/domain"
	"github.com/devpablocristo/pymes/core/backend/internal/pricelists"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// pymesAuditPort implements lifecycle.AuditPort by translating each
// ArchiveAudit into a pymes audit.LogInput and forwarding to the existing
// audit.Usecases. This is what activates the audit trail for archive,
// restore and hard-delete actions emitted via lifecycle.Service.
//
// The mapping is direct:
//
//   lifecycle.ArchiveAudit       audit.LogInput
//   ---------------------------- ----------------------------
//   TenantID         (uuid.UUID) OrgID            (uuid.UUID)
//   ResourceType     (string)    ResourceType     (string)
//   ResourceID       (uuid.UUID) ResourceID       (string)
//   Action           (Action)    Action           (string)
//   Actor            (string)    Actor.Raw / Label
//   Reason           (*string)   Payload["reason"]
//   BatchID          (*uuid.UUID) Payload["batch_id"]
//   OccurredAt       (time.Time) (set internally by audit hash chain)
type pymesAuditPort struct {
	uc *audit.Usecases
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
		Action:       string(e.Action),
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID.String(),
		Payload:      payload,
	})
	return nil
}

// Ensure pymesAuditPort satisfies lifecycle.AuditPort at compile time.
var _ lifecycle.AuditPort = (*pymesAuditPort)(nil)

// buildPriceListsLifecycleService wires a Service that handles archive /
// restore / hard-delete for the "price_list" resource type. Returns nil
// (legacy path) on any construction error so the system keeps booting.
//
// auditUC must be the same pymes audit.Usecases used by the rest of the
// modules so all audit entries land in the canonical audit_log table.
// Passing nil leaves audit disabled (a one-line warn is emitted; the
// system keeps booting).
func buildPriceListsLifecycleService(gdb *gorm.DB, auditUC *audit.Usecases) *lifecycle.Service {
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

	var auditPort lifecycle.AuditPort
	if auditUC != nil {
		auditPort = &pymesAuditPort{uc: auditUC}
	} else {
		log.Println("warn: pricelists lifecycle wired without audit (auditUC=nil); audit trail disabled")
		auditPort = &noopAuditPort{}
	}

	registry := lifecycle.NewStaticPolicyRegistry(pricelists.Policy)
	svc, err := lifecycle.NewServiceWithRepos(
		map[string]lifecycle.RepositoryPort{
			pricelists.ResourceTypePriceList: sd,
		},
		auditPort,
		registry,
	)
	if err != nil {
		log.Printf("warn: pricelists lifecycle wire skipped (NewServiceWithRepos: %v)", err)
		return nil
	}
	return svc
}

// noopAuditPort is the fallback AuditPort used only when auditUC is not
// supplied. It discards entries.
type noopAuditPort struct{}

func (a *noopAuditPort) Append(_ context.Context, _ lifecycle.ArchiveAudit) error { return nil }

// (compile-time placeholder: keeps uuid import in case future versions of
// this file need to construct UUIDs directly. Free to remove if unused.)
var _ = uuid.Nil
