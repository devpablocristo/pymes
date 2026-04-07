package workorders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/workorders/usecases/domain"
)

// --- fakes ---

type fakeRepo struct {
	items        []domain.WorkOrder
	created      domain.WorkOrder
	updated      domain.WorkOrder
	getByIDErr   error
	createErr    error
	updateErr    error
	saveIntErr   error
	saveIntOut   domain.WorkOrder
}

func (f *fakeRepo) List(_ context.Context, _ ListParams) ([]domain.WorkOrder, int64, bool, *uuid.UUID, error) {
	return f.items, int64(len(f.items)), false, nil, nil
}

func (f *fakeRepo) Create(_ context.Context, in domain.WorkOrder) (domain.WorkOrder, error) {
	if f.createErr != nil {
		return domain.WorkOrder{}, f.createErr
	}
	f.created = in
	in.ID = uuid.New()
	return in, nil
}

func (f *fakeRepo) GetByID(_ context.Context, _ uuid.UUID, _ uuid.UUID) (domain.WorkOrder, error) {
	if f.getByIDErr != nil {
		return domain.WorkOrder{}, f.getByIDErr
	}
	if len(f.items) > 0 {
		return f.items[0], nil
	}
	return domain.WorkOrder{}, gorm.ErrRecordNotFound
}

func (f *fakeRepo) Update(_ context.Context, in domain.WorkOrder) (domain.WorkOrder, error) {
	if f.updateErr != nil {
		return domain.WorkOrder{}, f.updateErr
	}
	f.updated = in
	return in, nil
}

func (f *fakeRepo) SaveIntegrations(_ context.Context, _, _ uuid.UUID, _, _ *uuid.UUID, _ *string) (domain.WorkOrder, error) {
	if f.saveIntErr != nil {
		return domain.WorkOrder{}, f.saveIntErr
	}
	return f.saveIntOut, nil
}

