package pricelists_test

import (
	"context"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/core/backend/internal/pricelists"
	lifecycle "github.com/devpablocristo/platform/lifecycle/go/lifecycle"
	"github.com/google/uuid"
)

// fakeRepo is an in-memory RepositoryPort for testing lifecycle.Service
// against pricelists without touching the DB. Mirrors what the production
// SoftDeleter would do.
type fakeRepo struct {
	rows map[uuid.UUID]*time.Time // archivedAt; nil = active
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{rows: make(map[uuid.UUID]*time.Time)}
}

func (r *fakeRepo) seed(id uuid.UUID) { r.rows[id] = nil }

func (r *fakeRepo) SoftDelete(_ context.Context, _, id uuid.UUID, at time.Time) error {
	if v, ok := r.rows[id]; !ok || v != nil {
		return lifecycleNotFound(id)
	}
	t := at
	r.rows[id] = &t
	return nil
}
func (r *fakeRepo) Restore(_ context.Context, _, id uuid.UUID) error {
	if v, ok := r.rows[id]; !ok || v == nil {
		return lifecycleNotFound(id)
	}
	r.rows[id] = nil
	return nil
}
func (r *fakeRepo) HardDelete(_ context.Context, _, id uuid.UUID) error {
	if _, ok := r.rows[id]; !ok {
		return lifecycleNotFound(id)
	}
	delete(r.rows, id)
	return nil
}
func (r *fakeRepo) IsArchived(_ context.Context, _, id uuid.UUID) (bool, error) {
	v, ok := r.rows[id]
	if !ok {
		return false, lifecycleNotFound(id)
	}
	return v != nil, nil
}

func lifecycleNotFound(_ uuid.UUID) error {
	return &lifecycleErr{msg: "not found"}
}

type lifecycleErr struct{ msg string }

func (e *lifecycleErr) Error() string { return e.msg }

// recordingAudit captures lifecycle.ArchiveAudit entries for assertions.
type recordingAudit struct{ entries []lifecycle.ArchiveAudit }

func (a *recordingAudit) Append(_ context.Context, e lifecycle.ArchiveAudit) error {
	a.entries = append(a.entries, e)
	return nil
}

// TestPolicy_LifecycleService demonstrates that the pricelists policy wires
// correctly into lifecycle.Service. This is the piloto for Ola C step 2 —
// it does not replace the existing Usecases yet; it proves the contract.
func TestPolicy_LifecycleService(t *testing.T) {
	repo := newFakeRepo()
	audit := &recordingAudit{}
	registry := lifecycle.NewStaticPolicyRegistry(pricelists.Policy)

	svc, err := lifecycle.NewServiceWithRepos(
		map[string]lifecycle.RepositoryPort{
			pricelists.ResourceTypePriceList: repo,
		},
		audit,
		registry,
	)
	if err != nil {
		t.Fatalf("NewServiceWithRepos: %v", err)
	}

	tenantID := uuid.New()
	id := uuid.New()
	repo.seed(id)

	// 1. Archive
	if err := svc.SoftDelete(context.Background(), &lifecycle.ArchiveRequest{
		ResourceType: pricelists.ResourceTypePriceList,
		ResourceID:   id,
		TenantID:     tenantID,
		Actor:        "admin@example.com",
	}); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if repo.rows[id] == nil {
		t.Fatal("row should be archived")
	}

	// 2. Restore
	if err := svc.Restore(context.Background(), &lifecycle.RestoreRequest{
		ResourceType: pricelists.ResourceTypePriceList,
		ResourceID:   id,
		TenantID:     tenantID,
		Actor:        "admin@example.com",
	}); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if repo.rows[id] != nil {
		t.Fatal("row should be restored")
	}

	// 3. Archive again, then HardDelete (must-be-archived = true)
	_ = svc.SoftDelete(context.Background(), &lifecycle.ArchiveRequest{
		ResourceType: pricelists.ResourceTypePriceList,
		ResourceID:   id,
		TenantID:     tenantID,
		Actor:        "admin@example.com",
	})
	if err := svc.HardDelete(context.Background(), &lifecycle.HardDeleteRequest{
		ResourceType:   pricelists.ResourceTypePriceList,
		ResourceID:     id,
		TenantID:       tenantID,
		Actor:          "admin@example.com",
		MustBeArchived: true,
	}); err != nil {
		t.Fatalf("HardDelete: %v", err)
	}
	if _, ok := repo.rows[id]; ok {
		t.Fatal("row should be hard-deleted")
	}

	// 4. Verify all three actions made an audit entry.
	wantActions := []lifecycle.Action{
		lifecycle.ActionArchive,
		lifecycle.ActionRestore,
		lifecycle.ActionArchive,
		lifecycle.ActionHardDelete,
	}
	if len(audit.entries) != len(wantActions) {
		t.Fatalf("expected %d audit entries, got %d", len(wantActions), len(audit.entries))
	}
	for i, want := range wantActions {
		if audit.entries[i].Action != want {
			t.Errorf("entry %d: action=%q want %q", i, audit.entries[i].Action, want)
		}
		if audit.entries[i].ResourceType != pricelists.ResourceTypePriceList {
			t.Errorf("entry %d: resource_type=%q want %q", i, audit.entries[i].ResourceType, pricelists.ResourceTypePriceList)
		}
	}
}
