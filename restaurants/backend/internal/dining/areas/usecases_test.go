package areas

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/areas/usecases/domain"
)

// --- fakes ---

type fakeRepo struct {
	areas   map[uuid.UUID]domain.DiningArea
	createFn func(ctx context.Context, in domain.DiningArea) (domain.DiningArea, error)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{areas: make(map[uuid.UUID]domain.DiningArea)}
}

func (r *fakeRepo) List(_ context.Context, _ ListParams) ([]domain.DiningArea, int64, bool, *uuid.UUID, error) {
	out := make([]domain.DiningArea, 0, len(r.areas))
	for _, a := range r.areas {
		out = append(out, a)
	}
	return out, int64(len(out)), false, nil, nil
}

func (r *fakeRepo) Create(_ context.Context, in domain.DiningArea) (domain.DiningArea, error) {
	if r.createFn != nil {
		return r.createFn(nil, in)
	}
	in.ID = uuid.New()
	r.areas[in.ID] = in
	return in, nil
}

func (r *fakeRepo) GetByID(_ context.Context, orgID, id uuid.UUID) (domain.DiningArea, error) {
	a, ok := r.areas[id]
	if !ok || a.OrgID != orgID {
		return domain.DiningArea{}, gorm.ErrRecordNotFound
	}
	return a, nil
}

func (r *fakeRepo) Update(_ context.Context, in domain.DiningArea) (domain.DiningArea, error) {
	r.areas[in.ID] = in
	return in, nil
}

type fakeAudit struct {
	calls int
}

func (a *fakeAudit) Log(_ context.Context, _, _, _, _, _ string, _ map[string]any) {
	a.calls++
}

// --- tests ---

func TestCreateHappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	audit := &fakeAudit{}
	uc := NewUsecases(repo, audit)

	orgID := uuid.New()
	out, err := uc.Create(context.Background(), domain.DiningArea{
		OrgID: orgID,
		Name:  "Terraza",
	}, "user-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Name != "Terraza" {
		t.Errorf("expected name Terraza, got %s", out.Name)
	}
	if out.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if audit.calls != 1 {
		t.Errorf("expected 1 audit call, got %d", audit.calls)
	}
}

func TestCreateNameTooShort(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), nil)

	_, err := uc.Create(context.Background(), domain.DiningArea{
		OrgID: uuid.New(),
		Name:  "A",
	}, "user-1")

	if err == nil {
		t.Fatal("expected error for short name")
	}
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Errorf("expected ErrBadInput, got %v", err)
	}
}

func TestCreateTrimsWhitespace(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	out, err := uc.Create(context.Background(), domain.DiningArea{
		OrgID: uuid.New(),
		Name:  "  Terraza  ",
	}, "user-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Name != "Terraza" {
		t.Errorf("expected trimmed name 'Terraza', got %q", out.Name)
	}
}

func TestGetByIDHappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	orgID := uuid.New()
	id := uuid.New()
	repo.areas[id] = domain.DiningArea{ID: id, OrgID: orgID, Name: "Barra"}

	out, err := uc.GetByID(context.Background(), orgID, id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Name != "Barra" {
		t.Errorf("expected name Barra, got %s", out.Name)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), nil)

	_, err := uc.GetByID(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateHappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	audit := &fakeAudit{}
	uc := NewUsecases(repo, audit)

	orgID := uuid.New()
	id := uuid.New()
	repo.areas[id] = domain.DiningArea{ID: id, OrgID: orgID, Name: "Barra", SortOrder: 1}

	newName := "Terraza"
	newSort := 5
	out, err := uc.Update(context.Background(), orgID, id, UpdateInput{
		Name:      &newName,
		SortOrder: &newSort,
	}, "user-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Name != "Terraza" {
		t.Errorf("expected name Terraza, got %s", out.Name)
	}
	if out.SortOrder != 5 {
		t.Errorf("expected sort order 5, got %d", out.SortOrder)
	}
	if audit.calls != 1 {
		t.Errorf("expected 1 audit call, got %d", audit.calls)
	}
}

func TestUpdateNotFound(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), nil)

	name := "Foo"
	_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), UpdateInput{Name: &name}, "user-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateNameTooShort(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	orgID := uuid.New()
	id := uuid.New()
	repo.areas[id] = domain.DiningArea{ID: id, OrgID: orgID, Name: "Barra"}

	short := "X"
	_, err := uc.Update(context.Background(), orgID, id, UpdateInput{Name: &short}, "user-1")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Errorf("expected ErrBadInput, got %v", err)
	}
}

func TestListReturnsItems(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	orgID := uuid.New()
	repo.areas[uuid.New()] = domain.DiningArea{OrgID: orgID, Name: "A"}
	repo.areas[uuid.New()] = domain.DiningArea{OrgID: orgID, Name: "B"}

	items, total, _, _, err := uc.List(context.Background(), ListParams{OrgID: orgID})
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
