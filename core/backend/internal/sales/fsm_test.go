package sales

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/devpablocristo/platform/errors/go/domainerr"
	saledomain "github.com/devpablocristo/pymes/core/backend/internal/sales/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/core/shared/backend/httperrors"
)

// TestSale_UpdateStatus_FSM cubre los 9 casos canónicos definidos en el plan
// arquitectónico (Sección 9.2): válido, inválido, terminal a destino, terminal
// origen, status vacío, desconocido, same-status no terminal, same-status
// terminal, side effects no se disparan si el FSM rechaza.
func TestSale_UpdateStatus_FSM(t *testing.T) {
	t.Parallel()

	const (
		actor = "test-actor"
	)

	cases := []struct {
		name           string
		current        string
		next           string
		wantOK         bool
		wantConflict   bool
		wantBadInput   bool
		wantRepoCalled bool
		wantAuditCalls int
	}{
		{name: "valid: draft -> pending", current: "draft", next: "pending", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "valid: draft -> completed", current: "draft", next: "completed", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "valid: pending -> completed", current: "pending", next: "completed", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "valid: completed -> paid", current: "completed", next: "paid", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "valid: any non-terminal -> voided", current: "draft", next: "voided", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},

		{name: "invalid: draft -> paid", current: "draft", next: "paid", wantConflict: true},
		{name: "invalid: completed -> draft", current: "completed", next: "draft", wantConflict: true},
		{name: "terminal origin: voided -> draft", current: "voided", next: "draft", wantConflict: true},
		{name: "terminal origin: paid -> voided (must use Void())", current: "paid", next: "voided", wantConflict: true},
		{name: "unknown next status", current: "draft", next: "weird", wantConflict: true},

		{name: "empty status -> bad input", current: "draft", next: "", wantBadInput: true},
		{name: "whitespace-only status -> bad input", current: "draft", next: "   ", wantBadInput: true},

		// Same-status idempotente: no toca DB, no emite audit, devuelve current.
		{name: "same-status non-terminal: draft -> draft", current: "draft", next: "draft", wantOK: true},
		{name: "same-status terminal: paid -> paid", current: "paid", next: "paid", wantOK: true},
		{name: "same-status terminal: voided -> voided", current: "voided", next: "voided", wantOK: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			orgID := uuid.New()
			saleID := uuid.New()

			repo := &mockRepo{
				getByIDFn: func(ctx context.Context, _ uuid.UUID, id uuid.UUID) (saledomain.Sale, error) {
					return saledomain.Sale{ID: id, OrgID: orgID, Status: tc.current}, nil
				},
			}
			audit := &mockAudit{}
			uc := NewUsecases(repo, nil, nil, audit)

			out, err := uc.UpdateStatus(context.Background(), UpdateStatusInput{
				OrgID:  orgID,
				ID:     saleID,
				Status: tc.next,
			}, actor)

			switch {
			case tc.wantOK:
				if err != nil {
					t.Fatalf("expected ok, got %v", err)
				}
				if tc.wantRepoCalled && repo.updateStatusCalled != 1 {
					t.Fatalf("expected repo.UpdateStatus called once, got %d", repo.updateStatusCalled)
				}
				if !tc.wantRepoCalled && repo.updateStatusCalled != 0 {
					t.Fatalf("expected repo.UpdateStatus NOT called (idempotent same-status), got %d", repo.updateStatusCalled)
				}
				if audit.calls != tc.wantAuditCalls {
					t.Fatalf("expected audit calls=%d, got %d", tc.wantAuditCalls, audit.calls)
				}
				if tc.wantOK && out.ID == uuid.Nil {
					t.Fatal("expected non-zero sale ID in output")
				}
			case tc.wantConflict:
				if err == nil {
					t.Fatal("expected conflict error, got nil")
				}
				if !domainerr.IsConflict(err) {
					t.Fatalf("expected domainerr.Conflict, got %v", err)
				}
				if repo.updateStatusCalled != 0 {
					t.Fatal("repo.UpdateStatus must NOT be called on conflict")
				}
				if audit.calls != 0 {
					t.Fatal("audit must NOT be emitted on conflict")
				}
			case tc.wantBadInput:
				if err == nil {
					t.Fatal("expected bad input error, got nil")
				}
				if !errors.Is(err, httperrors.ErrBadInput) {
					t.Fatalf("expected ErrBadInput, got %v", err)
				}
				if repo.updateStatusCalled != 0 {
					t.Fatal("repo.UpdateStatus must NOT be called on bad input")
				}
				if audit.calls != 0 {
					t.Fatal("audit must NOT be emitted on bad input")
				}
			}
		})
	}
}
