package bicycles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/bicycles/usecases/domain"
)

// --- fakes ---

type fakeRepo struct {
	items      []domain.Bicycle
	created    domain.Bicycle
	updated    domain.Bicycle
	getByIDErr error
	createErr  error
	updateErr  error
}

func (f *fakeRepo) List(_ context.Context, _ ListParams) ([]domain.Bicycle, int64, bool, *uuid.UUID, error) {
	return f.items, int64(len(f.items)), false, nil, nil
}

func (f *fakeRepo) Create(_ context.Context, in domain.Bicycle) (domain.Bicycle, error) {
	if f.createErr != nil {
		return domain.Bicycle{}, f.createErr
	}
	f.created = in
	in.ID = uuid.New()
	return in, nil
}

func (f *fakeRepo) GetByID(_ context.Context, _ uuid.UUID, _ uuid.UUID) (domain.Bicycle, error) {
	if f.getByIDErr != nil {
		return domain.Bicycle{}, f.getByIDErr
	}
	if len(f.items) > 0 {
		return f.items[0], nil
	}
	return domain.Bicycle{}, gorm.ErrRecordNotFound
}

func (f *fakeRepo) Update(_ context.Context, in domain.Bicycle) (domain.Bicycle, error) {
	if f.updateErr != nil {
		return domain.Bicycle{}, f.updateErr
	}
	f.updated = in
	return in, nil
}

type fakeCP struct {
	customer map[string]any
	party    map[string]any
	err      error
}

func (f *fakeCP) GetCustomer(_ context.Context, _, _ string) (map[string]any, error) {
	if f.customer == nil {
		return nil, f.err
	}
	return f.customer, nil
}

func (f *fakeCP) GetParty(_ context.Context, _, _ string) (map[string]any, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.party, nil
}

// --- helpers ---

func validBicycle() domain.Bicycle {
	return domain.Bicycle{
		OrgID:           uuid.New(),
		FrameNumber:     "ABC123",
		Make:            "Trek",
		Model:           "Marlin 7",
		BikeType:        "mtb",
		WheelSizeInches: 29,
	}
}

// --- tests ---

func TestCreateHappyPath(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validBicycle()
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if out.ID == uuid.Nil {
		t.Fatal("Create() returned nil ID")
	}
	if out.FrameNumber != "ABC123" {
		t.Fatalf("Create().FrameNumber = %q, want ABC123", out.FrameNumber)
	}
}

func TestCreateNormalizesFrameNumber(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validBicycle()
	in.FrameNumber = "  abc123  "
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if out.FrameNumber != "ABC123" {
		t.Fatalf("Create().FrameNumber = %q, want ABC123", out.FrameNumber)
	}
}

func TestCreateRejectsMissingFrameNumber(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validBicycle()
	in.FrameNumber = ""
	_, err := uc.Create(context.Background(), in, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}

func TestCreateRejectsShortMake(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validBicycle()
	in.Make = "X"
	_, err := uc.Create(context.Background(), in, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}

func TestCreateRejectsMissingModel(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validBicycle()
	in.Model = ""
	_, err := uc.Create(context.Background(), in, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}

func TestCreateRejectsInvalidWheelSize(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validBicycle()
	in.WheelSizeInches = 100
	_, err := uc.Create(context.Background(), in, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}

func TestCreateEnrichesCustomerName(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	cp := &fakeCP{customer: map[string]any{"name": "Carlos García"}}
	uc := NewUsecases(repo, nil, cp)

	in := validBicycle()
	customerID := uuid.New()
	in.CustomerID = &customerID
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if out.CustomerName != "Carlos García" {
		t.Fatalf("Create().CustomerName = %q, want Carlos García", out.CustomerName)
	}
}

func TestCreateRejectsInvalidCustomer(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	cp := &fakeCP{err: errors.New("not found")}
	uc := NewUsecases(repo, nil, cp)

	in := validBicycle()
	customerID := uuid.New()
	in.CustomerID = &customerID
	_, err := uc.Create(context.Background(), in, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}

func TestGetByIDHappyPath(t *testing.T) {
	t.Parallel()
	bike := validBicycle()
	bike.ID = uuid.New()
	repo := &fakeRepo{items: []domain.Bicycle{bike}}
	uc := NewUsecases(repo, nil, nil)

	out, err := uc.GetByID(context.Background(), bike.OrgID, bike.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if out.ID != bike.ID {
		t.Fatalf("GetByID().ID = %v, want %v", out.ID, bike.ID)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{getByIDErr: gorm.ErrRecordNotFound}
	uc := NewUsecases(repo, nil, nil)

	_, err := uc.GetByID(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestListReturnsItems(t *testing.T) {
	t.Parallel()
	bike := validBicycle()
	bike.ID = uuid.New()
	repo := &fakeRepo{items: []domain.Bicycle{bike}}
	uc := NewUsecases(repo, nil, nil)

	items, total, _, _, err := uc.List(context.Background(), ListParams{OrgID: bike.OrgID, Limit: 25})
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

func TestUpdateHappyPath(t *testing.T) {
	t.Parallel()
	bike := validBicycle()
	bike.ID = uuid.New()
	repo := &fakeRepo{items: []domain.Bicycle{bike}}
	uc := NewUsecases(repo, nil, nil)

	newColor := "Rojo"
	out, err := uc.Update(context.Background(), bike.OrgID, bike.ID, UpdateInput{
		Color: &newColor,
	}, "tester")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if out.Color != "Rojo" {
		t.Fatalf("Update().Color = %q, want Rojo", out.Color)
	}
}

func TestUpdateNotFound(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{getByIDErr: gorm.ErrRecordNotFound}
	uc := NewUsecases(repo, nil, nil)

	notes := "test"
	_, err := uc.Update(context.Background(), uuid.New(), uuid.New(), UpdateInput{Notes: &notes}, "tester")
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("Update() error = %v, want ErrNotFound", err)
	}
}

func TestUpdateValidatesAfterPatch(t *testing.T) {
	t.Parallel()
	bike := validBicycle()
	bike.ID = uuid.New()
	repo := &fakeRepo{items: []domain.Bicycle{bike}}
	uc := NewUsecases(repo, nil, nil)

	emptyMake := ""
	_, err := uc.Update(context.Background(), bike.OrgID, bike.ID, UpdateInput{
		Make: &emptyMake,
	}, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Update() error = %v, want ErrBadInput", err)
	}
}
