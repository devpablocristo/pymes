// architecture:adapter external
package scheduling

import (
	"fmt"
	"sort"
	"time"

	platform "github.com/devpablocristo/platform/features/scheduling/go"
	platformdomain "github.com/devpablocristo/platform/features/scheduling/go/domain"
	platformhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/platform_scheduling/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
)

// PlatformScheduling is an anti-corruption adapter. No Platform-owned type is
// exposed by its methods.
type PlatformScheduling struct{}

func NewPlatformScheduling() *PlatformScheduling { return &PlatformScheduling{} }

func (p *PlatformScheduling) NormalizeAllocations(
	allocations []domain.Allocation,
	participants int,
) ([]domain.Allocation, error) {
	providerValues, err := platform.NormalizeResourceAllocationsForParticipants(
		nil,
		platformhelpers.ToPlatformAllocations(allocations),
		participants,
	)
	if err != nil {
		return nil, domain.WrapError(domain.CodeValidation, "invalid resource allocations", err)
	}
	return platformhelpers.FromPlatformAllocations(providerValues), nil
}

func (p *PlatformScheduling) CalculateSlots(
	query domain.AvailabilityQuery,
	snapshot domain.AvailabilitySnapshot,
) ([]domain.Slot, error) {
	if err := domain.ValidateRange(query.From, query.Until); err != nil {
		return nil, err
	}
	if query.Participants <= 0 {
		return nil, domain.NewError(domain.CodeValidation, "participants must be positive")
	}
	allocations, err := p.NormalizeAllocations(query.Allocations, query.Participants)
	if err != nil {
		return nil, err
	}
	if len(allocations) == 0 {
		return nil, domain.NewError(domain.CodeValidation, "resource allocations are required")
	}
	location, err := domain.EnsureTimezone(snapshot.Branch.Timezone)
	if err != nil {
		return nil, domain.NewError(domain.CodeValidation, "branch timezone is invalid")
	}
	resourceByID := make(map[uuid.UUID]domain.Resource, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		resourceByID[resource.ID] = resource
	}
	resolved := make([]domain.Allocation, 0, len(allocations))
	slotSets := make(map[uuid.UUID][]platformdomain.TimeSlot, len(allocations))
	for _, allocation := range allocations {
		resource, ok := resourceByID[allocation.ResourceID]
		if !ok || !resource.Active || resource.BranchID != query.BranchID {
			return nil, domain.NewError(domain.CodeValidation, "requested resource is unavailable")
		}
		providerAllocation, err := platform.ResolveAllocationUnits(
			platformhelpers.ToPlatformAllocations([]domain.Allocation{allocation})[0],
			resource.Capacity,
		)
		if err != nil {
			return nil, domain.WrapError(domain.CodeCapacityExceeded, "requested resource capacity is unavailable", err)
		}
		allocation = platformhelpers.FromPlatformAllocations([]platformdomain.ResourceAllocation{providerAllocation})[0]
		resolved = append(resolved, allocation)
		slotSets[resource.ID] = p.resourceSlots(query, snapshot, resource, allocation, location)
	}
	providerSlots := platform.IntersectResourceSlots(
		platformhelpers.ToPlatformAllocations(resolved),
		slotSets,
	)
	result := make([]domain.Slot, 0, len(providerSlots))
	for _, slot := range providerSlots {
		result = append(result, domain.Slot{
			StartAt:          slot.StartAt.UTC(),
			EndAt:            slot.EndAt.UTC(),
			OccupiesFrom:     slot.OccupiesFrom.UTC(),
			OccupiesUntil:    slot.OccupiesUntil.UTC(),
			Timezone:         snapshot.Branch.Timezone,
			Allocations:      platformhelpers.FromPlatformAllocations(slot.Allocations),
			Remaining:        slot.Remaining,
			ServiceRemaining: slot.ServiceRemaining,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StartAt.Before(result[j].StartAt) })
	return deduplicateSlots(result), nil
}

func (p *PlatformScheduling) resourceSlots(
	query domain.AvailabilityQuery,
	snapshot domain.AvailabilitySnapshot,
	resource domain.Resource,
	allocation domain.Allocation,
	location *time.Location,
) []platformdomain.TimeSlot {
	duration := time.Duration(snapshot.Service.DurationMinutes) * time.Minute
	before := time.Duration(snapshot.Service.BufferBeforeMinutes) * time.Minute
	after := time.Duration(snapshot.Service.BufferAfterMinutes) * time.Minute
	granularity := time.Duration(snapshot.Service.SlotMinutes) * time.Minute
	if granularity <= 0 {
		granularity = 15 * time.Minute
	}
	fromLocal := query.From.In(location)
	untilLocal := query.Until.In(location)
	day := time.Date(fromLocal.Year(), fromLocal.Month(), fromLocal.Day(), 0, 0, 0, 0, location)
	lastDay := time.Date(untilLocal.Year(), untilLocal.Month(), untilLocal.Day(), 0, 0, 0, 0, location)
	result := make([]platformdomain.TimeSlot, 0)
	for ; !day.After(lastDay); day = day.AddDate(0, 0, 1) {
		windows := platformhelpers.WindowsFor(day, resource.ID, snapshot.Rules, snapshot.Exceptions, location)
		for _, window := range windows {
			for startAt := window.StartAt; !startAt.Add(duration).After(window.EndAt); startAt = startAt.Add(granularity) {
				endAt := startAt.Add(duration)
				occupiesFrom, occupiesUntil := startAt.Add(-before), endAt.Add(after)
				if startAt.Before(query.From) || !startAt.Before(query.Until) ||
					occupiesFrom.Before(window.StartAt) || occupiesUntil.After(window.EndAt) ||
					platformhelpers.Blocked(occupiesFrom, occupiesUntil, resource.ID, snapshot.Exceptions) {
					continue
				}
				allocated, concurrent := occupancy(snapshot.Occupancies, query, resource.ID, occupiesFrom, occupiesUntil)
				if allocation.Mode == domain.AllocationExclusive && allocated > 0 {
					continue
				}
				remainingUnits := resource.Capacity - allocated
				// Per-booking participant limits are validated by the use case;
				// ordinary concurrent capacity is owned by resources. Group
				// capacity is protected transactionally by group sessions.
				serviceRemaining := 100_000 - concurrent
				if remainingUnits < allocation.Units || serviceRemaining < query.Participants {
					continue
				}
				result = append(result, platformdomain.TimeSlot{
					ResourceID:       resource.ID,
					ResourceName:     resource.Name,
					StartAt:          startAt.UTC(),
					EndAt:            endAt.UTC(),
					OccupiesFrom:     occupiesFrom.UTC(),
					OccupiesUntil:    occupiesUntil.UTC(),
					Timezone:         snapshot.Branch.Timezone,
					Remaining:        remainingUnits,
					ServiceRemaining: serviceRemaining,
					AllocatedUnits:   allocated,
					GranularityMin:   int(granularity / time.Minute),
				})
			}
		}
	}
	return result
}

func occupancy(
	values []domain.Occupancy,
	query domain.AvailabilityQuery,
	resourceID uuid.UUID,
	startAt, endAt time.Time,
) (int, int) {
	allocated := 0
	bookings := make(map[uuid.UUID]struct{})
	for _, value := range values {
		if !value.BookingOpen || value.ResourceID != resourceID ||
			(query.ExcludeBookingID != nil && value.BookingID == *query.ExcludeBookingID) ||
			!startAt.Before(value.EndAt) || !endAt.After(value.StartAt) {
			continue
		}
		allocated += value.Units
		if value.ServiceID == query.ServiceID {
			bookings[value.BookingID] = struct{}{}
		}
	}
	return allocated, len(bookings)
}

func deduplicateSlots(values []domain.Slot) []domain.Slot {
	result := make([]domain.Slot, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := fmt.Sprintf("%d:%d", value.StartAt.UnixNano(), value.EndAt.UnixNano())
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}
