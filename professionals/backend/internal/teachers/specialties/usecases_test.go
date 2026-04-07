package specialties

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/specialties/usecases/domain"
)

// --- fakes ---

type fakeRepo struct {
	specialties map[uuid.UUID]domain.Specialty
	codeExists  bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{specialties: make(map[uuid.UUID]domain.Specialty)}
}

func (f *fakeRepo) List(_ context.Context, _ ListParams) ([]domain.Specialty, int64, bool, *uuid.UUID, error) {
	out := make([]domain.Specialty, 0, len(f.specialties))
	for _, s := range f.specialties {
		out = append(out, s)
	}
	return out, int64(len(out)), false, nil, nil
}

func (f *fakeRepo) Create(_ context.Context, in domain.Specialty) (domain.Specialty, error) {
	in.ID = uuid.New()
	f.specialties[in.ID] = in
	return in, nil
}

func (f *fakeRepo) GetByID(_ context.Context, orgID, id uuid.UUID) (domain.Specialty, error) {
	s, ok := f.specialties[id]
	if !ok || s.OrgID != orgID {
		return domain.Specialty{}, gorm.ErrRecordNotFound
	}
	return s, nil
}

func (f *fakeRepo) Update(_ context.Context, in domain.Specialty) (domain.Specialty, error) {
	if _, ok := f.specialties[in.ID]; !ok {
		return domain.Specialty{}, gorm.ErrRecordNotFound
	}
	f.specialties[in.ID] = in
	return in, nil
}

func (f *fakeRepo) CodeExists(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID) (bool, error) {
	return f.codeExists, nil
}

func (f *fakeRepo) AssignProfessionals(_ context.Context, _, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}

type fakeAudit struct{ calls int }

func (f *fakeAudit) Log(_ context.Context, _, _, _, _, _ string, _ map[string]any) {
	f.calls++
}

// --- tests ---

func TestCreate_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	audit := &fakeAudit{}
	uc := NewUsecases(repo, audit)

	out, err := uc.Create(context.Background(), domain.Specialty{
		OrgID: uuid.New(),
		Code:  "TRAU",
		Name:  "Traumatología",
	}, "tester")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID == uuid.Nil {
		t.Fatal("expected non-nil ID")
	}
	if audit.calls != 1 {
		t.Fatalf("expected 1 audit call, got %d", audit.calls)
	}
}

func TestCreate_CodeTooShort(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), nil)

	_, err := uc.Create(context.Background(), domain.Specialty{
		OrgID: uuid.New(),
		Code:  "X",
		Name:  "Válido",
	}, "tester")

	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("expected ErrBadInput, got %v", err)
	}
}

func TestCreate_NameTooShort(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), nil)

	_, err := uc.Create(context.Background(), domain.Specialty{
		OrgID: uuid.New(),
		Code:  "OK",
		Name:  "A",
	}, "tester")

	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("expected ErrBadInput, got %v", err)
	}
}

func TestCreate_DuplicateCode(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	repo.codeExists = true
	uc := NewUsecases(repo, nil)

	_, err := uc.Create(context.Background(), domain.Specialty{
		OrgID: uuid.New(),
		Code:  "DUP",
		Name:  "Duplicado",
	}, "tester")

	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !errors.Is(err, httperrors.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestGetByID_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	orgID := uuid.New()
	created, err := uc.Create(context.Background(), domain.Specialty{
		OrgID: orgID,
		Code:  "ORT",
		Name:  "Ortodoncia",
	}, "tester")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := uc.GetByID(context.Background(), orgID, created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected ID %s, got %s", created.ID, got.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), nil)

	_, err := uc.GetByID(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdate_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	audit := &fakeAudit{}
	uc := NewUsecases(repo, audit)

	orgID := uuid.New()
	created, err := uc.Create(context.Background(), domain.Specialty{
		OrgID: orgID,
		Code:  "FIS",
		Name:  "Fisioterapia",
	}, "tester")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	newName := "Fisioterapia Deportiva"
	updated, err := uc.Update(context.Background(), orgID, created.ID, UpdateInput{
		Name: &newName,
	}, "tester")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("expected name %q, got %q", newName, updated.Name)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), nil)

	name := "test"
	_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), UpdateInput{Name: &name}, "tester")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdate_DuplicateCode(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	orgID := uuid.New()
	created, err := uc.Create(context.Background(), domain.Specialty{
		OrgID: orgID,
		Code:  "ABC",
		Name:  "Original",
	}, "tester")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Ahora simular que el código ya existe
	repo.codeExists = true
	newCode := "TAKEN"
	_, err = uc.Update(context.Background(), orgID, created.ID, UpdateInput{Code: &newCode}, "tester")
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !errors.Is(err, httperrors.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestAssignProfessionals_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	audit := &fakeAudit{}
	uc := NewUsecases(repo, audit)

	orgID := uuid.New()
	created, err := uc.Create(context.Background(), domain.Specialty{
		OrgID: orgID,
		Code:  "PSI",
		Name:  "Psicología",
	}, "tester")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	profileIDs := []uuid.UUID{uuid.New(), uuid.New()}
	err = uc.AssignProfessionals(context.Background(), orgID, created.ID, profileIDs, "tester")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1 del Create + 1 del Assign
	if audit.calls != 2 {
		t.Fatalf("expected 2 audit calls, got %d", audit.calls)
	}
}

func TestAssignProfessionals_NotFound(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), nil)

	err := uc.AssignProfessionals(context.Background(), uuid.New(), uuid.New(), []uuid.UUID{uuid.New()}, "tester")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
