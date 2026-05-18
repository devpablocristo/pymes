package quotes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/core/errors/go/domainerr"
	quotedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/quotes/usecases/domain"
	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
)

// TestQuote_UpdateStatus_FSM cubre los casos canónicos del plan: válidos,
// inválidos, terminales, vacío, desconocido, same-status idempotente y
// archivado. Verifica que side effects (audit, repo.SetStatus) NO se invocan
// cuando el FSM rechaza.
func TestQuote_UpdateStatus_FSM(t *testing.T) {
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
		{name: "valid: draft -> sent", current: "draft", next: "sent", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "valid: sent -> accepted", current: "sent", next: "accepted", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "valid: draft -> rejected", current: "draft", next: "rejected", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "valid: sent -> rejected", current: "sent", next: "rejected", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "valid: draft -> expired", current: "draft", next: "expired", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},
		{name: "valid: sent -> expired", current: "sent", next: "expired", wantOK: true, wantRepoCalled: true, wantAuditCalls: 1},

		// Inválidos
		{name: "invalid: draft -> accepted (skip sent)", current: "draft", next: "accepted", wantConflict: true},
		{name: "invalid: sent -> draft", current: "sent", next: "draft", wantConflict: true},
		{name: "terminal: accepted -> rejected", current: "accepted", next: "rejected", wantConflict: true},
		{name: "terminal: rejected -> sent", current: "rejected", next: "sent", wantConflict: true},
		{name: "terminal: expired -> sent", current: "expired", next: "sent", wantConflict: true},
		{name: "unknown: draft -> weird", current: "draft", next: "weird", wantConflict: true},

		// Bad input
		{name: "empty status", current: "draft", next: "", wantBadInput: true},
		{name: "whitespace status", current: "draft", next: "   ", wantBadInput: true},

		// Same-status (idempotente)
		{name: "same: draft -> draft", current: "draft", next: "draft", wantOK: true},
		{name: "same: accepted -> accepted (terminal idempotent)", current: "accepted", next: "accepted", wantOK: true},

		// Archived
		{name: "archived quote -> conflict", current: "draft", next: "sent", archived: true, wantConflict: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			orgID := uuid.New()
			quoteID := uuid.New()

			repo := &mockQuoteRepo{
				getByIDFn: func(ctx context.Context, _ uuid.UUID, id uuid.UUID) (quotedomain.Quote, error) {
					q := quotedomain.Quote{ID: id, OrgID: orgID, Status: tc.current}
					if tc.archived {
						q.ArchivedAt = &archivedAt
					}
					return q, nil
				},
			}
			audit := &mockQuoteAudit{}
			uc := NewUsecases(repo, nil, audit)

			out, err := uc.UpdateStatus(context.Background(), UpdateStatusInput{
				OrgID:  orgID,
				ID:     quoteID,
				Status: tc.next,
			}, actor)

			switch {
			case tc.wantOK:
				if err != nil {
					t.Fatalf("expected ok, got %v", err)
				}
				if tc.wantRepoCalled && repo.setStatusCalled != 1 {
					t.Fatalf("expected repo.SetStatus called once, got %d", repo.setStatusCalled)
				}
				if !tc.wantRepoCalled && repo.setStatusCalled != 0 {
					t.Fatalf("expected repo.SetStatus NOT called (idempotent same-status), got %d", repo.setStatusCalled)
				}
				if audit.calls != tc.wantAuditCalls {
					t.Fatalf("expected audit calls=%d, got %d", tc.wantAuditCalls, audit.calls)
				}
				if tc.wantOK && out.ID == uuid.Nil {
					t.Fatal("expected non-zero quote ID in output")
				}
			case tc.wantConflict:
				if err == nil {
					t.Fatal("expected conflict error, got nil")
				}
				if !domainerr.IsConflict(err) {
					t.Fatalf("expected domainerr.Conflict, got %v", err)
				}
				if repo.setStatusCalled != 0 {
					t.Fatal("repo.SetStatus must NOT be called on conflict")
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
				if repo.setStatusCalled != 0 {
					t.Fatal("repo.SetStatus must NOT be called on bad input")
				}
			}
		})
	}
}

// TestQuote_SendAcceptReject_DelegateToFSM verifica que los métodos
// Send/Accept/Reject pasan por el FSM (delegados a UpdateStatus). Si el grafo
// rechaza, devuelven Conflict.
func TestQuote_SendAcceptReject_DelegateToFSM(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		current string
		op      func(uc *Usecases, ctx context.Context, orgID, id uuid.UUID, actor string) (quotedomain.Quote, error)
		wantOK  bool
	}{
		{name: "Send from draft ok", current: "draft", op: (*Usecases).Send, wantOK: true},
		// Send from sent: same-status idempotente (no rechaza, no toca DB).
		{name: "Send from sent ok (idempotent)", current: "sent", op: (*Usecases).Send, wantOK: true},
		{name: "Send from accepted rejected (terminal)", current: "accepted", op: (*Usecases).Send, wantOK: false},
		{name: "Accept from sent ok", current: "sent", op: (*Usecases).Accept, wantOK: true},
		{name: "Accept from draft rejected (must go via sent)", current: "draft", op: (*Usecases).Accept, wantOK: false},
		{name: "Reject from draft ok", current: "draft", op: (*Usecases).Reject, wantOK: true},
		{name: "Reject from sent ok", current: "sent", op: (*Usecases).Reject, wantOK: true},
		{name: "Reject from accepted rejected (terminal)", current: "accepted", op: (*Usecases).Reject, wantOK: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			orgID := uuid.New()
			quoteID := uuid.New()
			repo := &mockQuoteRepo{
				getByIDFn: func(ctx context.Context, _ uuid.UUID, id uuid.UUID) (quotedomain.Quote, error) {
					return quotedomain.Quote{ID: id, OrgID: orgID, Status: tc.current}, nil
				},
			}
			audit := &mockQuoteAudit{}
			uc := NewUsecases(repo, nil, audit)

			_, err := tc.op(uc, context.Background(), orgID, quoteID, "test-actor")
			if tc.wantOK && err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if !tc.wantOK {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !domainerr.IsConflict(err) {
					t.Fatalf("expected domainerr.Conflict, got %v", err)
				}
			}
		})
	}
}
