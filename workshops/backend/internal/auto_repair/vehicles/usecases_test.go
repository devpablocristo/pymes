package vehicles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles/usecases/domain"
)

type fakeRepo struct {
	created domain.Vehicle
}

func (f *fakeRepo) List(ctx context.Context, p ListParams) ([]domain.Vehicle, int64, bool, *uuid.UUID, error) {
	_ = ctx
	_ = p
	return nil, 0, false, nil, nil
}

func (f *fakeRepo) Create(ctx context.Context, in domain.Vehicle) (domain.Vehicle, error) {
	_ = ctx
	f.created = in
	in.ID = uuid.New()
	return in, nil
}

func (f *fakeRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Vehicle, error) {
	_ = ctx
	_ = orgID
	_ = id
	return domain.Vehicle{}, errors.New("not implemented")
}

func (f *fakeRepo) Update(ctx context.Context, in domain.Vehicle) (domain.Vehicle, error) {
	_ = ctx
	return in, nil
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
	cp := &fakeCP{customer: map[string]any{"name": "Juan Perez"}}
	uc := NewUsecases(repo, nil, cp)
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	customerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	out, err := uc.Create(context.Background(), domain.Vehicle{
		OrgID:        orgID,
		CustomerID:   &customerID,
		LicensePlate: "ab123cd",
		Make:         "Toyota",
		Model:        "Hilux",
	}, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if out.CustomerName != "Juan Perez" {
		t.Fatalf("Create().CustomerName = %q, want Juan Perez", out.CustomerName)
	}
	if repo.created.CustomerName != "Juan Perez" {
		t.Fatalf("repo.created.CustomerName = %q, want Juan Perez", repo.created.CustomerName)
	}
	if out.LicensePlate != "AB123CD" {
		t.Fatalf("Create().LicensePlate = %q, want AB123CD", out.LicensePlate)
	}
}

func TestCreateRejectsInvalidCustomerReference(t *testing.T) {
	repo := &fakeRepo{}
	cp := &fakeCP{err: errors.New("not found")}
	uc := NewUsecases(repo, nil, cp)
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	customerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	_, err := uc.Create(context.Background(), domain.Vehicle{
		OrgID:        orgID,
		CustomerID:   &customerID,
		LicensePlate: "AB123CD",
		Make:         "Ford",
		Model:        "Ranger",
	}, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}
