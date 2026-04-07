package staff

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/beauty/backend/internal/salon/staff/usecases/domain"
)

// --- fakes ---

type fakeRepo struct {
	items      []domain.StaffMember
	created    domain.StaffMember
	updated    domain.StaffMember
	getByIDErr error
	createErr  error
	updateErr  error
}

func (f *fakeRepo) List(_ context.Context, _ ListParams) ([]domain.StaffMember, int64, bool, *uuid.UUID, error) {
	return f.items, int64(len(f.items)), false, nil, nil
}

func (f *fakeRepo) Create(_ context.Context, in domain.StaffMember) (domain.StaffMember, error) {
	if f.createErr != nil {
		return domain.StaffMember{}, f.createErr
	}
	f.created = in
	in.ID = uuid.New()
	return in, nil
}

func (f *fakeRepo) GetByID(_ context.Context, _ uuid.UUID, _ uuid.UUID) (domain.StaffMember, error) {
	if f.getByIDErr != nil {
		return domain.StaffMember{}, f.getByIDErr
	}
	if len(f.items) > 0 {
		return f.items[0], nil
	}
	return domain.StaffMember{}, gorm.ErrRecordNotFound
}

func (f *fakeRepo) Update(_ context.Context, in domain.StaffMember) (domain.StaffMember, error) {
	if f.updateErr != nil {
		return domain.StaffMember{}, f.updateErr
	}
	f.updated = in
	return in, nil
}

// --- helpers ---

func validStaff() domain.StaffMember {
	return domain.StaffMember{
		OrgID:       uuid.New(),
		DisplayName: "Ana Torres",
		Role:        "stylist",
		Color:       "#ff6600",
		IsActive:    true,
	}
}

// --- tests ---

func TestCreateHappyPath(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil)

	in := validStaff()
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if out.ID == uuid.Nil {
		t.Fatal("Create() returned nil ID")
	}
	if out.DisplayName != "Ana Torres" {
		t.Fatalf("Create().DisplayName = %q, want Ana Torres", out.DisplayName)
	}
}

func TestCreateDefaultsColor(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil)

	in := validStaff()
	in.Color = ""
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if out.Color != "#6366f1" {
		t.Fatalf("Create().Color = %q, want #6366f1", out.Color)
	}
}

func TestCreateRejectsShortDisplayName(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil)

	in := validStaff()
	in.DisplayName = "A"
	_, err := uc.Create(context.Background(), in, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}

func TestCreateRejectsEmptyDisplayName(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil)

	in := validStaff()
	in.DisplayName = ""
	_, err := uc.Create(context.Background(), in, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}

func TestCreateTrimsWhitespace(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil)

	in := validStaff()
	in.DisplayName = "  Ana Torres  "
	in.Role = "  stylist  "
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if out.DisplayName != "Ana Torres" {
		t.Fatalf("Create().DisplayName = %q, want Ana Torres", out.DisplayName)
	}
	if out.Role != "stylist" {
		t.Fatalf("Create().Role = %q, want stylist", out.Role)
	}
}

func TestGetByIDHappyPath(t *testing.T) {
	t.Parallel()
	member := validStaff()
	member.ID = uuid.New()
	repo := &fakeRepo{items: []domain.StaffMember{member}}
	uc := NewUsecases(repo, nil)

	out, err := uc.GetByID(context.Background(), member.OrgID, member.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if out.ID != member.ID {
		t.Fatalf("GetByID().ID = %v, want %v", out.ID, member.ID)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{getByIDErr: gorm.ErrRecordNotFound}
	uc := NewUsecases(repo, nil)

	_, err := uc.GetByID(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestListReturnsItems(t *testing.T) {
	t.Parallel()
	member := validStaff()
	member.ID = uuid.New()
	repo := &fakeRepo{items: []domain.StaffMember{member}}
	uc := NewUsecases(repo, nil)

	items, total, _, _, err := uc.List(context.Background(), ListParams{OrgID: member.OrgID, Limit: 25})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("List() total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Fatalf("List() len = %d, want 1", len(items))
	}
}

func TestListEmpty(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{items: []domain.StaffMember{}}
	uc := NewUsecases(repo, nil)

	items, total, _, _, err := uc.List(context.Background(), ListParams{OrgID: uuid.New(), Limit: 25})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 0 {
		t.Fatalf("List() total = %d, want 0", total)
	}
	if len(items) != 0 {
		t.Fatalf("List() len = %d, want 0", len(items))
	}
}

func TestUpdateHappyPath(t *testing.T) {
	t.Parallel()
	member := validStaff()
	member.ID = uuid.New()
	repo := &fakeRepo{items: []domain.StaffMember{member}}
	uc := NewUsecases(repo, nil)

	newRole := "colorist"
	out, err := uc.Update(context.Background(), member.OrgID, member.ID, UpdateInput{
		Role: &newRole,
	}, "tester")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if out.Role != "colorist" {
		t.Fatalf("Update().Role = %q, want colorist", out.Role)
	}
}

func TestUpdateNotFound(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{getByIDErr: gorm.ErrRecordNotFound}
	uc := NewUsecases(repo, nil)

	notes := "test"
	_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), UpdateInput{Notes: &notes}, "tester")
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("Update() error = %v, want ErrNotFound", err)
	}
}

func TestUpdateValidatesAfterPatch(t *testing.T) {
	t.Parallel()
	member := validStaff()
	member.ID = uuid.New()
	repo := &fakeRepo{items: []domain.StaffMember{member}}
	uc := NewUsecases(repo, nil)

	emptyName := ""
	_, err := uc.Update(context.Background(), member.OrgID, member.ID, UpdateInput{
		DisplayName: &emptyName,
	}, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Update() error = %v, want ErrBadInput", err)
	}
}

func TestUpdateIsActive(t *testing.T) {
	t.Parallel()
	member := validStaff()
	member.ID = uuid.New()
	member.IsActive = true
	repo := &fakeRepo{items: []domain.StaffMember{member}}
	uc := NewUsecases(repo, nil)

	inactive := false
	out, err := uc.Update(context.Background(), member.OrgID, member.ID, UpdateInput{
		IsActive: &inactive,
	}, "tester")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if out.IsActive {
		t.Fatal("Update().IsActive = true, want false")
	}
}
