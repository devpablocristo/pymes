package sessions

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/sessions/usecases/domain"
)

// --- fakes ---

type fakeRepo struct {
	sessions    map[uuid.UUID]domain.TableSession
	openErr     error
	closeErr    error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{sessions: make(map[uuid.UUID]domain.TableSession)}
}

func (r *fakeRepo) List(_ context.Context, _ ListParams) ([]domain.TableSessionListItem, int64, error) {
	out := make([]domain.TableSessionListItem, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, domain.TableSessionListItem{TableSession: s})
	}
	return out, int64(len(out)), nil
}

func (r *fakeRepo) OpenSession(_ context.Context, orgID, tableID uuid.UUID, guestCount int, partyLabel, notes string) (domain.TableSession, error) {
	if r.openErr != nil {
		return domain.TableSession{}, r.openErr
	}
	s := domain.TableSession{
		ID:         uuid.New(),
		OrgID:      orgID,
		TableID:    tableID,
		GuestCount: guestCount,
		PartyLabel: partyLabel,
		Notes:      notes,
		OpenedAt:   time.Now(),
	}
	r.sessions[s.ID] = s
	return s, nil
}

func (r *fakeRepo) CloseSession(_ context.Context, orgID, sessionID uuid.UUID) (domain.TableSession, error) {
	if r.closeErr != nil {
		return domain.TableSession{}, r.closeErr
	}
	s, ok := r.sessions[sessionID]
	if !ok || s.OrgID != orgID {
		return domain.TableSession{}, fmt.Errorf("session not found: %w", httperrors.ErrNotFound)
	}
	now := time.Now()
	s.ClosedAt = &now
	r.sessions[sessionID] = s
	return s, nil
}

type fakeAudit struct {
	calls int
}

func (a *fakeAudit) Log(_ context.Context, _, _, _, _, _ string, _ map[string]any) {
	a.calls++
}

// --- tests ---

func TestOpenHappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	audit := &fakeAudit{}
	uc := NewUsecases(repo, audit)

	orgID := uuid.New()
	tableID := uuid.New()
	out, err := uc.Open(context.Background(), orgID, tableID, 4, "Familia Garcia", "cumpleanos", "user-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if out.GuestCount != 4 {
		t.Errorf("expected guest count 4, got %d", out.GuestCount)
	}
	if out.PartyLabel != "Familia Garcia" {
		t.Errorf("expected party label 'Familia Garcia', got %q", out.PartyLabel)
	}
	if audit.calls != 1 {
		t.Errorf("expected 1 audit call, got %d", audit.calls)
	}
}

func TestOpenInvalidGuestCount(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), nil)

	cases := []struct {
		name  string
		count int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too large", 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := uc.Open(context.Background(), uuid.New(), uuid.New(), tc.count, "", "", "user-1")
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, httperrors.ErrBadInput) {
				t.Errorf("expected ErrBadInput for count %d, got %v", tc.count, err)
			}
		})
	}
}

func TestOpenTrimsWhitespace(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	out, err := uc.Open(context.Background(), uuid.New(), uuid.New(), 2, "  Mesa VIP  ", "  notas  ", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.PartyLabel != "Mesa VIP" {
		t.Errorf("expected trimmed party label, got %q", out.PartyLabel)
	}
	if out.Notes != "notas" {
		t.Errorf("expected trimmed notes, got %q", out.Notes)
	}
}

func TestOpenRepoError(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	repo.openErr = fmt.Errorf("table occupied: %w", httperrors.ErrConflict)
	uc := NewUsecases(repo, nil)

	_, err := uc.Open(context.Background(), uuid.New(), uuid.New(), 2, "", "", "user-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, httperrors.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestCloseHappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	audit := &fakeAudit{}
	uc := NewUsecases(repo, audit)

	orgID := uuid.New()
	// abrir primero
	opened, err := uc.Open(context.Background(), orgID, uuid.New(), 3, "Test", "", "user-1")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	closed, err := uc.Close(context.Background(), orgID, opened.ID, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed.ClosedAt == nil {
		t.Error("expected ClosedAt to be set")
	}
	// 1 for open + 1 for close
	if audit.calls != 2 {
		t.Errorf("expected 2 audit calls, got %d", audit.calls)
	}
}

func TestCloseNotFound(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	_, err := uc.Close(context.Background(), uuid.New(), uuid.New(), "user-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListReturnsItems(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	orgID := uuid.New()
	// abrir dos sesiones
	_, _ = uc.Open(context.Background(), orgID, uuid.New(), 2, "A", "", "user-1")
	_, _ = uc.Open(context.Background(), orgID, uuid.New(), 3, "B", "", "user-1")

	items, total, err := uc.List(context.Background(), ListParams{OrgID: orgID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}
