package professional_profiles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/professionals/backend/internal/teachers/professional_profiles/usecases/domain"
)

// --- fakes ---

type fakeRepo struct {
	profiles   map[uuid.UUID]domain.ProfessionalProfile
	slugExists bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{profiles: make(map[uuid.UUID]domain.ProfessionalProfile)}
}

func (f *fakeRepo) List(_ context.Context, _ ListParams) ([]domain.ProfessionalProfile, int64, bool, *uuid.UUID, error) {
	out := make([]domain.ProfessionalProfile, 0, len(f.profiles))
	for _, p := range f.profiles {
		out = append(out, p)
	}
	return out, int64(len(out)), false, nil, nil
}

func (f *fakeRepo) Create(_ context.Context, in domain.ProfessionalProfile) (domain.ProfessionalProfile, error) {
	in.ID = uuid.New()
	f.profiles[in.ID] = in
	return in, nil
}

func (f *fakeRepo) GetByID(_ context.Context, orgID, id uuid.UUID) (domain.ProfessionalProfile, error) {
	p, ok := f.profiles[id]
	if !ok || p.OrgID != orgID {
		return domain.ProfessionalProfile{}, gorm.ErrRecordNotFound
	}
	return p, nil
}

func (f *fakeRepo) GetBySlug(_ context.Context, orgID uuid.UUID, slug string) (domain.ProfessionalProfile, error) {
	for _, p := range f.profiles {
		if p.OrgID == orgID && p.PublicSlug == slug {
			return p, nil
		}
	}
	return domain.ProfessionalProfile{}, gorm.ErrRecordNotFound
}

func (f *fakeRepo) SlugExists(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID) (bool, error) {
	return f.slugExists, nil
}

func (f *fakeRepo) Update(_ context.Context, in domain.ProfessionalProfile) (domain.ProfessionalProfile, error) {
	if _, ok := f.profiles[in.ID]; !ok {
		return domain.ProfessionalProfile{}, gorm.ErrRecordNotFound
	}
	f.profiles[in.ID] = in
	return in, nil
}

func (f *fakeRepo) ListPublic(_ context.Context, orgID uuid.UUID) ([]domain.ProfessionalProfile, error) {
	var out []domain.ProfessionalProfile
	for _, p := range f.profiles {
		if p.OrgID == orgID && p.IsPublic {
			out = append(out, p)
		}
	}
	return out, nil
}

type fakeAudit struct{ calls int }

func (f *fakeAudit) Log(_ context.Context, _, _, _, _, _ string, _ map[string]any) {
	f.calls++
}

// --- tests ---

func TestCreate_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, &fakeAudit{})

	orgID := uuid.New()
	partyID := uuid.New()
	out, err := uc.Create(context.Background(), domain.ProfessionalProfile{
		OrgID:    orgID,
		PartyID:  partyID,
		Headline: "Dr. García",
	}, "tester")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID == uuid.Nil {
		t.Fatal("expected non-nil ID")
	}
	if out.PublicSlug == "" {
		t.Fatal("expected auto-generated slug")
	}
	if out.Metadata == nil {
		t.Fatal("expected metadata to be initialized")
	}
}

func TestCreate_MissingPartyID(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	_, err := uc.Create(context.Background(), domain.ProfessionalProfile{
		OrgID: uuid.New(),
	}, "tester")

	if err == nil {
		t.Fatal("expected error for missing party_id")
	}
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("expected ErrBadInput, got %v", err)
	}
}

func TestCreate_DuplicateSlug(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	repo.slugExists = true
	uc := NewUsecases(repo, nil)

	_, err := uc.Create(context.Background(), domain.ProfessionalProfile{
		OrgID:      uuid.New(),
		PartyID:    uuid.New(),
		PublicSlug: "duplicated",
	}, "tester")

	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !errors.Is(err, httperrors.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	_, err := uc.GetByID(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetByID_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	orgID := uuid.New()
	created, err := uc.Create(context.Background(), domain.ProfessionalProfile{
		OrgID:   orgID,
		PartyID: uuid.New(),
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

func TestUpdate_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	audit := &fakeAudit{}
	uc := NewUsecases(repo, audit)

	orgID := uuid.New()
	created, err := uc.Create(context.Background(), domain.ProfessionalProfile{
		OrgID:    orgID,
		PartyID:  uuid.New(),
		Headline: "Original",
	}, "tester")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	newBio := "Especialista en traumatología"
	updated, err := uc.Update(context.Background(), orgID, created.ID, UpdateInput{
		Bio: &newBio,
	}, "tester")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Bio != newBio {
		t.Fatalf("expected bio %q, got %q", newBio, updated.Bio)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	bio := "test"
	_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), UpdateInput{Bio: &bio}, "tester")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetBySlug_NotPublic(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	orgID := uuid.New()
	created, err := uc.Create(context.Background(), domain.ProfessionalProfile{
		OrgID:      orgID,
		PartyID:    uuid.New(),
		PublicSlug: "private-prof",
		IsPublic:   false,
	}, "tester")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	_ = created

	_, err = uc.GetBySlug(context.Background(), orgID, "private-prof")
	if err == nil {
		t.Fatal("expected not found for non-public profile")
	}
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetBySlug_HappyPath(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	orgID := uuid.New()
	_, err := uc.Create(context.Background(), domain.ProfessionalProfile{
		OrgID:      orgID,
		PartyID:    uuid.New(),
		PublicSlug: "dr-garcia",
		IsPublic:   true,
	}, "tester")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := uc.GetBySlug(context.Background(), orgID, "dr-garcia")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PublicSlug != "dr-garcia" {
		t.Fatalf("expected slug dr-garcia, got %s", got.PublicSlug)
	}
}

func TestListPublic(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil)

	orgID := uuid.New()
	// Crear uno público y uno privado
	_, err := uc.Create(context.Background(), domain.ProfessionalProfile{
		OrgID:      orgID,
		PartyID:    uuid.New(),
		PublicSlug: "public-one",
		IsPublic:   true,
	}, "tester")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	repo.slugExists = false
	_, err = uc.Create(context.Background(), domain.ProfessionalProfile{
		OrgID:      orgID,
		PartyID:    uuid.New(),
		PublicSlug: "private-one",
		IsPublic:   false,
	}, "tester")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	list, err := uc.ListPublic(context.Background(), orgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 public profile, got %d", len(list))
	}
}
