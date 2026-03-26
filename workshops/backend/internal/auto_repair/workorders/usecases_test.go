package workorders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/workorders/usecases/domain"
)

type fakeRepo struct {
	created domain.WorkOrder
	orders  map[string]domain.WorkOrder
}

func fakeOrderKey(orgID, id uuid.UUID) string {
	return orgID.String() + ":" + id.String()
}

func (f *fakeRepo) List(ctx context.Context, p ListParams) ([]domain.WorkOrder, int64, bool, *uuid.UUID, error) {
	_ = ctx
	_ = p
	return nil, 0, false, nil, nil
}

func (f *fakeRepo) Create(ctx context.Context, in domain.WorkOrder) (domain.WorkOrder, error) {
	_ = ctx
	f.created = in
	if in.ID == uuid.Nil {
		in.ID = uuid.New()
	}
	f.created = in
	if f.orders != nil {
		f.orders[fakeOrderKey(in.OrgID, in.ID)] = in
	}
	return in, nil
}

func (f *fakeRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.WorkOrder, error) {
	_ = ctx
	if f.orders == nil {
		return domain.WorkOrder{}, gorm.ErrRecordNotFound
	}
	wo, ok := f.orders[fakeOrderKey(orgID, id)]
	if !ok {
		return domain.WorkOrder{}, gorm.ErrRecordNotFound
	}
	return wo, nil
}

func (f *fakeRepo) Update(ctx context.Context, in domain.WorkOrder) (domain.WorkOrder, error) {
	_ = ctx
	if f.orders == nil {
		return domain.WorkOrder{}, gorm.ErrRecordNotFound
	}
	k := fakeOrderKey(in.OrgID, in.ID)
	if _, ok := f.orders[k]; !ok {
		return domain.WorkOrder{}, gorm.ErrRecordNotFound
	}
	f.orders[k] = in
	return in, nil
}

func (f *fakeRepo) SaveIntegrations(ctx context.Context, orgID, id uuid.UUID, quoteID, saleID *uuid.UUID, status *string) (domain.WorkOrder, error) {
	_ = ctx
	_ = orgID
	_ = id
	_ = quoteID
	_ = saleID
	_ = status
	return domain.WorkOrder{}, nil
}

func (f *fakeRepo) MarkReadyPickupNotified(ctx context.Context, orgID, id uuid.UUID, at time.Time) error {
	_ = ctx
	if f.orders == nil {
		return gorm.ErrRecordNotFound
	}
	k := fakeOrderKey(orgID, id)
	wo, ok := f.orders[k]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	wo.ReadyPickupNotifiedAt = &at
	f.orders[k] = wo
	return nil
}

func (f *fakeRepo) ListArchived(ctx context.Context, orgID uuid.UUID) ([]domain.WorkOrder, error) {
	_ = ctx
	_ = orgID
	return nil, nil
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
	product  map[string]any
}

func (f *fakeCP) GetCustomer(ctx context.Context, orgID, customerID string) (map[string]any, error) {
	_ = ctx
	_ = orgID
	_ = customerID
	return f.customer, nil
}

func (f *fakeCP) GetParty(ctx context.Context, orgID, partyID string) (map[string]any, error) {
	_ = ctx
	_ = orgID
	_ = partyID
	return f.party, nil
}

func (f *fakeCP) GetProduct(ctx context.Context, orgID, productID string) (map[string]any, error) {
	_ = ctx
	_ = orgID
	_ = productID
	return f.product, nil
}

