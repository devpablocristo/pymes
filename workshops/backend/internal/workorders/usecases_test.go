package workorders

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

// fakeRepo es un stub in-memory para tests del módulo base.
// (En producción la implementación es GORM contra workshops.work_orders.)
type fakeRepo struct {
	store map[uuid.UUID]domain.WorkOrder
}

func newFakeRepo() *fakeRepo { return &fakeRepo{store: map[uuid.UUID]domain.WorkOrder{}} }

func (r *fakeRepo) List(_ context.Context, p ListParams) ([]domain.WorkOrder, int64, bool, *uuid.UUID, error) {
	out := make([]domain.WorkOrder, 0, len(r.store))
	for _, wo := range r.store {
		if wo.OrgID != p.OrgID || wo.ArchivedAt != nil {
			continue
		}
		if p.TargetType != "" && wo.TargetType != p.TargetType {
			continue
		}
		out = append(out, wo)
	}
	return out, int64(len(out)), false, nil, nil
}
func (r *fakeRepo) ListArchived(_ context.Context, orgID uuid.UUID, targetType string) ([]domain.WorkOrder, error) {
	out := make([]domain.WorkOrder, 0)
	for _, wo := range r.store {
		if wo.OrgID != orgID || wo.ArchivedAt == nil {
			continue
		}
		if targetType != "" && wo.TargetType != targetType {
			continue
		}
		out = append(out, wo)
	}
	return out, nil
}
func (r *fakeRepo) Create(_ context.Context, in domain.WorkOrder) (domain.WorkOrder, error) {
	in.ID = uuid.New()
	in.CreatedAt = time.Now().UTC()
	in.UpdatedAt = time.Now().UTC()
	r.store[in.ID] = in
	return in, nil
}
func (r *fakeRepo) GetByID(_ context.Context, orgID, id uuid.UUID) (domain.WorkOrder, error) {
	wo, ok := r.store[id]
	if !ok || wo.OrgID != orgID {
		return domain.WorkOrder{}, gorm.ErrRecordNotFound
	}
	return wo, nil
}
func (r *fakeRepo) Update(_ context.Context, in domain.WorkOrder) (domain.WorkOrder, error) {
	if _, ok := r.store[in.ID]; !ok {
		return domain.WorkOrder{}, gorm.ErrRecordNotFound
	}
	in.UpdatedAt = time.Now().UTC()
	r.store[in.ID] = in
	return in, nil
}
func (r *fakeRepo) SaveIntegrations(_ context.Context, _, id uuid.UUID, q, s *uuid.UUID, st *string) (domain.WorkOrder, error) {
	wo, ok := r.store[id]
	if !ok {
		return domain.WorkOrder{}, gorm.ErrRecordNotFound
	}
	if q != nil {
		wo.QuoteID = q
	}
	if s != nil {
		wo.SaleID = s
	}
	if st != nil {
		wo.Status = *st
	}
	r.store[id] = wo
	return wo, nil
}
func (r *fakeRepo) SoftDelete(_ context.Context, _, id uuid.UUID) error {
	wo, ok := r.store[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	now := time.Now().UTC()
	wo.ArchivedAt = &now
	r.store[id] = wo
	return nil
}
func (r *fakeRepo) Restore(_ context.Context, _, id uuid.UUID) error {
	wo, ok := r.store[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	wo.ArchivedAt = nil
	r.store[id] = wo
	return nil
}
func (r *fakeRepo) HardDelete(_ context.Context, _, id uuid.UUID) error {
	if _, ok := r.store[id]; !ok {
		return gorm.ErrRecordNotFound
	}
	delete(r.store, id)
	return nil
}

func TestCreateWithVehicleTarget(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), nil, nil, nil, NewNoopHook("vehicle"))

	in := domain.WorkOrder{
		OrgID:       uuid.New(),
		Number:      "OT-001",
		TargetType:  "vehicle",
		TargetID:    uuid.New(),
		TargetLabel: "AB 123 CD",
		Status:      "received",
		Items: []domain.WorkOrderItem{{
			ItemType:    "service",
			Description: "Cambio de aceite",
			Quantity:    1,
			UnitPrice:   10000,
			TaxRate:     21,
		}},
	}
	out, err := uc.Create(context.Background(), in, "tester")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if out.TargetType != "vehicle" {
		t.Errorf("target_type = %q, want vehicle", out.TargetType)
	}
	if out.TargetLabel != "AB 123 CD" {
		t.Errorf("target_label = %q", out.TargetLabel)
	}
	if out.Total != 12100 { // 10000 * 1 + 21% IVA
		t.Errorf("total = %v, want 12100", out.Total)
	}
}

func TestCreateRequiresTargetType(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(newFakeRepo(), nil, nil, nil)
	_, err := uc.Create(context.Background(), domain.WorkOrder{
		OrgID:    uuid.New(),
		Number:   "OT-002",
		TargetID: uuid.New(),
		Status:   "received",
		Items: []domain.WorkOrderItem{{
			ItemType:    "service",
			Description: "x",
			Quantity:    1,
			UnitPrice:   1,
			TaxRate:     0,
		}},
	}, "tester")
	if err == nil {
		t.Fatal("expected target_type required error")
	}
}

func TestListFiltersByTargetType(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	uc := NewUsecases(repo, nil, nil, nil)
	orgID := uuid.New()
	for _, tt := range []struct{ targetType string }{{"vehicle"}, {"vehicle"}, {"bicycle"}} {
		_, err := uc.Create(context.Background(), domain.WorkOrder{
			OrgID:       orgID,
			Number:      "OT-" + tt.targetType,
			TargetType:  tt.targetType,
			TargetID:    uuid.New(),
			TargetLabel: "x",
			Status:      "received",
			Items: []domain.WorkOrderItem{{
				ItemType: "service", Description: "x", Quantity: 1, UnitPrice: 1, TaxRate: 0,
			}},
		}, "tester")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	got, _, _, _, err := uc.List(context.Background(), ListParams{OrgID: orgID, TargetType: "vehicle"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 vehicle, got %d", len(got))
	}
}
