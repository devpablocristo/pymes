package bicycles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/bicycles/usecases/domain"
)

type fakeRepo struct {
	created domain.Bicycle
}

func (f *fakeRepo) List(ctx context.Context, p ListParams) ([]domain.Bicycle, int64, bool, *uuid.UUID, error) {
	_ = ctx
	_ = p
	return nil, 0, false, nil, nil
}

func (f *fakeRepo) ListArchived(ctx context.Context, orgID uuid.UUID) ([]domain.Bicycle, error) {
	_ = ctx
	_ = orgID
	return nil, nil
}

func (f *fakeRepo) Create(ctx context.Context, in domain.Bicycle) (domain.Bicycle, error) {
	_ = ctx
	f.created = in
	in.ID = uuid.New()
	return in, nil
}

func (f *fakeRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Bicycle, error) {
	_ = ctx
	_ = orgID
	_ = id
	return domain.Bicycle{}, errors.New("not implemented")
}

func (f *fakeRepo) Update(ctx context.Context, in domain.Bicycle) (domain.Bicycle, error) {
	_ = ctx
	return in, nil
}

func (f *fakeRepo) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	_ = ctx
	_ = orgID
	_ = id
	return nil
}

func (f *fakeRepo) Restore(ctx context.Context, orgID, id uuid.UUID) error {
	_ = ctx
	_ = orgID
	_ = id
	return nil
}

func (f *fakeRepo) HardDelete(ctx context.Context, orgID, id uuid.UUID) error {
	_ = ctx
	_ = orgID
	_ = id
	return nil
}

type fakeCP struct {
	customer map[string]any
	party    map[string]any
	err      error
}

func (f *fakeCP) GetCustomer(ctx context.Context, orgID, customerID string) (map[string]any, error) {
	_ = ctx
	_ = orgID
	_ = customerID
	if f.customer == nil {
		return nil, f.err
	}
	return f.customer, nil
}

func (f *fakeCP) GetParty(ctx context.Context, orgID, partyID string) (map[string]any, error) {
	_ = ctx
	_ = orgID
	_ = partyID
	if f.err != nil {
		return nil, f.err
	}
	return f.party, nil
}

func TestCreateAutofillsCustomerNameFromPymesCore(t *testing.T) {
	repo := &fakeRepo{}
	cp := &fakeCP{customer: map[string]any{"name": "Ana Bici"}}
	uc := NewUsecases(repo, nil, cp)
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	customerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	out, err := uc.Create(context.Background(), domain.Bicycle{
		OrgID:       orgID,
		CustomerID:  &customerID,
		FrameNumber: " bike-123 ",
		Brand:       "Trek",
		Model:       "Marlin 7",
	}, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if out.CustomerName != "Ana Bici" {
		t.Fatalf("Create().CustomerName = %q, want Ana Bici", out.CustomerName)
	}
	if repo.created.CustomerName != "Ana Bici" {
		t.Fatalf("repo.created.CustomerName = %q, want Ana Bici", repo.created.CustomerName)
	}
	if out.FrameNumber != "BIKE-123" {
		t.Fatalf("Create().FrameNumber = %q, want BIKE-123", out.FrameNumber)
	}
}

func TestCreateRejectsInvalidCustomerReference(t *testing.T) {
	repo := &fakeRepo{}
	cp := &fakeCP{err: errors.New("not found")}
	uc := NewUsecases(repo, nil, cp)
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	customerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	_, err := uc.Create(context.Background(), domain.Bicycle{
		OrgID:       orgID,
		CustomerID:  &customerID,
		FrameNumber: "BIKE-123",
		Brand:       "Specialized",
		Model:       "Rockhopper",
	}, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}
