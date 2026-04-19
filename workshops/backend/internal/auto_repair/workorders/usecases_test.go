package workorders

import (
	"context"
	"testing"

	"github.com/google/uuid"

	baseworkorders "github.com/devpablocristo/pymes/workshops/backend/internal/workorders"
	domain "github.com/devpablocristo/pymes/workshops/backend/internal/workorders/usecases/domain"
)

func TestListForcesVehicleTargetType(t *testing.T) {
	t.Parallel()

	base := &fakeBasePort{}
	uc := NewUsecases(base)

	_, _, _, _, err := uc.List(context.Background(), ListParams{OrgID: uuid.New(), TargetType: "bicycle"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if base.lastList.TargetType != targetType {
		t.Fatalf("List() target_type = %q, want %q", base.lastList.TargetType, targetType)
	}
}

func TestGetByIDRejectsOtherSubvertical(t *testing.T) {
	t.Parallel()

	base := &fakeBasePort{getByIDResult: WorkOrder{ID: uuid.New(), OrgID: uuid.New(), TargetType: "bicycle"}}
	uc := NewUsecases(base)

	_, err := uc.GetByID(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("GetByID() error = nil, want not found")
	}
}

func TestCreateForcesVehicleTargetType(t *testing.T) {
	t.Parallel()

	base := &fakeBasePort{}
	uc := NewUsecases(base)

	_, err := uc.Create(context.Background(), WorkOrder{OrgID: uuid.New(), TargetType: "bicycle"}, "tester")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if base.lastCreated.TargetType != targetType {
		t.Fatalf("Create() target_type = %q, want %q", base.lastCreated.TargetType, targetType)
	}
}

type fakeBasePort struct {
	lastList      baseworkorders.ListParams
	lastCreated   WorkOrder
	getByIDResult WorkOrder
}

func (f *fakeBasePort) List(_ context.Context, p baseworkorders.ListParams) ([]domain.WorkOrder, int64, bool, *uuid.UUID, error) {
	f.lastList = p
	return nil, 0, false, nil, nil
}

func (f *fakeBasePort) ListArchived(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ string) ([]domain.WorkOrder, error) {
	return nil, nil
}

func (f *fakeBasePort) Create(_ context.Context, in domain.WorkOrder, _ string) (domain.WorkOrder, error) {
	f.lastCreated = in
	return in, nil
}

func (f *fakeBasePort) GetByID(_ context.Context, _, _ uuid.UUID) (domain.WorkOrder, error) {
	return f.getByIDResult, nil
}

func (f *fakeBasePort) Update(_ context.Context, _, _ uuid.UUID, _ baseworkorders.UpdateInput, _ string) (domain.WorkOrder, error) {
	return f.getByIDResult, nil
}

func (f *fakeBasePort) SaveIntegrations(_ context.Context, _, _ uuid.UUID, _, _ *uuid.UUID, _ *string, _ string) (domain.WorkOrder, error) {
	return f.getByIDResult, nil
}

func (f *fakeBasePort) SoftDelete(_ context.Context, _, _ uuid.UUID, _ string) error { return nil }
func (f *fakeBasePort) Restore(_ context.Context, _, _ uuid.UUID, _ string) error    { return nil }
func (f *fakeBasePort) HardDelete(_ context.Context, _, _ uuid.UUID, _ string) error { return nil }
