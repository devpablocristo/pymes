package scheduling

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
)

type createBookingRepositoryFake struct {
	Repository
	branch       domain.Branch
	service      domain.Service
	requirements []domain.ResourceRequirement
	resources    []domain.Resource
	snapshot     domain.AvailabilitySnapshot
	series       *domain.RecurrenceSeries
	bookings     []domain.Booking
	actionTokens []domain.ActionToken
	events       []domain.Event
}

func (f *createBookingRepositoryFake) GetBranch(context.Context, string, uuid.UUID) (domain.Branch, error) {
	return f.branch, nil
}

func (f *createBookingRepositoryFake) GetService(
	context.Context,
	string,
	uuid.UUID,
) (domain.Service, []domain.ResourceRequirement, error) {
	return f.service, f.requirements, nil
}

func (f *createBookingRepositoryFake) GetResources(context.Context, string, []uuid.UUID) ([]domain.Resource, error) {
	return f.resources, nil
}

func (f *createBookingRepositoryFake) ListResources(context.Context, string, uuid.UUID) ([]domain.Resource, error) {
	return f.resources, nil
}

func (f *createBookingRepositoryFake) LoadAvailability(
	context.Context,
	domain.AvailabilityQuery,
) (domain.AvailabilitySnapshot, error) {
	return f.snapshot, nil
}

func (f *createBookingRepositoryFake) ReserveBookings(
	_ context.Context,
	_ domain.CommandMetadata,
	series *domain.RecurrenceSeries,
	bookings []domain.Booking,
	actionTokens []domain.ActionToken,
	events []domain.Event,
) ([]domain.Booking, error) {
	f.series, f.bookings, f.actionTokens, f.events = series, bookings, actionTokens, events
	return bookings, nil
}

type algorithmsFake struct {
	slots []domain.Slot
}

func (f algorithmsFake) NormalizeAllocations(values []domain.Allocation, _ int) ([]domain.Allocation, error) {
	return values, nil
}

func (f algorithmsFake) CalculateSlots(
	query domain.AvailabilityQuery,
	_ domain.AvailabilitySnapshot,
) ([]domain.Slot, error) {
	for _, slot := range f.slots {
		if slot.StartAt.Equal(query.From.UTC()) {
			return []domain.Slot{slot}, nil
		}
	}
	return []domain.Slot{}, nil
}

type partyDirectoryFake struct {
	partyID string
	calls   int
}

func (f *partyDirectoryFake) EnsureCustomer(context.Context, domain.CommandMetadata, PublicCustomer) (string, error) {
	f.calls++
	return f.partyID, nil
}

