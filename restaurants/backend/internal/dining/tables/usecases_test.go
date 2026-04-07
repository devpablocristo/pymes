package tables

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/restaurants/backend/internal/dining/tables/usecases/domain"
)

// --- fakes ---

type fakeRepo struct {
	tables   map[uuid.UUID]domain.DiningTable
	createFn func(ctx context.Context, in domain.DiningTable) (domain.DiningTable, error)
	updateFn func(ctx context.Context, in domain.DiningTable) (domain.DiningTable, error)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{tables: make(map[uuid.UUID]domain.DiningTable)}
}

func (r *fakeRepo) List(_ context.Context, _ ListParams) ([]domain.DiningTable, int64, bool, *uuid.UUID, error) {
	out := make([]domain.DiningTable, 0, len(r.tables))
	for _, t := range r.tables {
		out = append(out, t)
	}
	return out, int64(len(out)), false, nil, nil
}

func (r *fakeRepo) Create(_ context.Context, in domain.DiningTable) (domain.DiningTable, error) {
	if r.createFn != nil {
		return r.createFn(nil, in)
	}
	in.ID = uuid.New()
	r.tables[in.ID] = in
	return in, nil
}

func (r *fakeRepo) GetByID(_ context.Context, orgID, id uuid.UUID) (domain.DiningTable, error) {
	t, ok := r.tables[id]
	if !ok || t.OrgID != orgID {
		return domain.DiningTable{}, gorm.ErrRecordNotFound
	}
	return t, nil
}

func (r *fakeRepo) Update(_ context.Context, in domain.DiningTable) (domain.DiningTable, error) {
	if r.updateFn != nil {
		return r.updateFn(nil, in)
	}
	r.tables[in.ID] = in
	return in, nil
}

type fakeAreaLookup struct {
	exists bool
	err    error
}

func (f *fakeAreaLookup) ExistsForOrg(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return f.exists, f.err
}

type fakeAudit struct {
	calls int
}

func (a *fakeAudit) Log(_ context.Context, _, _, _, _, _ string, _ map[string]any) {
	a.calls++
}

func validTable(orgID, areaID uuid.UUID) domain.DiningTable {
	return domain.DiningTable{
		OrgID:    orgID,
		AreaID:   areaID,
		Code:     "M01",
		Label:    "Mesa 1",
		Capacity: 4,
		Status:   "available",
	}
}

// --- tests ---

func TestCreateHappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	audit := &fakeAudit{}
	uc := NewUsecases(repo, &fakeAreaLookup{exists: true}, audit)

	orgID := uuid.New()
	areaID := uuid.New()
	out, err := uc.Create(context.Background(), validTable(orgID, areaID), "user-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Code != "M01" {
		t.Errorf("expected code M01, got %s", out.Code)
	}
	if out.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if audit.calls != 1 {
		t.Errorf("expected 1 audit call, got %d", audit.calls)
	}
}

func TestCreateCodeRequired(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), &fakeAreaLookup{exists: true}, nil)

	in := validTable(uuid.New(), uuid.New())
	in.Code = ""
	_, err := uc.Create(context.Background(), in, "user-1")

	if err == nil {
		t.Fatal("expected error for empty code")
	}
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Errorf("expected ErrBadInput, got %v", err)
	}
}

func TestCreateInvalidCapacity(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), &fakeAreaLookup{exists: true}, nil)

	cases := []struct {
		name string
		cap  int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too large", 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := validTable(uuid.New(), uuid.New())
			in.Capacity = tc.cap
			_, err := uc.Create(context.Background(), in, "user-1")
			if !errors.Is(err, httperrors.ErrBadInput) {
				t.Errorf("expected ErrBadInput for capacity %d, got %v", tc.cap, err)
			}
		})
	}
}

func TestCreateInvalidStatus(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), &fakeAreaLookup{exists: true}, nil)

	in := validTable(uuid.New(), uuid.New())
	in.Status = "broken"
	_, err := uc.Create(context.Background(), in, "user-1")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Errorf("expected ErrBadInput, got %v", err)
	}
}

func TestCreateAreaNotFound(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), &fakeAreaLookup{exists: false}, nil)

	_, err := uc.Create(context.Background(), validTable(uuid.New(), uuid.New()), "user-1")
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateDuplicateCode(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	repo.createFn = func(_ context.Context, _ domain.DiningTable) (domain.DiningTable, error) {
		return domain.DiningTable{}, ErrDuplicateTableCode
	}
	uc := NewUsecases(repo, &fakeAreaLookup{exists: true}, nil)

	_, err := uc.Create(context.Background(), validTable(uuid.New(), uuid.New()), "user-1")
	if !errors.Is(err, httperrors.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestGetByIDHappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, &fakeAreaLookup{exists: true}, nil)

	orgID := uuid.New()
	id := uuid.New()
	repo.tables[id] = domain.DiningTable{ID: id, OrgID: orgID, Code: "M01", Capacity: 4, Status: "available"}

	out, err := uc.GetByID(context.Background(), orgID, id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Code != "M01" {
		t.Errorf("expected code M01, got %s", out.Code)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), &fakeAreaLookup{exists: true}, nil)

	_, err := uc.GetByID(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateHappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	audit := &fakeAudit{}
	uc := NewUsecases(repo, &fakeAreaLookup{exists: true}, audit)

	orgID := uuid.New()
	areaID := uuid.New()
	id := uuid.New()
	repo.tables[id] = domain.DiningTable{ID: id, OrgID: orgID, AreaID: areaID, Code: "M01", Label: "Mesa 1", Capacity: 4, Status: "available"}

	newLabel := "Mesa VIP"
	newCap := 6
	out, err := uc.Update(context.Background(), orgID, id, UpdateInput{
		Label:    &newLabel,
		Capacity: &newCap,
	}, "user-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Label != "Mesa VIP" {
		t.Errorf("expected label Mesa VIP, got %s", out.Label)
	}
	if out.Capacity != 6 {
		t.Errorf("expected capacity 6, got %d", out.Capacity)
	}
	if audit.calls != 1 {
		t.Errorf("expected 1 audit call, got %d", audit.calls)
	}
}

func TestUpdateNotFound(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), &fakeAreaLookup{exists: true}, nil)

	label := "Foo"
	_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), UpdateInput{Label: &label}, "user-1")
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateDuplicateCode(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	repo.updateFn = func(_ context.Context, _ domain.DiningTable) (domain.DiningTable, error) {
		return domain.DiningTable{}, ErrDuplicateTableCode
	}
	uc := NewUsecases(repo, &fakeAreaLookup{exists: true}, nil)

	orgID := uuid.New()
	areaID := uuid.New()
	id := uuid.New()
	repo.tables[id] = domain.DiningTable{ID: id, OrgID: orgID, AreaID: areaID, Code: "M01", Capacity: 4, Status: "available"}

	newCode := "M02"
	_, err := uc.Update(context.Background(), orgID, id, UpdateInput{Code: &newCode}, "user-1")
	if !errors.Is(err, httperrors.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestListReturnsItems(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, &fakeAreaLookup{exists: true}, nil)

	orgID := uuid.New()
	repo.tables[uuid.New()] = domain.DiningTable{OrgID: orgID, Code: "M01"}
	repo.tables[uuid.New()] = domain.DiningTable{OrgID: orgID, Code: "M02"}

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
