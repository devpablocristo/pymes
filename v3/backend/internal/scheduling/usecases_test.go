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

type durationAlgorithmsFake struct {
	duration int
}

func (f *durationAlgorithmsFake) NormalizeAllocations(
	values []domain.Allocation,
	_ int,
) ([]domain.Allocation, error) {
	return values, nil
}

func (f *durationAlgorithmsFake) CalculateSlots(
	query domain.AvailabilityQuery,
	_ domain.AvailabilitySnapshot,
) ([]domain.Slot, error) {
	f.duration = query.DurationMinutes
	duration := query.DurationMinutes
	if duration == 0 {
		duration = 60
	}
	return []domain.Slot{{
		StartAt:       query.From.UTC(),
		EndAt:         query.From.UTC().Add(time.Duration(duration) * time.Minute),
		OccupiesFrom:  query.From.UTC(),
		OccupiesUntil: query.From.UTC().Add(time.Duration(duration) * time.Minute),
		Timezone:      "UTC",
		Allocations:   query.Allocations,
		Remaining:     1,
	}}, nil
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

func (f *partyDirectoryFake) EnsureCustomer(
	context.Context,
	domain.CommandMetadata,
	PublicCustomer,
) (CustomerIdentity, error) {
	f.calls++
	return CustomerIdentity{PartyID: f.partyID, Name: "Ada"}, nil
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

type rescheduleRepositoryFake struct {
	Repository
	current     domain.Booking
	service     domain.Service
	resource    domain.Resource
	replacement domain.Booking
	reason      string
}

func (f *rescheduleRepositoryFake) GetBooking(
	context.Context,
	string,
	uuid.UUID,
) (domain.Booking, error) {
	return f.current, nil
}

func (f *rescheduleRepositoryFake) GetService(
	context.Context,
	string,
	uuid.UUID,
) (domain.Service, []domain.ResourceRequirement, error) {
	resourceID := f.resource.ID
	return f.service, []domain.ResourceRequirement{{
		ResourceID: &resourceID,
		Mode:       domain.AllocationExclusive,
		Units:      1,
	}}, nil
}

func (f *rescheduleRepositoryFake) GetResources(
	context.Context,
	string,
	[]uuid.UUID,
) ([]domain.Resource, error) {
	return []domain.Resource{f.resource}, nil
}

func (f *rescheduleRepositoryFake) ListResources(
	context.Context,
	string,
	uuid.UUID,
) ([]domain.Resource, error) {
	return []domain.Resource{f.resource}, nil
}

func (f *rescheduleRepositoryFake) LoadAvailability(
	context.Context,
	domain.AvailabilityQuery,
) (domain.AvailabilitySnapshot, error) {
	return domain.AvailabilitySnapshot{Service: f.service}, nil
}

func (f *rescheduleRepositoryFake) RescheduleBooking(
	_ context.Context,
	_ domain.CommandMetadata,
	_ uuid.UUID,
	_ int,
	replacement domain.Booking,
	_ []domain.Event,
) (domain.Booking, error) {
	f.replacement = replacement
	return replacement, nil
}

func (f *rescheduleRepositoryFake) TransitionBooking(
	_ context.Context,
	_ domain.CommandMetadata,
	_ uuid.UUID,
	_ int,
	status domain.BookingStatus,
	reason string,
	_ []domain.Event,
) (domain.Booking, error) {
	f.reason = reason
	result := f.current
	result.Status = status
	result.CancellationReason = reason
	result.Version++
	return result, nil
}

func TestResizeRevalidatesAndFreezesDurationAndCancellationReason(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	branchID, serviceID, resourceID, bookingID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	allocation := domain.Allocation{
		ResourceID: resourceID,
		Mode:       domain.AllocationExclusive,
		Units:      1,
	}
	current := domain.Booking{
		OrganizationID:  "org-resize",
		ID:              bookingID,
		BranchID:        branchID,
		ServiceID:       serviceID,
		PartyID:         "party-1",
		Status:          domain.BookingConfirmed,
		Participants:    1,
		StartAt:         now.Add(time.Hour),
		EndAt:           now.Add(2 * time.Hour),
		OccupiesFrom:    now.Add(time.Hour),
		OccupiesUntil:   now.Add(2 * time.Hour),
		Version:         1,
		ServiceName:     "Consulta",
		Price:           "100",
		Currency:        "ARS",
		DurationMinutes: 60,
		Timezone:        "UTC",
		Allocations:     []domain.Allocation{allocation},
		CreatedBy:       "actor",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	repository := &rescheduleRepositoryFake{
		current: current,
		service: domain.Service{
			OrganizationID:  current.OrganizationID,
			ID:              serviceID,
			DurationMinutes: 60,
			SlotMinutes:     15,
			MaxParticipants: 1,
			Active:          true,
		},
		resource: domain.Resource{
			OrganizationID: current.OrganizationID,
			ID:             resourceID,
			BranchID:       branchID,
			Kind:           domain.ResourceProfessional,
			Capacity:       1,
			Active:         true,
		},
	}
	algorithms := &durationAlgorithmsFake{}
	service := NewService(
		repository,
		algorithms,
		nil,
		WithClock(func() time.Time { return now }),
	)
	metadata := testMetadata(current.OrganizationID, "resize", current.ID.String())
	newStart := now.Add(4 * time.Hour)
	resized, err := service.RescheduleBooking(context.Background(), metadata, RescheduleInput{
		OrganizationID:  current.OrganizationID,
		BookingID:       current.ID,
		ExpectedVersion: 1,
		StartAt:         newStart,
		DurationMinutes: 90,
		Allocations:     []domain.Allocation{allocation},
	})
	if err != nil {
		t.Fatal(err)
	}
	if algorithms.duration != 90 || resized.DurationMinutes != 90 ||
		!resized.EndAt.Equal(newStart.Add(90*time.Minute)) ||
		resized.SubstateCode != "" {
		t.Fatalf("resize was not revalidated/frozen: booking=%+v duration=%d", resized, algorithms.duration)
	}
	repository.current.Status = domain.BookingHeld
	repository.current.SubstateCode = "awaiting_customer"
	heldReplacement, err := service.RescheduleBooking(
		context.Background(),
		testMetadata(current.OrganizationID, "resize-held", current.ID.String()),
		RescheduleInput{
			OrganizationID:  current.OrganizationID,
			BookingID:       current.ID,
			ExpectedVersion: 1,
			StartAt:         newStart.Add(24 * time.Hour),
			Allocations:     []domain.Allocation{allocation},
		},
	)
	if err != nil ||
		heldReplacement.Status != domain.BookingPendingConfirmation ||
		heldReplacement.SubstateCode != "" {
		t.Fatalf(
			"held replacement retained incompatible state: booking=%+v err=%v",
			heldReplacement,
			err,
		)
	}
	repository.current = current
	cancelMetadata := testMetadata(current.OrganizationID, "cancel", current.ID.String())
	cancelled, err := service.TransitionBooking(
		context.Background(), cancelMetadata, current.OrganizationID,
		current.ID, 1, domain.BookingCancelled, "Cliente sin disponibilidad",
	)
	if err != nil || cancelled.CancellationReason != "Cliente sin disponibilidad" ||
		repository.reason != "Cliente sin disponibilidad" {
		t.Fatalf("cancellation reason lost: booking=%+v reason=%q err=%v", cancelled, repository.reason, err)
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

type statusRepositoryFake struct {
	Repository
	configured              domain.BookingStatusConfiguration
	configureMetadata       domain.CommandMetadata
	configureCalls          int
	configurations          []domain.BookingStatusConfiguration
	listOrganizationID      string
	substateBookingID       uuid.UUID
	substateExpectedVersion int
	substateCode            string
	substateMetadata        domain.CommandMetadata
	substateCalls           int
}

func (f *statusRepositoryFake) ConfigureBookingStatus(
	_ context.Context,
	metadata domain.CommandMetadata,
	configuration domain.BookingStatusConfiguration,
) (domain.BookingStatusConfiguration, error) {
	f.configureCalls++
	f.configureMetadata = metadata
	f.configured = configuration
	return configuration, nil
}

func (f *statusRepositoryFake) ListBookingStatusConfigurations(
	_ context.Context,
	organizationID string,
) ([]domain.BookingStatusConfiguration, error) {
	f.listOrganizationID = organizationID
	return f.configurations, nil
}

func (f *statusRepositoryFake) SetBookingSubstate(
	_ context.Context,
	metadata domain.CommandMetadata,
	bookingID uuid.UUID,
	expectedVersion int,
	substateCode string,
) (domain.Booking, error) {
	f.substateCalls++
	f.substateMetadata = metadata
	f.substateBookingID = bookingID
	f.substateExpectedVersion = expectedVersion
	f.substateCode = substateCode
	return domain.Booking{
		OrganizationID: metadata.OrganizationID,
		ID:             bookingID,
		Status:         domain.BookingConfirmed,
		SubstateCode:   substateCode,
		Version:        expectedVersion + 1,
	}, nil
}

func TestBookingStatusCustomizationValidatesAndDelegates(t *testing.T) {
	organizationID := "org-status"
	bookingID := uuid.New()
	repository := &statusRepositoryFake{
		configurations: []domain.BookingStatusConfiguration{{
			OrganizationID: organizationID,
			Status:         domain.BookingConfirmed,
			Label:          "Confirmado",
		}},
	}
	service := NewService(repository, algorithmsFake{}, nil)
	metadata := testMetadata(organizationID, "status-configure", "status-configure")
	configuration := domain.BookingStatusConfiguration{
		OrganizationID: organizationID,
		Status:         domain.BookingConfirmed,
		Label:          "Confirmado",
		Substates: []domain.BookingSubstateDefinition{{
			Code: "arrived", Label: "Llegó", Active: true, SortOrder: 10,
		}},
	}

	configured, err := service.ConfigureBookingStatus(
		context.Background(),
		metadata,
		configuration,
	)
	if err != nil ||
		repository.configureCalls != 1 ||
		repository.configureMetadata.IdempotencyKey != metadata.IdempotencyKey ||
		configured.Status != domain.BookingConfirmed {
		t.Fatalf(
			"configured=%+v calls=%d metadata=%+v err=%v",
			configured,
			repository.configureCalls,
			repository.configureMetadata,
			err,
		)
	}

	mismatched := configuration
	mismatched.OrganizationID = "org-other"
	if _, err := service.ConfigureBookingStatus(
		context.Background(),
		metadata,
		mismatched,
	); domain.ErrorCodeOf(err) != domain.CodeValidation ||
		repository.configureCalls != 1 {
		t.Fatalf(
			"tenant mismatch err=%v calls=%d",
			err,
			repository.configureCalls,
		)
	}

	configurations, err := service.ListBookingStatusConfigurations(
		context.Background(),
		organizationID,
	)
	if err != nil ||
		len(configurations) != 1 ||
		repository.listOrganizationID != organizationID {
		t.Fatalf(
			"configurations=%+v org=%q err=%v",
			configurations,
			repository.listOrganizationID,
			err,
		)
	}
	if _, err := service.ListBookingStatusConfigurations(
		context.Background(),
		" ",
	); domain.ErrorCodeOf(err) != domain.CodeValidation {
		t.Fatalf("blank organization err=%v", err)
	}

	booking, err := service.SetBookingSubstate(
		context.Background(),
		metadata,
		organizationID,
		bookingID,
		3,
		" arrived ",
	)
	if err != nil ||
		repository.substateCalls != 1 ||
		repository.substateBookingID != bookingID ||
		repository.substateExpectedVersion != 3 ||
		repository.substateCode != "arrived" ||
		repository.substateMetadata.IdempotencyKey != metadata.IdempotencyKey ||
		booking.SubstateCode != "arrived" {
		t.Fatalf(
			"booking=%+v calls=%d booking_id=%s version=%d code=%q metadata=%+v err=%v",
			booking,
			repository.substateCalls,
			repository.substateBookingID,
			repository.substateExpectedVersion,
			repository.substateCode,
			repository.substateMetadata,
			err,
		)
	}
	for _, test := range []struct {
		name           string
		organizationID string
		bookingID      uuid.UUID
		version        int
		code           string
	}{
		{
			name: "tenant mismatch", organizationID: "org-other",
			bookingID: bookingID, version: 3, code: "arrived",
		},
		{
			name: "missing booking", organizationID: organizationID,
			bookingID: uuid.Nil, version: 3, code: "arrived",
		},
		{
			name: "invalid version", organizationID: organizationID,
			bookingID: bookingID, version: 0, code: "arrived",
		},
		{
			name: "invalid code", organizationID: organizationID,
			bookingID: bookingID, version: 3, code: "Arrived!",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.SetBookingSubstate(
				context.Background(),
				metadata,
				test.organizationID,
				test.bookingID,
				test.version,
				test.code,
			)
			if domain.ErrorCodeOf(err) != domain.CodeValidation {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if repository.substateCalls != 1 {
		t.Fatalf("invalid commands reached repository: calls=%d", repository.substateCalls)
	}
}

type maintenanceRepositoryFake struct {
	Repository
	candidate domain.WaitlistEntry
	service   domain.Service
	resource  domain.Resource
	offers    int
	releases  int
	slot      domain.Slot
}

func (f *maintenanceRepositoryFake) ExpireHolds(
	context.Context,
	int,
	time.Time,
) ([]domain.Booking, error) {
	return []domain.Booking{}, nil
}

func (f *maintenanceRepositoryFake) ClaimReminders(
	context.Context,
	int,
	time.Time,
	time.Time,
) ([]domain.Event, error) {
	return []domain.Event{}, nil
}

func (f *maintenanceRepositoryFake) ClaimWaitlistCandidates(
	context.Context,
	int,
	time.Time,
) ([]domain.WaitlistEntry, error) {
	return []domain.WaitlistEntry{f.candidate}, nil
}

func (f *maintenanceRepositoryFake) GetService(
	context.Context,
	string,
	uuid.UUID,
) (domain.Service, []domain.ResourceRequirement, error) {
	resourceID := f.resource.ID
	return f.service, []domain.ResourceRequirement{{
		ResourceID: &resourceID,
		Mode:       domain.AllocationExclusive,
		Units:      1,
	}}, nil
}

func (f *maintenanceRepositoryFake) ListResources(
	context.Context,
	string,
	uuid.UUID,
) ([]domain.Resource, error) {
	return []domain.Resource{f.resource}, nil
}

func (f *maintenanceRepositoryFake) LoadAvailability(
	context.Context,
	domain.AvailabilityQuery,
) (domain.AvailabilitySnapshot, error) {
	return domain.AvailabilitySnapshot{Service: f.service}, nil
}

func (f *maintenanceRepositoryFake) ReleaseWaitlistClaim(
	context.Context,
	string,
	uuid.UUID,
) error {
	f.releases++
	return nil
}

func (f *maintenanceRepositoryFake) OfferWaitlist(
	_ context.Context,
	_ string,
	_ uuid.UUID,
	slot domain.Slot,
	_ time.Time,
	_ domain.ActionToken,
	_ []domain.Event,
) (domain.WaitlistEntry, error) {
	f.offers++
	f.slot = slot
	result := f.candidate
	result.Status = domain.WaitlistOffered
	result.Version++
	return result, nil
}

func TestMaintenanceOffersWaitlistOnlyWhenAConcreteSlotExists(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	branchID, serviceID, resourceID := uuid.New(), uuid.New(), uuid.New()
	candidate := domain.WaitlistEntry{
		OrganizationID: "org-waitlist",
		ID:             uuid.New(),
		BranchID:       branchID,
		ServiceID:      serviceID,
		PartyID:        "party-1",
		PreferredFrom:  now.Add(time.Hour),
		PreferredUntil: now.Add(3 * time.Hour),
		Participants:   1,
		Status:         domain.WaitlistPending,
		Version:        1,
	}
	serviceSnapshot := domain.Service{
		OrganizationID:  candidate.OrganizationID,
		ID:              serviceID,
		DurationMinutes: 30,
		SlotMinutes:     30,
		MaxParticipants: 1,
		Active:          true,
	}
	resource := domain.Resource{
		OrganizationID: candidate.OrganizationID,
		ID:             resourceID,
		BranchID:       branchID,
		Kind:           domain.ResourceProfessional,
		Capacity:       1,
		Active:         true,
	}
	codec, err := NewHMACActionTokenCodec([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("no availability releases claim", func(t *testing.T) {
		repository := &maintenanceRepositoryFake{
			candidate: candidate,
			service:   serviceSnapshot,
			resource:  resource,
		}
		result, err := NewService(
			repository,
			algorithmsFake{},
			codec,
			WithClock(func() time.Time { return now }),
		).RunMaintenance(context.Background(), 10)
		if err != nil || result.WaitlistOffers != 0 ||
			repository.offers != 0 || repository.releases != 1 {
			t.Fatalf(
				"result=%+v offers=%d releases=%d err=%v",
				result, repository.offers, repository.releases, err,
			)
		}
	})
	t.Run("availability persists exact offered slot", func(t *testing.T) {
		slot := domain.Slot{
			StartAt:       candidate.PreferredFrom,
			EndAt:         candidate.PreferredFrom.Add(30 * time.Minute),
			OccupiesFrom:  candidate.PreferredFrom,
			OccupiesUntil: candidate.PreferredFrom.Add(30 * time.Minute),
			Timezone:      "UTC",
			Allocations: []domain.Allocation{{
				ResourceID: resourceID,
				Mode:       domain.AllocationExclusive,
				Units:      1,
			}},
		}
		repository := &maintenanceRepositoryFake{
			candidate: candidate,
			service:   serviceSnapshot,
			resource:  resource,
		}
		result, err := NewService(
			repository,
			algorithmsFake{slots: []domain.Slot{slot}},
			codec,
			WithClock(func() time.Time { return now }),
		).RunMaintenance(context.Background(), 10)
		if err != nil || result.WaitlistOffers != 1 ||
			repository.offers != 1 || !repository.slot.StartAt.Equal(slot.StartAt) {
			t.Fatalf("result=%+v slot=%+v err=%v", result, repository.slot, err)
		}
	})
}
