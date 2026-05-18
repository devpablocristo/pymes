package invoices

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/errors/go/domainerr"
	invdomain "github.com/devpablocristo/pymes/core/backend/internal/invoices/usecases/domain"
)

type mockInvoiceRepo struct {
	getByIDFn        func(ctx context.Context, orgID, id uuid.UUID) (invdomain.Invoice, error)
	updateStatusFn   func(ctx context.Context, orgID, id uuid.UUID, status string) (invdomain.Invoice, error)
	updateStatusCnt  int
}

func (m *mockInvoiceRepo) List(context.Context, ListParams) ([]invdomain.Invoice, int64, bool, *uuid.UUID, error) {
	return nil, 0, false, nil, nil
}
func (m *mockInvoiceRepo) ListArchived(context.Context, uuid.UUID, int) ([]invdomain.Invoice, error) {
	return nil, nil
}
func (m *mockInvoiceRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (invdomain.Invoice, error) {
	return m.getByIDFn(ctx, orgID, id)
}
func (m *mockInvoiceRepo) Create(context.Context, invdomain.Invoice) (invdomain.Invoice, error) {
	return invdomain.Invoice{}, nil
}
func (m *mockInvoiceRepo) Update(context.Context, invdomain.Invoice) (invdomain.Invoice, error) {
	return invdomain.Invoice{}, nil
}
func (m *mockInvoiceRepo) UpdateStatus(ctx context.Context, orgID, id uuid.UUID, status string) (invdomain.Invoice, error) {
	m.updateStatusCnt++
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, orgID, id, status)
	}
	return invdomain.Invoice{ID: id, OrgID: orgID, Status: invdomain.InvoiceStatus(status)}, nil
}
func (m *mockInvoiceRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error  { return nil }
func (m *mockInvoiceRepo) Restore(context.Context, uuid.UUID, uuid.UUID) error    { return nil }
func (m *mockInvoiceRepo) HardDelete(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type mockInvoiceAudit struct{ calls int }

func (m *mockInvoiceAudit) Log(context.Context, string, string, string, string, string, map[string]any) {
	m.calls++
}

// TestInvoice_UpdateStatus_FSM cubre los casos canónicos para invoices:
// pending↔overdue, ambos→paid (terminal), same-status idempotente, archived,
// status vacío y desconocido.
func TestInvoice_UpdateStatus_FSM(t *testing.T) {
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
		// Válidos
		{name: "pending -> paid", current: "pending", next: "paid", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "pending -> overdue", current: "pending", next: "overdue", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "overdue -> paid", current: "overdue", next: "paid", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},

		// Inválidos
		{name: "paid -> pending (terminal)", current: "paid", next: "pending", wantConflict: true},
		{name: "paid -> overdue (terminal)", current: "paid", next: "overdue", wantConflict: true},
		{name: "overdue -> pending (no rollback)", current: "overdue", next: "pending", wantConflict: true},
		{name: "unknown status", current: "pending", next: "weird", wantConflict: true},

		// Bad input
		{name: "empty", current: "pending", next: "", wantBadInput: true},
		{name: "whitespace", current: "pending", next: "   ", wantBadInput: true},

		// Same-status idempotente
		{name: "same: pending -> pending", current: "pending", next: "pending", wantOK: true},
		{name: "same: paid -> paid (terminal idempotent)", current: "paid", next: "paid", wantOK: true},

		// Archived
		{name: "archived rejected", current: "pending", next: "paid", archived: true, wantConflict: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			orgID := uuid.New()
			invID := uuid.New()

			repo := &mockInvoiceRepo{
				getByIDFn: func(ctx context.Context, _ uuid.UUID, id uuid.UUID) (invdomain.Invoice, error) {
					inv := invdomain.Invoice{ID: id, OrgID: orgID, Status: invdomain.InvoiceStatus(tc.current)}
					if tc.archived {
						inv.ArchivedAt = &archivedAt
					}
					return inv, nil
				},
			}
			audit := &mockInvoiceAudit{}
			uc := &Usecases{repo: repo, audit: audit}

			out, err := uc.UpdateStatus(context.Background(), UpdateStatusInput{
				OrgID:  orgID,
				ID:     invID,
				Status: tc.next,
			}, actor)

			switch {
			case tc.wantOK:
				if err != nil {
					t.Fatalf("expected ok, got %v", err)
				}
				if tc.wantRepoCalled && repo.updateStatusCnt != 1 {
					t.Fatalf("expected repo.UpdateStatus called once, got %d", repo.updateStatusCnt)
				}
				if !tc.wantRepoCalled && repo.updateStatusCnt != 0 {
					t.Fatalf("expected repo.UpdateStatus NOT called (idempotent), got %d", repo.updateStatusCnt)
				}
				if audit.calls != tc.wantAuditCalls {
					t.Fatalf("expected audit calls=%d, got %d", tc.wantAuditCalls, audit.calls)
				}
				if tc.wantOK && out.ID == uuid.Nil {
					t.Fatal("expected non-zero invoice ID in output")
				}
			case tc.wantConflict:
				if err == nil {
					t.Fatal("expected conflict, got nil")
				}
				if !domainerr.IsConflict(err) {
					t.Fatalf("expected domainerr.Conflict, got %v", err)
				}
				if repo.updateStatusCnt != 0 {
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
				if repo.updateStatusCnt != 0 {
					t.Fatal("repo.UpdateStatus must NOT be called on bad input")
				}
			}
		})
	}
}