func TestCreateRecurringBookingFreezesSnapshotsAndQueuesIntegrationEvents(t *testing.T) {
	organizationID := "org_booking"
	branchID, serviceID, resourceID := uuid.New(), uuid.New(), uuid.New()
	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		t.Fatal(err)
	}
	startAt := time.Date(2026, time.August, 3, 10, 0, 0, 0, location).UTC()
	secondStart := time.Date(2026, time.August, 4, 10, 0, 0, 0, location).UTC()
	allocation := domain.Allocation{ResourceID: resourceID, Mode: domain.AllocationExclusive, Units: 1}
	slots := []domain.Slot{
		{
			StartAt: startAt, EndAt: startAt.Add(45 * time.Minute),
			OccupiesFrom: startAt.Add(-10 * time.Minute), OccupiesUntil: startAt.Add(50 * time.Minute),
			Timezone: location.String(), Allocations: []domain.Allocation{allocation}, Remaining: 1,
		},
		{
			StartAt: secondStart, EndAt: secondStart.Add(45 * time.Minute),
			OccupiesFrom: secondStart.Add(-10 * time.Minute), OccupiesUntil: secondStart.Add(50 * time.Minute),
			Timezone: location.String(), Allocations: []domain.Allocation{allocation}, Remaining: 1,
		},
	}
	repository := &createBookingRepositoryFake{
		branch: domain.Branch{
			OrganizationID: organizationID, ID: branchID, Timezone: location.String(), Active: true,
		},
		service: domain.Service{
			OrganizationID: organizationID, ID: serviceID, Name: "Consulta",
			DurationMinutes: 45, BufferBeforeMinutes: 10, BufferAfterMinutes: 5,
			SlotMinutes: 15, Price: "1234.50", Currency: "ARS",
			Mode: domain.FulfillmentInPerson, MaxParticipants: 1, Active: true,
		},
		requirements: []domain.ResourceRequirement{{
			ResourceID: &resourceID, Mode: domain.AllocationExclusive, Units: 1,
		}},
		resources: []domain.Resource{{
			OrganizationID: organizationID, ID: resourceID, BranchID: branchID,
			Kind: domain.ResourceProfessional, Capacity: 1, Active: true,
		}},
		snapshot: domain.AvailabilitySnapshot{},
	}
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	codec, err := NewHMACActionTokenCodec([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(
		repository,
		algorithmsFake{slots: slots},
		codec,
		WithPartyDirectory(&partyDirectoryFake{partyID: "party-1"}),
		WithClock(func() time.Time { return now }),
	)
	metadata := testMetadata(organizationID, "recurrence-create", "source-recurrence")
	result, err := service.CreateBooking(context.Background(), metadata, CreateBookingInput{
		OrganizationID: organizationID, BranchID: branchID, ServiceID: serviceID,
		Customer: PublicCustomer{Name: "Ada", Email: "ada@example.com"},
		StartAt:  startAt, Participants: 1, Allocations: []domain.Allocation{allocation},
		Recurrence: &domain.RecurrenceRule{
			Frequency: domain.RecurrenceDaily, Interval: 1, Count: 2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || repository.series == nil || len(repository.events) != 6 ||
		len(repository.actionTokens) != 4 {
		t.Fatalf(
			"bookings=%d series=%+v events=%d action_tokens=%d",
			len(result), repository.series, len(repository.events), len(repository.actionTokens),
		)
	}
	for index, booking := range result {
		if booking.ServiceName != "Consulta" || booking.Price != "1234.50" ||
			booking.Currency != "ARS" || booking.DurationMinutes != 45 ||
			booking.Timezone != location.String() || booking.PartyID != "party-1" ||
			booking.SeriesID == nil || booking.Occurrence != index {
			t.Fatalf("snapshot %d not frozen: %+v", index, booking)
		}
	}
	eventTypes := map[string]int{}
	for _, event := range repository.events {
		eventTypes[event.Type]++
		if event.OrganizationID != organizationID || event.PayloadHash == "" ||
			event.RequestID != metadata.RequestID || event.CorrelationID != metadata.CorrelationID {
			t.Fatalf("event metadata incomplete: %+v", event)
		}
	}
	for _, event := range repository.events {
		if event.Type != domain.EventNotificationRequested {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		actions, ok := payload["action_tokens"].(map[string]any)
		if !ok || actions[string(domain.ActionCancel)] == "" ||
			actions[string(domain.ActionReschedule)] == "" {
			t.Fatalf("notification action tokens are missing: %+v", payload)
		}
	}
	for _, eventType := range []string{
		domain.EventBookingCreated,
		domain.EventCalendarSyncRequested,
		domain.EventNotificationRequested,
	} {
		if eventTypes[eventType] != 2 {
			t.Fatalf("event %s count=%d", eventType, eventTypes[eventType])
		}
	}
}

func TestAllocationCandidatesFillOptionalProfessionalSelectionWithRequiredRoom(t *testing.T) {
	professionalA, professionalB, room := uuid.New(), uuid.New(), uuid.New()
	requirements := []domain.ResourceRequirement{
		{Kind: domain.ResourceProfessional, Mode: domain.AllocationExclusive, Units: 1},
		{ResourceID: &room, Kind: domain.ResourceRoom, Mode: domain.AllocationExclusive, Units: 1},
	}
	resources := []domain.Resource{
		{ID: professionalB, Kind: domain.ResourceProfessional, Capacity: 1, Active: true},
		{ID: room, Kind: domain.ResourceRoom, Capacity: 1, Active: true},
		{ID: professionalA, Kind: domain.ResourceProfessional, Capacity: 1, Active: true},
	}
	candidates, err := allocationCandidates(requirements, resources, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidate combinations=%d want=2: %+v", len(candidates), candidates)
	}
	for _, candidate := range candidates {
		if len(candidate) != 2 || !containsAllocation(candidate, room) {
			t.Fatalf("candidate does not atomically include professional and room: %+v", candidate)
		}
	}
}

func TestCreateWaitlistEnsuresCustomerThroughConsumerOwnedPort(t *testing.T) {
	repository := &waitlistRepositoryFake{}
	parties := &partyDirectoryFake{partyID: "party-public"}
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	service := NewService(
		repository,
		algorithmsFake{},
		nil,
		WithPartyDirectory(parties),
		WithClock(func() time.Time { return now }),
	)
	organizationID := "org-waitlist"
	metadata := testMetadata(organizationID, "waitlist-customer", "waitlist-source")
	result, err := service.CreateWaitlistEntry(context.Background(), metadata, CreateWaitlistInput{
		OrganizationID: organizationID,
		ID:             uuid.New(),
		BranchID:       uuid.New(),
		ServiceID:      uuid.New(),
		Customer:       PublicCustomer{Name: "Ada", Email: "ada@example.com"},
		PreferredFrom:  now.Add(time.Hour),
		PreferredUntil: now.Add(2 * time.Hour),
		Participants:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PartyID != "party-public" || parties.calls != 1 {
		t.Fatalf("waitlist customer was not resolved exactly once: result=%+v calls=%d", result, parties.calls)
	}
}

type waitlistRepositoryFake struct {
	Repository
}

func (f *waitlistRepositoryFake) CreateWaitlistEntry(
	_ context.Context,
	_ domain.CommandMetadata,
	value domain.WaitlistEntry,
	_ domain.Event,
) (domain.WaitlistEntry, error) {
	return value, nil
}
