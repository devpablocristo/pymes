package helpers

import (
	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
)

func BookingResponseFromDomain(value domain.Booking) repositorymodels.BookingResponse {
	allocations := make([]repositorymodels.AllocationResponse, 0, len(value.Allocations))
	for _, allocation := range value.Allocations {
		allocations = append(allocations, repositorymodels.AllocationResponse{
			ResourceID: allocation.ResourceID,
			Mode:       string(allocation.Mode),
			Units:      allocation.Units,
		})
	}
	return repositorymodels.BookingResponse{
		OrganizationID:     value.OrganizationID,
		ID:                 value.ID,
		SeriesID:           value.SeriesID,
		SessionID:          value.SessionID,
		SupersedesID:       value.SupersedesID,
		Occurrence:         value.Occurrence,
		BranchID:           value.BranchID,
		ServiceID:          value.ServiceID,
		PartyID:            value.PartyID,
		Status:             string(value.Status),
		SubstateCode:       value.SubstateCode,
		Participants:       value.Participants,
		StartAt:            value.StartAt,
		EndAt:              value.EndAt,
		OccupiesFrom:       value.OccupiesFrom,
		OccupiesUntil:      value.OccupiesUntil,
		HoldExpiresAt:      value.HoldExpiresAt,
		Version:            value.Version,
		ServiceName:        value.ServiceName,
		Price:              value.Price,
		Currency:           value.Currency,
		DurationMinutes:    value.DurationMinutes,
		Timezone:           value.Timezone,
		CustomerName:       value.CustomerName,
		CustomerEmail:      value.CustomerEmail,
		CustomerPhone:      value.CustomerPhone,
		MeetRequested:      value.MeetRequested,
		Notes:              value.Notes,
		CancellationReason: value.CancellationReason,
		Allocations:        allocations,
		CreatedBy:          value.CreatedBy,
		CreatedAt:          value.CreatedAt,
		UpdatedAt:          value.UpdatedAt,
	}
}

func BookingResponseToDomain(value repositorymodels.BookingResponse) domain.Booking {
	allocations := make([]domain.Allocation, 0, len(value.Allocations))
	for _, allocation := range value.Allocations {
		allocations = append(allocations, domain.Allocation{
			ResourceID: allocation.ResourceID,
			Mode:       domain.AllocationMode(allocation.Mode),
			Units:      allocation.Units,
		})
	}
	return domain.Booking{
		OrganizationID:     value.OrganizationID,
		ID:                 value.ID,
		SeriesID:           value.SeriesID,
		SessionID:          value.SessionID,
		SupersedesID:       value.SupersedesID,
		Occurrence:         value.Occurrence,
		BranchID:           value.BranchID,
		ServiceID:          value.ServiceID,
		PartyID:            value.PartyID,
		Status:             domain.BookingStatus(value.Status),
		SubstateCode:       value.SubstateCode,
		Participants:       value.Participants,
		StartAt:            value.StartAt,
		EndAt:              value.EndAt,
		OccupiesFrom:       value.OccupiesFrom,
		OccupiesUntil:      value.OccupiesUntil,
		HoldExpiresAt:      value.HoldExpiresAt,
		Version:            value.Version,
		ServiceName:        value.ServiceName,
		Price:              value.Price,
		Currency:           value.Currency,
		DurationMinutes:    value.DurationMinutes,
		Timezone:           value.Timezone,
		CustomerName:       value.CustomerName,
		CustomerEmail:      value.CustomerEmail,
		CustomerPhone:      value.CustomerPhone,
		MeetRequested:      value.MeetRequested,
		Notes:              value.Notes,
		CancellationReason: value.CancellationReason,
		Allocations:        allocations,
		CreatedBy:          value.CreatedBy,
		CreatedAt:          value.CreatedAt,
		UpdatedAt:          value.UpdatedAt,
	}
}
