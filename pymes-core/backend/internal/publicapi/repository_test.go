package publicapi

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
)

type fakeScheduling struct {
	branches        []schedulingdomain.Branch
	services        []schedulingdomain.Service
	resources       []schedulingdomain.Resource
	slots           []schedulingdomain.TimeSlot
	createInput     schedulingdomain.CreateBookingInput
	createInputSeen bool
}

func (f *fakeScheduling) ListBranches(_ context.Context, _ uuid.UUID) ([]schedulingdomain.Branch, error) {
	return f.branches, nil
}

func (f *fakeScheduling) ListServices(_ context.Context, _ uuid.UUID) ([]schedulingdomain.Service, error) {
	return f.services, nil
}

func (f *fakeScheduling) ListResources(_ context.Context, _ uuid.UUID, _ *uuid.UUID) ([]schedulingdomain.Resource, error) {
	return f.resources, nil
}

func (f *fakeScheduling) ListAvailableSlots(_ context.Context, _ uuid.UUID, _ schedulingdomain.SlotQuery) ([]schedulingdomain.TimeSlot, error) {
	return f.slots, nil
}

func (f *fakeScheduling) CreateBooking(_ context.Context, _ uuid.UUID, _ string, in schedulingdomain.CreateBookingInput) (schedulingdomain.Booking, error) {
	f.createInput = in
	f.createInputSeen = true
	endAt := in.StartAt.Add(30 * time.Minute)
	return schedulingdomain.Booking{
		ID:            uuid.New(),
		BranchID:      in.BranchID,
		ServiceID:     in.ServiceID,
		ResourceID:    derefUUID(in.ResourceID),
		CustomerName:  in.CustomerName,
		CustomerPhone: in.CustomerPhone,
		Status:        schedulingdomain.BookingStatusConfirmed,
		StartAt:       in.StartAt,
		EndAt:         endAt,
	}, nil
}

func TestRepositoryGetAvailabilityUsesSchedulingWhenConfigured(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	branchID := uuid.New()
	serviceID := uuid.New()
	repo := &Repository{
		scheduling: &fakeScheduling{
			branches: []schedulingdomain.Branch{{ID: branchID, OrgID: orgID, Name: "Central", Active: true}},
			services: []schedulingdomain.Service{{ID: serviceID, OrgID: orgID, Name: "Consulta", Active: true, FulfillmentMode: schedulingdomain.FulfillmentModeSchedule}},
			slots: []schedulingdomain.TimeSlot{{
				ResourceID: uuid.New(),
				StartAt:    time.Date(2026, 4, 7, 13, 0, 0, 0, time.UTC),
				EndAt:      time.Date(2026, 4, 7, 13, 30, 0, 0, time.UTC),
				Remaining:  1,
			}},
		},
	}

	slots, err := repo.GetAvailability(context.Background(), orgID, AvailabilityQuery{
		Date: time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("GetAvailability() error = %v", err)
	}
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(slots))
	}
	if !slots[0].StartAt.Equal(time.Date(2026, 4, 7, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected slot start %s", slots[0].StartAt)
	}
}

func TestRepositoryBookUsesSchedulingCompatibilityPayload(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	branchID := uuid.New()
	serviceID := uuid.New()
	resourceID := uuid.New()
	engine := &fakeScheduling{
		branches:  []schedulingdomain.Branch{{ID: branchID, OrgID: orgID, Name: "Central", Active: true}},
		services:  []schedulingdomain.Service{{ID: serviceID, OrgID: orgID, Name: "Consulta", Active: true, FulfillmentMode: schedulingdomain.FulfillmentModeSchedule}},
		resources: []schedulingdomain.Resource{{ID: resourceID, OrgID: orgID, BranchID: branchID, Name: "Profesional", Active: true}},
	}
	repo := &Repository{scheduling: engine}

	out, err := repo.Book(context.Background(), orgID, map[string]any{
		"party_name":  "Ana",
		"party_phone": "+54 381 5551234",
		"title":       "Consulta inicial",
		"start_at":    "2026-04-07T13:00:00Z",
		"resource_id": resourceID.String(),
	})
	if err != nil {
		t.Fatalf("Book() error = %v", err)
	}
	if !engine.createInputSeen {
		t.Fatalf("expected create booking to be called")
	}
	if engine.createInput.BranchID != branchID {
		t.Fatalf("expected branch_id %s, got %s", branchID, engine.createInput.BranchID)
	}
	if engine.createInput.ServiceID != serviceID {
		t.Fatalf("expected service_id %s, got %s", serviceID, engine.createInput.ServiceID)
	}
	if engine.createInput.ResourceID == nil || *engine.createInput.ResourceID != resourceID {
		t.Fatalf("expected resource_id %s, got %v", resourceID, engine.createInput.ResourceID)
	}
	if out.Title != "Consulta inicial" {
		t.Fatalf("expected title to keep compatibility payload, got %q", out.Title)
	}
}

func TestRepositoryResolveSchedulingSelectionRequiresExplicitIDsWhenAmbiguous(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	repo := &Repository{
		scheduling: &fakeScheduling{
			branches: []schedulingdomain.Branch{
				{ID: uuid.New(), OrgID: orgID, Name: "Central", Active: true},
				{ID: uuid.New(), OrgID: orgID, Name: "Norte", Active: true},
			},
			services: []schedulingdomain.Service{
				{ID: uuid.New(), OrgID: orgID, Name: "Consulta", Active: true, FulfillmentMode: schedulingdomain.FulfillmentModeSchedule},
			},
		},
	}

	_, _, err := repo.resolveSchedulingSelection(context.Background(), orgID, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected ambiguity error")
	}
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func derefUUID(v *uuid.UUID) uuid.UUID {
	if v == nil {
		return uuid.Nil
	}
	return *v
}