type fakeCP struct {
	customer map[string]any
	party    map[string]any
	product  map[string]any
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

func (f *fakeCP) GetProduct(_ context.Context, _, _ string) (map[string]any, error) {
	if f.product == nil {
		return nil, f.err
	}
	return f.product, nil
}

// --- helpers ---

func validItem() domain.WorkOrderItem {
	return domain.WorkOrderItem{
		ItemType:    "service",
		Description: "Ajuste de frenos",
		Quantity:    1,
		UnitPrice:   100,
		TaxRate:     21,
	}
}

func validWorkOrder() domain.WorkOrder {
	return domain.WorkOrder{
		OrgID:     uuid.New(),
		BicycleID: uuid.New(),
		Number:    "OT-BIKE-TEST-001",
		OpenedAt:  time.Now().UTC(),
		Currency:  "ARS",
		Items:     []domain.WorkOrderItem{validItem()},
	}
}

// --- tests ---

func TestCreateHappyPath(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validWorkOrder()
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if out.ID == uuid.Nil {
		t.Fatal("Create() returned nil ID")
	}
	if out.Currency != "ARS" {
		t.Fatalf("Create().Currency = %q, want ARS", out.Currency)
	}
	if out.Total == 0 {
		t.Fatal("Create().Total should not be zero")
	}
}

func TestCreateDefaultsCurrency(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validWorkOrder()
	in.Currency = ""
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if out.Currency != "ARS" {
		t.Fatalf("Create().Currency = %q, want ARS", out.Currency)
	}
}

func TestCreateAutoGeneratesNumber(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validWorkOrder()
	in.Number = ""
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if out.Number == "" {
		t.Fatal("Create() should auto-generate a number")
	}
}

func TestCreateRejectsMissingBicycleID(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validWorkOrder()
	in.BicycleID = uuid.Nil
	_, err := uc.Create(context.Background(), in, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}

func TestCreateRejectsMissingItems(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validWorkOrder()
	in.Items = nil
	_, err := uc.Create(context.Background(), in, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}

func TestCreateEnrichesCustomerName(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	cp := &fakeCP{customer: map[string]any{"name": "María López"}}
	uc := NewUsecases(repo, nil, cp)

	in := validWorkOrder()
	customerID := uuid.New()
	in.CustomerID = &customerID
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if out.CustomerName != "María López" {
		t.Fatalf("Create().CustomerName = %q, want María López", out.CustomerName)
	}
}

func TestCreateRejectsInvalidCustomer(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	cp := &fakeCP{err: errors.New("not found")}
	uc := NewUsecases(repo, nil, cp)

	in := validWorkOrder()
	customerID := uuid.New()
	in.CustomerID = &customerID
	_, err := uc.Create(context.Background(), in, "tester")
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("Create() error = %v, want ErrBadInput", err)
	}
}

func TestCreateCalculatesTotals(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, nil, nil)

	in := validWorkOrder()
	in.Items = []domain.WorkOrderItem{
		{ItemType: "service", Description: "Ajuste", Quantity: 2, UnitPrice: 100, TaxRate: 10},
		{ItemType: "part", Description: "Pastilla", Quantity: 1, UnitPrice: 50, TaxRate: 21},
	}
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	// subtotalServices = 2*100 = 200, subtotalParts = 1*50 = 50
	// taxTotal = 200*10/100 + 50*21/100 = 20 + 10.5 = 30.5
	// total = 200 + 50 + 30.5 = 280.5
	if out.SubtotalServices != 200 {
		t.Fatalf("SubtotalServices = %f, want 200", out.SubtotalServices)
	}
	if out.SubtotalParts != 50 {
		t.Fatalf("SubtotalParts = %f, want 50", out.SubtotalParts)
	}
	if out.TaxTotal != 30.5 {
		t.Fatalf("TaxTotal = %f, want 30.5", out.TaxTotal)
	}
	if out.Total != 280.5 {
		t.Fatalf("Total = %f, want 280.5", out.Total)
	}
}

func TestGetByIDHappyPath(t *testing.T) {
	t.Parallel()
	wo := validWorkOrder()
	wo.ID = uuid.New()
	repo := &fakeRepo{items: []domain.WorkOrder{wo}}
	uc := NewUsecases(repo, nil, nil)

	out, err := uc.GetByID(context.Background(), wo.OrgID, wo.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if out.ID != wo.ID {
		t.Fatalf("GetByID().ID = %v, want %v", out.ID, wo.ID)
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
	wo := validWorkOrder()
	wo.ID = uuid.New()
	repo := &fakeRepo{items: []domain.WorkOrder{wo}}
	uc := NewUsecases(repo, nil, nil)

	items, total, _, _, err := uc.List(context.Background(), ListParams{OrgID: wo.OrgID, Limit: 25})
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
	wo := validWorkOrder()
	wo.ID = uuid.New()
	wo.Status = "received"
	wo.Items = []domain.WorkOrderItem{validItem()}
	repo := &fakeRepo{items: []domain.WorkOrder{wo}}
	uc := NewUsecases(repo, nil, nil)

	newDiag := "Cadena estirada"
	out, err := uc.Update(context.Background(), wo.OrgID, wo.ID, UpdateInput{
		Diagnosis: &newDiag,
	}, "tester")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if out.Diagnosis != "Cadena estirada" {
		t.Fatalf("Update().Diagnosis = %q, want 'Cadena estirada'", out.Diagnosis)
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

func TestSaveIntegrationsNotFound(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{saveIntErr: gorm.ErrRecordNotFound}
	uc := NewUsecases(repo, nil, nil)

	_, err := uc.SaveIntegrations(context.Background(), uuid.New(), uuid.New(), nil, nil, nil, "tester")
	if !errors.Is(err, httperrors.ErrNotFound) {
		t.Fatalf("SaveIntegrations() error = %v, want ErrNotFound", err)
	}
}
