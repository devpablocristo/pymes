package scheduling

import (
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
)

func TestPlatformSchedulingIntersectsMultipleResourcesBuffersAndBlocks(t *testing.T) {
	branchID, serviceID := uuid.New(), uuid.New()
	professionalID, roomID := uuid.New(), uuid.New()
	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, time.August, 3, 0, 0, 0, 0, location) // Monday
	snapshot := domain.AvailabilitySnapshot{
		Branch: domain.Branch{
			ID: branchID, Timezone: location.String(), Active: true,
		},
		Service: domain.Service{
			ID: serviceID, DurationMinutes: 60, BufferBeforeMinutes: 15,
			BufferAfterMinutes: 15, SlotMinutes: 30, MaxParticipants: 1,
		},
		Resources: []domain.Resource{
			{ID: professionalID, BranchID: branchID, Name: "Ana", Capacity: 1, Active: true},
			{ID: roomID, BranchID: branchID, Name: "Sala", Capacity: 1, Active: true},
		},
		Rules: []domain.AvailabilityRule{
			{BranchID: branchID, Kind: domain.AvailabilityBranch, Weekday: time.Monday, StartMinute: 9 * 60, EndMinute: 17 * 60, Active: true},
			{BranchID: branchID, ResourceID: &professionalID, Kind: domain.AvailabilityResource, Weekday: time.Monday, StartMinute: 10 * 60, EndMinute: 16 * 60, Active: true},
			{BranchID: branchID, ResourceID: &roomID, Kind: domain.AvailabilityResource, Weekday: time.Monday, StartMinute: 9 * 60, EndMinute: 15 * 60, Active: true},
		},
		Exceptions: []domain.AvailabilityException{
			{
				ResourceID: &roomID, Kind: domain.ExceptionManualBlock,
				StartAt: time.Date(2026, time.August, 3, 11, 0, 0, 0, location),
				EndAt:   time.Date(2026, time.August, 3, 12, 0, 0, 0, location),
			},
		},
	}
	adapter := NewPlatformScheduling()
	slots, err := adapter.CalculateSlots(domain.AvailabilityQuery{
		BranchID: branchID, ServiceID: serviceID,
		From: day, Until: day.AddDate(0, 0, 1), Participants: 1,
		Allocations: []domain.Allocation{
			{ResourceID: professionalID, Mode: domain.AllocationExclusive, Units: 1},
			{ResourceID: roomID, Mode: domain.AllocationExclusive, Units: 1},
		},
	}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) == 0 {
		t.Fatal("expected composite slots")
	}
	for _, slot := range slots {
		local := slot.StartAt.In(location)
		if local.Hour() < 10 || local.Hour() >= 14 {
			t.Fatalf("slot escaped branch/resource intersection: %s", local)
		}
		if slot.OccupiesFrom.After(slot.StartAt) ||
			slot.OccupiesUntil.Before(slot.EndAt) ||
			len(slot.Allocations) != 2 {
			t.Fatalf("slot lost buffers/allocations: %+v", slot)
		}
		if slot.OccupiesFrom.Before(snapshot.Exceptions[0].EndAt) &&
			slot.OccupiesUntil.After(snapshot.Exceptions[0].StartAt) {
			t.Fatalf("slot overlaps block: %+v", slot)
		}
	}
}

func TestPlatformSchedulingSupportsCapacityAndCrossMidnight(t *testing.T) {
	branchID, serviceID, resourceID := uuid.New(), uuid.New(), uuid.New()
	location := time.UTC
	day := time.Date(2026, time.August, 7, 0, 0, 0, 0, location) // Friday
	occupiedID := uuid.New()
	snapshot := domain.AvailabilitySnapshot{
		Branch:    domain.Branch{ID: branchID, Timezone: "UTC", Active: true},
		Service:   domain.Service{ID: serviceID, DurationMinutes: 60, SlotMinutes: 30, MaxParticipants: 10},
		Resources: []domain.Resource{{ID: resourceID, BranchID: branchID, Name: "Vehicle", Capacity: 4, Active: true}},
		Rules: []domain.AvailabilityRule{{
			BranchID: branchID, Kind: domain.AvailabilityBranch, Weekday: time.Friday,
			StartMinute: 23 * 60, EndMinute: 60, Active: true,
		}},
		Occupancies: []domain.Occupancy{{
			ResourceID: resourceID, StartAt: day.Add(23 * time.Hour), EndAt: day.Add(24 * time.Hour),
			Units: 2, BookingID: occupiedID, ServiceID: serviceID, BookingOpen: true,
		}},
	}
	slots, err := NewPlatformScheduling().CalculateSlots(domain.AvailabilityQuery{
		BranchID: branchID, ServiceID: serviceID, From: day, Until: day.Add(26 * time.Hour),
		Participants: 2,
		Allocations: []domain.Allocation{{
			ResourceID: resourceID, Mode: domain.AllocationCapacity, Units: 0,
		}},
	}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) == 0 {
		t.Fatal("expected capacity slots")
	}
	foundAfterMidnight := false
	for _, slot := range slots {
		if slot.EndAt.Day() != slot.StartAt.Day() {
			foundAfterMidnight = true
		}
		if slot.StartAt.Equal(day.Add(23*time.Hour)) && slot.Remaining != 1 {
			t.Fatalf("remaining reservations=%d, want 1", slot.Remaining)
		}
	}
	if !foundAfterMidnight {
		t.Fatal("cross-midnight slot was not produced")
	}
}
