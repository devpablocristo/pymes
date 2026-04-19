package workordersext

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	httperrors "github.com/devpablocristo/pymes/pymes-core/shared/backend/httperrors"
	vehiclesdomain "github.com/devpablocristo/pymes/workshops/backend/internal/auto_repair/vehicles/usecases/domain"
	workordersdomain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

type fakeVehicleLookup struct {
	vehicle vehiclesdomain.Vehicle
	err     error
}

func (f *fakeVehicleLookup) GetByID(ctx context.Context, orgID, id uuid.UUID) (vehiclesdomain.Vehicle, error) {
	_ = ctx
	_ = orgID
	_ = id
	if f.err != nil {
		return vehiclesdomain.Vehicle{}, f.err
	}
	return f.vehicle, nil
}

func TestHookBeforeCreateSyncsVehicleData(t *testing.T) {
	customerID := uuid.New()
	hook := New(&fakeVehicleLookup{
		vehicle: vehiclesdomain.Vehicle{
			ID:           uuid.New(),
			CustomerID:   &customerID,
			CustomerName: "Cliente Auto",
			LicensePlate: "AB123CD",
		},
	})
	wo := &workordersdomain.WorkOrder{
		OrgID:      uuid.New(),
		TargetID:   uuid.New(),
		TargetType: "vehicle",
	}

	if err := hook.BeforeCreate(context.Background(), wo); err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}
	if wo.TargetLabel != "AB123CD" {
		t.Fatalf("TargetLabel = %q, want AB123CD", wo.TargetLabel)
	}
	if wo.CustomerName != "Cliente Auto" {
		t.Fatalf("CustomerName = %q, want Cliente Auto", wo.CustomerName)
	}
	if wo.CustomerID == nil || *wo.CustomerID != customerID {
		t.Fatalf("CustomerID = %v, want %v", wo.CustomerID, customerID)
	}
}

func TestHookBeforeCreateRejectsUnknownVehicle(t *testing.T) {
	hook := New(&fakeVehicleLookup{err: httperrors.ErrNotFound})
	wo := &workordersdomain.WorkOrder{
		OrgID:      uuid.New(),
		TargetID:   uuid.New(),
		TargetType: "vehicle",
	}

	err := hook.BeforeCreate(context.Background(), wo)
	if !errors.Is(err, httperrors.ErrBadInput) {
		t.Fatalf("BeforeCreate() error = %v, want ErrBadInput", err)
	}
}
