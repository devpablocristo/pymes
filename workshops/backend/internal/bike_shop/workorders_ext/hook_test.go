package workordersext

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	bicyclesdomain "github.com/devpablocristo/pymes/workshops/backend/internal/bike_shop/bicycles/usecases/domain"
	workordersdomain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

type fakeBicycleLookup struct {
	bicycle bicyclesdomain.Bicycle
	err     error
}

func (f *fakeBicycleLookup) GetByID(ctx context.Context, orgID, id uuid.UUID) (bicyclesdomain.Bicycle, error) {
	_ = ctx
	_ = orgID
	_ = id
	if f.err != nil {
		return bicyclesdomain.Bicycle{}, f.err
	}
	return f.bicycle, nil
}

func TestHookBeforeCreateSyncsBicycleData(t *testing.T) {
	customerID := uuid.New()
	hook := New(&fakeBicycleLookup{
		bicycle: bicyclesdomain.Bicycle{
			ID:           uuid.New(),
			CustomerID:   &customerID,
			CustomerName: "Cliente Bici",
			Brand:        "Trek",
			Model:        "Marlin 7",
		},
	})
	wo := &workordersdomain.WorkOrder{
		OrgID:      uuid.New(),
		TargetID:   uuid.New(),
		TargetType: "bicycle",
	}

	if err := hook.BeforeCreate(context.Background(), wo); err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}
	if wo.TargetLabel != "Trek Marlin 7" {
		t.Fatalf("TargetLabel = %q, want Trek Marlin 7", wo.TargetLabel)
	}
	if wo.CustomerName != "Cliente Bici" {
		t.Fatalf("CustomerName = %q, want Cliente Bici", wo.CustomerName)
	}
	if wo.CustomerID == nil || *wo.CustomerID != customerID {
		t.Fatalf("CustomerID = %v, want %v", wo.CustomerID, customerID)
	}
}

func TestHookBeforeCreateRejectsUnknownBicycle(t *testing.T) {
	hook := New(&fakeBicycleLookup{err: httperrors.ErrNotFound})
	wo := &workordersdomain.WorkOrder{
		OrgID:      uuid.New(),
		TargetID:   uuid.New(),
		TargetType: "bicycle",
	}

	err := hook.BeforeCreate(context.Background(), wo)
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("BeforeCreate() error = %v, want ErrBadInput", err)
	}
}
