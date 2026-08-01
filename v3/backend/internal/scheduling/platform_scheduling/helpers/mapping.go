package helpers

import (
	platformdomain "github.com/devpablocristo/platform/features/scheduling/go/domain"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
)

func ToPlatformAllocations(values []domain.Allocation) []platformdomain.ResourceAllocation {
	result := make([]platformdomain.ResourceAllocation, 0, len(values))
	for _, value := range values {
		result = append(result, platformdomain.ResourceAllocation{
			ResourceID: value.ResourceID,
			Mode:       platformdomain.ResourceAllocationMode(value.Mode),
			Units:      value.Units,
		})
	}
	return result
}

func FromPlatformAllocations(values []platformdomain.ResourceAllocation) []domain.Allocation {
	result := make([]domain.Allocation, 0, len(values))
	for _, value := range values {
		result = append(result, domain.Allocation{
			ResourceID: value.ResourceID,
			Mode:       domain.AllocationMode(value.Mode),
			Units:      value.Units,
		})
	}
	return result
}