func TestCreateEnrichesReferencesAndCalculatesTotals(t *testing.T) {
	repo := &fakeRepo{orders: make(map[string]domain.WorkOrder)}
	cp := &fakeCP{
		customer: map[string]any{"name": "Flota Perez"},
		product: map[string]any{
			"name":  "Filtro de aceite",
			"price": 12000.0,
		},
	}
	uc := NewUsecases(repo, nil, cp, nil)

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	customerID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	vehicleID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	productID := uuid.MustParse("00000000-0000-0000-0000-000000000004")

	out, err := uc.Create(context.Background(), domain.WorkOrder{
		OrgID:         orgID,
		VehicleID:     vehicleID,
		VehiclePlate:  "ab123cd",
		CustomerID:    &customerID,
		RequestedWork: "Service de 10.000 km",
		OpenedAt:      time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC),
		Items: []domain.WorkOrderItem{
			{
				ItemType:    "service",
				Description: "Mano de obra",
				Quantity:    1,
				UnitPrice:   30000,
				TaxRate:     21,
			},
			{
				ItemType:  "part",
				ProductID: &productID,
				Quantity:  1,
				TaxRate:   21,
			},
		},
	}, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if out.CustomerName != "Flota Perez" {
		t.Fatalf("Create().CustomerName = %q, want Flota Perez", out.CustomerName)
	}
	if out.Items[1].Description != "Filtro de aceite" {
		t.Fatalf("Create().Items[1].Description = %q, want enriched product name", out.Items[1].Description)
	}
	if out.Items[1].UnitPrice != 12000 {
		t.Fatalf("Create().Items[1].UnitPrice = %v, want 12000", out.Items[1].UnitPrice)
	}
	if out.SubtotalServices != 30000 {
		t.Fatalf("Create().SubtotalServices = %v, want 30000", out.SubtotalServices)
	}
	if out.SubtotalParts != 12000 {
		t.Fatalf("Create().SubtotalParts = %v, want 12000", out.SubtotalParts)
	}
	if out.Total != 50820 {
		t.Fatalf("Create().Total = %v, want 50820", out.Total)
	}
	if repo.created.Number == "" {
		t.Fatal("expected autogenerated work order number")
	}
}

func sampleWorkOrder(orgID, id uuid.UUID, status string) domain.WorkOrder {
	vehicleID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	return domain.WorkOrder{
		ID:           id,
		OrgID:        orgID,
		Number:       "OT-T",
		VehicleID:    vehicleID,
		VehiclePlate: "AB123CD",
		Status:       status,
		OpenedAt:     time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC),
		Currency:     "ARS",
		Items: []domain.WorkOrderItem{
			{ItemType: "service", Description: "Trabajo", Quantity: 1, UnitPrice: 100, TaxRate: 0},
		},
	}
}

func TestUpdateStatusTransitionAllowsPipeline(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	repo := &fakeRepo{orders: map[string]domain.WorkOrder{
		fakeOrderKey(orgID, id): sampleWorkOrder(orgID, id, "received"),
	}}
	uc := NewUsecases(repo, nil, nil, nil)
	ctx := context.Background()
	next := "diagnosing"
	if _, err := uc.Update(ctx, orgID, id, UpdateInput{Status: &next}, "tester"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	stored := repo.orders[fakeOrderKey(orgID, id)]
	if stored.Status != "diagnosing" {
		t.Fatalf("status = %q, want diagnosing", stored.Status)
	}
}

func TestUpdateStatusTransitionAllowsKanbanSkipAhead(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id := uuid.MustParse("00000000-0000-0000-0000-000000000098")
	repo := &fakeRepo{orders: map[string]domain.WorkOrder{
		fakeOrderKey(orgID, id): sampleWorkOrder(orgID, id, "received"),
	}}
	uc := NewUsecases(repo, nil, nil, nil)
	ctx := context.Background()
	next := "ready_for_pickup"
	if _, err := uc.Update(ctx, orgID, id, UpdateInput{Status: &next}, "tester"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	stored := repo.orders[fakeOrderKey(orgID, id)]
	if stored.Status != "ready_for_pickup" {
		t.Fatalf("status = %q, want ready_for_pickup", stored.Status)
	}
}

func TestUpdateStatusTransitionBlocksFromInvoiced(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id := uuid.MustParse("00000000-0000-0000-0000-000000000097")
	repo := &fakeRepo{orders: map[string]domain.WorkOrder{
		fakeOrderKey(orgID, id): sampleWorkOrder(orgID, id, "invoiced"),
	}}
	uc := NewUsecases(repo, nil, nil, nil)
	ctx := context.Background()
	next := "delivered"
	_, err := uc.Update(ctx, orgID, id, UpdateInput{Status: &next}, "tester")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, httperrors.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}
