package purchases

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/errors/go/domainerr"
	purchasesdomain "github.com/devpablocristo/pymes/core/backend/internal/purchases/usecases/domain"
)

// TestPurchase_UpdateStatus_FSM cubre los casos canónicos para purchases.
// Decisión arquitectónica: voided NO es terminal (preservamos comportamiento
// histórico). Las transiciones entre los 4 estados son libres.
func TestPurchase_UpdateStatus_FSM(t *testing.T) {
	t.Parallel()

	const actor = "test-actor"
	archivedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name           string
		current        string
		next           string
		archived       bool
		wantOK         bool
		wantConflict   bool
		wantBadInput   bool
		wantRepoCalled bool
		wantAuditCalls int
	}{
		// Free transitions entre los 4 estados (válidas siempre).
		{name: "draft -> partial", current: "draft", next: "partial", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "partial -> received", current: "partial", next: "received", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "received -> voided", current: "received", next: "voided", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "voided -> draft (revivir)", current: "voided", next: "draft", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "voided -> received", current: "voided", next: "received", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},

		// Same-status idempotente.
		{name: "same: draft -> draft", current: "draft", next: "draft", wantOK: true},
		{name: "same: voided -> voided", current: "voided", next: "voided", wantOK: true},

		// Bad input.
		{name: "empty status", current: "draft", next: "", wantBadInput: true},
		{name: "whitespace status", current: "draft", next: "   ", wantBadInput: true},

		// Status desconocido (no en grafo).
		{name: "unknown next", current: "draft", next: "weird", wantConflict: true},

		// Archived.
		{name: "archived purchase rejected", current: "draft", next: "received", archived: true, wantConflict: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			orgID := uuid.New()
			purchaseID := uuid.New()
			var repoCalled int

			repo := &mockPurchasesRepo{
				getByIDFn: func(ctx context.Context, _ uuid.UUID, id uuid.UUID) (purchasesdomain.Purchase, error) {
					p := purchasesdomain.Purchase{ID: id, OrgID: orgID, Status: tc.current, Number: "PO-00001"}
					if tc.archived {
						p.DeletedAt = &archivedAt
					}
					return p, nil
				},
				updateStatusFn: func(ctx context.Context, in UpdateStatusInput) (purchasesdomain.Purchase, error) {
					repoCalled++
					return purchasesdomain.Purchase{ID: in.ID, OrgID: in.OrgID, Status: in.Status, Number: "PO-00001"}, nil
				},
			}
			audit := &mockPurchasesAudit{}
			uc := NewUsecases(repo, audit)

			out, err := uc.UpdateStatus(context.Background(), UpdateStatusInput{
				OrgID:  orgID,
				ID:     purchaseID,
				Status: tc.next,
			}, actor)

			switch {
			case tc.wantOK:
				if err != nil {
					t.Fatalf("expected ok, got %v", err)
				}
				if tc.wantRepoCalled && repoCalled != 1 {
					t.Fatalf("expected repo.UpdateStatus called once, got %d", repoCalled)
				}
				if !tc.wantRepoCalled && repoCalled != 0 {
					t.Fatalf("expected repo.UpdateStatus NOT called (idempotent), got %d", repoCalled)
				}
				if audit.calls != tc.wantAuditCalls {
					t.Fatalf("expected audit calls=%d, got %d", tc.wantAuditCalls, audit.calls)
				}
				if tc.wantOK && out.ID == uuid.Nil {
					t.Fatal("expected non-zero purchase ID in output")
				}
			case tc.wantConflict:
				if err == nil {
					t.Fatal("expected conflict error, got nil")
				}
				if !domainerr.IsConflict(err) {
					t.Fatalf("expected domainerr.Conflict, got %v", err)
				}
				if repoCalled != 0 {
					t.Fatal("repo.UpdateStatus must NOT be called on conflict")
				}
				if audit.calls != 0 {
					t.Fatal("audit must NOT be emitted on conflict")
				}
			case tc.wantBadInput:
				if err == nil {
					t.Fatal("expected bad input, got nil")
				}
				if !domainerr.IsValidation(err) {
					t.Fatalf("expected domainerr.Validation, got %v", err)
				}
				if repoCalled != 0 {
					t.Fatal("repo.UpdateStatus must NOT be called on bad input")
				}
			}
		})
	}
}
