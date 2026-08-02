package scheduling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/repository/helpers"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresSchedulingTenantIsolationConcurrencyAndRecovery(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repository := NewPostgresRepository(pool)
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	repository.now = func() time.Time { return now }

	sharedBranchID, sharedServiceID := uuid.New(), uuid.New()
	sharedResourceID, sharedRoomID := uuid.New(), uuid.New()
	sharedBookingID := uuid.New()
	type tenantFixture struct {
		organizationID string
		partyID        string
	}
	fixtures := make([]tenantFixture, 0, 2)
	for index := range 2 {
		suffix := uuid.NewString()
		fixture := tenantFixture{
			organizationID: "org_sched_" + suffix,
			partyID:        "party_" + suffix,
		}
		fixtures = append(fixtures, fixture)
		seedSchedulingTenant(
			t, pool, fixture.organizationID, fixture.partyID,
			sharedBranchID, sharedServiceID, sharedResourceID, sharedRoomID,
		)
		booking := testBooking(
			fixture.organizationID, sharedBookingID, sharedBranchID, sharedServiceID,
			fixture.partyID, sharedResourceID, now.Add(time.Duration(index+24)*time.Hour),
			domain.BookingConfirmed,
		)
		metadata := testMetadata(fixture.organizationID, "tenant-booking-"+suffix, booking.ID.String())
		tokenDigest := sha256.Sum256([]byte(fixture.organizationID + ":" + booking.ID.String()))
		token := domain.ActionToken{
			OrganizationID: fixture.organizationID,
			ID:             uuid.New(),
			BookingID:      &booking.ID,
			Purpose:        domain.ActionCancel,
			TokenHash:      hex.EncodeToString(tokenDigest[:]),
			ExpiresAt:      now.Add(24 * time.Hour),
			CreatedAt:      now,
		}
		created, err := repository.ReserveBookings(
			ctx, metadata, nil, []domain.Booking{booking}, []domain.ActionToken{token},
			bookingEvents(metadata, booking, domain.EventBookingCreated),
		)
		if err != nil || len(created) != 1 {
			t.Fatalf("seed tenant booking: result=%+v err=%v", created, err)
		}
		assertSchedulingEventOwnership(t, pool, fixture.organizationID, booking.ID)
		storedToken, err := repository.FindActionToken(ctx, token.TokenHash)
		if err != nil || storedToken.OrganizationID != fixture.organizationID ||
			storedToken.BookingID == nil || *storedToken.BookingID != booking.ID {
			t.Fatalf("booking action token not committed atomically: token=%+v err=%v", storedToken, err)
		}
	}
	for _, fixture := range fixtures {
		got, err := repository.GetBooking(ctx, fixture.organizationID, sharedBookingID)
		if err != nil {
			t.Fatal(err)
		}
		if got.OrganizationID != fixture.organizationID || got.PartyID != fixture.partyID {
			t.Fatalf("tenant escaped: %+v fixture=%+v", got, fixture)
		}
	}

	organizationID := fixtures[0].organizationID
	partyID := fixtures[0].partyID
	concurrentAt := now.Add(72 * time.Hour)
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for index := range 2 {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			booking := testBooking(
				organizationID, uuid.New(), sharedBranchID, sharedServiceID,
				partyID, sharedResourceID, concurrentAt, domain.BookingConfirmed,
			)
			booking.Allocations = append(booking.Allocations, domain.Allocation{
				ResourceID: sharedRoomID, Mode: domain.AllocationExclusive, Units: 1,
			})
			if index == 1 {
				booking.Allocations[0], booking.Allocations[1] =
					booking.Allocations[1], booking.Allocations[0]
			}
			metadata := testMetadata(
				organizationID, fmt.Sprintf("concurrent-%d-%s", index, uuid.NewString()),
				booking.ID.String(),
			)
			_, err := repository.ReserveBookings(
				ctx, metadata, nil, []domain.Booking{booking}, nil,
				bookingEvents(metadata, booking, domain.EventBookingCreated),
			)
			results <- err
		}(index)
	}
	wait.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if domain.ErrorCodeOf(err) == domain.CodeResourceConflict ||
			domain.ErrorCodeOf(err) == domain.CodeCapacityExceeded {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent reservation error: %v", err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent reservations successes=%d conflicts=%d", successes, conflicts)
	}

	idempotentAt := now.Add(96 * time.Hour)
	idempotent := testBooking(
		organizationID, uuid.New(), sharedBranchID, sharedServiceID,
		partyID, sharedResourceID, idempotentAt, domain.BookingConfirmed,
	)
	idempotency := testMetadata(organizationID, "idempotent-"+uuid.NewString(), idempotent.ID.String())
	first, err := repository.ReserveBookings(
		ctx, idempotency, nil, []domain.Booking{idempotent}, nil,
		bookingEvents(idempotency, idempotent, domain.EventBookingCreated),
	)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := repository.ReserveBookings(
		ctx, idempotency, nil, []domain.Booking{idempotent}, nil,
		bookingEvents(idempotency, idempotent, domain.EventBookingCreated),
	)
	if err != nil || replayed[0].ID != first[0].ID {
		t.Fatalf("idempotent replay=%+v first=%+v err=%v", replayed, first, err)
	}
	cancelMetadata := testMetadata(
		organizationID, "cancel-"+uuid.NewString(), idempotent.ID.String(),
	)
	cancelReason := "Cliente solicitó la cancelación"
	cancelEvent := newEvent(
		cancelMetadata, idempotent.ID.String(), domain.EventBookingCancelled,
		map[string]any{"booking_id": idempotent.ID, "reason": cancelReason},
	)
	cancelled, err := repository.TransitionBooking(
		ctx, cancelMetadata, idempotent.ID, 1, domain.BookingCancelled,
		cancelReason, []domain.Event{cancelEvent},
	)
	if err != nil || cancelled.CancellationReason != cancelReason {
		t.Fatalf("persist cancellation reason: booking=%+v err=%v", cancelled, err)
	}

	holdAt := now.Add(120 * time.Hour)
	hold := testBooking(
		organizationID, uuid.New(), sharedBranchID, sharedServiceID,
		partyID, sharedResourceID, holdAt, domain.BookingHeld,
	)
	holdExpiry := now.Add(5 * time.Minute)
	hold.HoldExpiresAt = &holdExpiry
	holdMetadata := testMetadata(organizationID, "hold-"+uuid.NewString(), hold.ID.String())
	if _, err := repository.ReserveBookings(
		ctx, holdMetadata, nil, []domain.Booking{hold}, nil,
		bookingEvents(holdMetadata, hold, domain.EventBookingCreated),
	); err != nil {
		t.Fatal(err)
	}
	expired, err := repository.ExpireHolds(ctx, 10, holdExpiry.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	foundExpired := false
	for _, value := range expired {
		foundExpired = foundExpired || value.ID == hold.ID
	}
	if !foundExpired {
		t.Fatalf("hold %s did not expire: %+v", hold.ID, expired)
	}
	afterExpiry := testBooking(
		organizationID, uuid.New(), sharedBranchID, sharedServiceID,
		partyID, sharedResourceID, holdAt, domain.BookingConfirmed,
	)
	afterExpiryMetadata := testMetadata(organizationID, "after-hold-"+uuid.NewString(), afterExpiry.ID.String())
	if _, err := repository.ReserveBookings(
		ctx, afterExpiryMetadata, nil, []domain.Booking{afterExpiry}, nil,
		bookingEvents(afterExpiryMetadata, afterExpiry, domain.EventBookingCreated),
	); err != nil {
		t.Fatalf("expired hold did not release resource: %v", err)
	}

	sessionAt := now.Add(144 * time.Hour)
	session := domain.GroupSession{
		OrganizationID: organizationID, ID: uuid.New(), BranchID: sharedBranchID,
		ServiceID: sharedServiceID, StartAt: sessionAt, EndAt: sessionAt.Add(time.Hour),
		Capacity: 2, Version: 1, Status: "open",
	}
	sessionMetadata := testMetadata(organizationID, "session-"+uuid.NewString(), session.ID.String())
	sessionEvent := newEvent(sessionMetadata, session.ID.String(), "GroupSessionCreated", map[string]any{"session_id": session.ID})
	if _, err := repository.CreateGroupSession(
		ctx, sessionMetadata, session,
		[]domain.Allocation{{ResourceID: sharedResourceID, Mode: domain.AllocationExclusive, Units: 1}},
		sessionEvent,
	); err != nil {
		t.Fatal(err)
	}
	groupResults := make(chan error, 2)
	for index := range 2 {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			booking := testBooking(
				organizationID, uuid.New(), sharedBranchID, sharedServiceID,
				partyID, sharedResourceID, sessionAt, domain.BookingConfirmed,
			)
			booking.SessionID = &session.ID
			booking.Participants = 2
			booking.Allocations = nil
			metadata := testMetadata(organizationID, fmt.Sprintf("group-%d-%s", index, uuid.NewString()), booking.ID.String())
			_, err := repository.ReserveBookings(
				ctx, metadata, nil, []domain.Booking{booking}, nil,
				bookingEvents(metadata, booking, domain.EventBookingCreated),
			)
			groupResults <- err
		}(index)
	}
	wait.Wait()
	close(groupResults)
	successes, conflicts = 0, 0
	for err := range groupResults {
		if err == nil {
			successes++
		} else if domain.ErrorCodeOf(err) == domain.CodeCapacityExceeded {
			conflicts++
		} else {
			t.Fatalf("unexpected group reservation error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("group reservations successes=%d conflicts=%d", successes, conflicts)
	}

	rescheduleAt := now.Add(168 * time.Hour)
	original := testBooking(
		organizationID, uuid.New(), sharedBranchID, sharedServiceID,
		partyID, sharedResourceID, rescheduleAt, domain.BookingConfirmed,
	)
	originalMetadata := testMetadata(organizationID, "original-"+uuid.NewString(), original.ID.String())
	if _, err := repository.ReserveBookings(
		ctx, originalMetadata, nil, []domain.Booking{original}, nil,
		bookingEvents(originalMetadata, original, domain.EventBookingCreated),
	); err != nil {
		t.Fatal(err)
	}
	replacement := original
	replacement.ID = uuid.New()
	replacement.SupersedesID = &original.ID
	replacement.StartAt = rescheduleAt.Add(2 * time.Hour)
	replacement.EndAt = replacement.StartAt.Add(time.Hour)
	replacement.OccupiesFrom = replacement.StartAt
	replacement.OccupiesUntil = replacement.EndAt
	replacement.Version = 1
	rescheduleMetadata := testMetadata(organizationID, "reschedule-"+uuid.NewString(), original.ID.String())
	rescheduleEvent := newEvent(
		rescheduleMetadata, replacement.ID.String(), domain.EventBookingRescheduled,
		map[string]any{"booking_id": replacement.ID},
	)
	if _, err := repository.RescheduleBooking(
		ctx, rescheduleMetadata, original.ID, 1, replacement, []domain.Event{rescheduleEvent},
	); err != nil {
		t.Fatal(err)
	}
	staleMetadata := testMetadata(
		organizationID, "reschedule-stale-"+uuid.NewString(), original.ID.String()+":stale",
	)
	if _, err := repository.RescheduleBooking(
		ctx, staleMetadata, original.ID, 1, replacement, []domain.Event{rescheduleEvent},
	); domain.ErrorCodeOf(err) != domain.CodeBookingVersionConflict {
		t.Fatalf("stale reschedule error=%v", err)
	}

	waitlistAt := now.Add(216 * time.Hour)
	waitlist := domain.WaitlistEntry{
		OrganizationID: organizationID,
		ID:             uuid.New(),
		BranchID:       sharedBranchID,
		ServiceID:      sharedServiceID,
		PartyID:        partyID,
		PreferredFrom:  waitlistAt,
		PreferredUntil: waitlistAt.Add(2 * time.Hour),
		Participants:   1,
		Status:         domain.WaitlistPending,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	waitlistMetadata := testMetadata(
		organizationID, "waitlist-"+uuid.NewString(), waitlist.ID.String(),
	)
	waitlistEvent := newEvent(
		waitlistMetadata, waitlist.ID.String(), "WaitlistCreated",
		map[string]any{"waitlist_id": waitlist.ID},
	)
	if _, err := repository.CreateWaitlistEntry(
		ctx, waitlistMetadata, waitlist, waitlistEvent,
	); err != nil {
		t.Fatal(err)
	}
	codec, err := NewHMACActionTokenCodec([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	rawToken, tokenHash, err := codec.Issue()
	if err != nil {
		t.Fatal(err)
	}
	actionToken := domain.ActionToken{
		OrganizationID: organizationID,
		ID:             uuid.New(),
		WaitlistID:     &waitlist.ID,
		Purpose:        domain.ActionAcceptWaitlist,
		TokenHash:      tokenHash,
		ExpiresAt:      now.Add(time.Hour),
		CreatedAt:      now,
	}
	offeredSlot := domain.Slot{
		StartAt:       waitlistAt,
		EndAt:         waitlistAt.Add(time.Hour),
		OccupiesFrom:  waitlistAt,
		OccupiesUntil: waitlistAt.Add(time.Hour),
		Timezone:      "UTC",
		Allocations: []domain.Allocation{{
			ResourceID: sharedResourceID,
			Mode:       domain.AllocationExclusive,
			Units:      1,
		}},
		Remaining: 1,
	}
	offered, err := repository.OfferWaitlist(
		ctx, organizationID, waitlist.ID, offeredSlot, now.Add(30*time.Minute),
		actionToken, []domain.Event{newEvent(
			waitlistMetadata, waitlist.ID.String(), domain.EventWaitlistOffered,
			map[string]any{"waitlist_id": waitlist.ID},
		)},
	)
	if err != nil || offered.Version != 2 || offered.OfferedStartAt == nil {
		t.Fatalf("offer waitlist: offered=%+v err=%v", offered, err)
	}
	waitlistService := NewService(
		repository,
		algorithmsFake{slots: []domain.Slot{offeredSlot}},
		codec,
		WithClock(func() time.Time { return now }),
	)
	actionMetadata := testMetadata("", "accept-waitlist-"+uuid.NewString(), waitlist.ID.String())
	accepted, err := waitlistService.ConsumeWaitlistAction(
		ctx, rawToken, actionMetadata, offered.Version,
	)
	if err != nil || accepted.Status != domain.WaitlistAccepted ||
		accepted.AcceptedBookingID == nil {
		t.Fatalf("accept waitlist: accepted=%+v err=%v", accepted, err)
	}
	replayedAccepted, err := waitlistService.ConsumeWaitlistAction(
		ctx, rawToken, actionMetadata, offered.Version,
	)
	if err != nil || replayedAccepted.AcceptedBookingID == nil ||
		*replayedAccepted.AcceptedBookingID != *accepted.AcceptedBookingID {
		t.Fatalf("replay waitlist action: replay=%+v accepted=%+v err=%v", replayedAccepted, accepted, err)
	}
	if _, err := repository.GetBooking(
		ctx, organizationID, *accepted.AcceptedBookingID,
	); err != nil {
		t.Fatalf("accepted waitlist booking is missing: %v", err)
	}
}

func TestPostgresBookingUpdateIsVersionedIdempotentAndPreservesImmutableFields(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	suffix := uuid.NewString()
	organizationID := "org_sched_update_" + suffix
	partyID := "party_sched_update_" + suffix
	branchID, serviceID, resourceID, roomID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedSchedulingTenant(
		t,
		pool,
		organizationID,
		partyID,
		branchID,
		serviceID,
		resourceID,
		roomID,
	)
	repository := NewPostgresRepository(pool)
	startAt := time.Date(2026, time.August, 12, 14, 0, 0, 0, time.UTC)
	original := testBooking(
		organizationID,
		uuid.New(),
		branchID,
		serviceID,
		partyID,
		resourceID,
		startAt,
		domain.BookingConfirmed,
	)
	original.CustomerName = "Cliente original"
	original.CustomerEmail = "original@example.com"
	createMetadata := testMetadata(
		organizationID,
		"create-update-"+suffix,
		original.ID.String(),
	)
	if _, err := repository.ReserveBookings(
		ctx,
		createMetadata,
		nil,
		[]domain.Booking{original},
		nil,
		nil,
	); err != nil {
		t.Fatal(err)
	}
	persistedOriginal, err := repository.GetBooking(ctx, organizationID, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	update := BookingUpdate{
		PartyID:       partyID,
		CustomerName:  "Cliente actualizado",
		CustomerEmail: "actualizado@example.com",
		CustomerPhone: "+541155555555",
		Participants:  2,
		Notes:         "Acceso por recepción",
		Allocations:   append([]domain.Allocation(nil), original.Allocations...),
	}
	updateMetadata := testMetadata(
		organizationID,
		"update-booking-"+suffix,
		original.ID.String(),
	)
	event := newEvent(
		updateMetadata,
		original.ID.String(),
		domain.EventBookingUpdated,
		map[string]any{"booking_id": original.ID, "version": 2},
	)
	updated, err := repository.UpdateBooking(
		ctx,
		updateMetadata,
		original.ID,
		1,
		update,
		[]domain.Event{event},
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 ||
		updated.Participants != 2 ||
		updated.CustomerName != update.CustomerName ||
		updated.CustomerEmail != update.CustomerEmail ||
		updated.CustomerPhone != update.CustomerPhone ||
		updated.Notes != update.Notes {
		t.Fatalf("editable fields not persisted: %+v", updated)
	}
	if updated.BranchID != persistedOriginal.BranchID ||
		updated.ServiceID != persistedOriginal.ServiceID ||
		!updated.StartAt.Equal(persistedOriginal.StartAt) ||
		!updated.EndAt.Equal(persistedOriginal.EndAt) ||
		updated.Status != persistedOriginal.Status ||
		updated.ServiceName != persistedOriginal.ServiceName ||
		updated.Price != persistedOriginal.Price ||
		updated.Currency != persistedOriginal.Currency ||
		updated.DurationMinutes != persistedOriginal.DurationMinutes ||
		updated.Timezone != persistedOriginal.Timezone {
		t.Fatalf("immutable fields changed: before=%+v after=%+v", persistedOriginal, updated)
	}
	preflight, replayed, err := repository.ReplayBookingUpdate(
		ctx,
		updateMetadata,
		original.ID,
	)
	if err != nil || !replayed || preflight.Version != updated.Version ||
		preflight.Notes != updated.Notes {
		t.Fatalf("preflight=%+v replayed=%v err=%v", preflight, replayed, err)
	}
	if _, _, err := repository.ReplayBookingUpdate(
		ctx,
		updateMetadata,
		uuid.New(),
	); domain.ErrorCodeOf(err) != domain.CodeIdempotencyKeyReused {
		t.Fatalf("cross-booking replay error=%v", err)
	}
	exactReplay, err := repository.UpdateBooking(
		ctx,
		updateMetadata,
		original.ID,
		1,
		update,
		[]domain.Event{event},
	)
	if err != nil || exactReplay.Version != updated.Version ||
		exactReplay.Notes != updated.Notes {
		t.Fatalf("exact replay=%+v err=%v", exactReplay, err)
	}
	reusedMetadata := updateMetadata
	reusedDigest := sha256.Sum256([]byte("different-payload"))
	reusedMetadata.PayloadHash = hex.EncodeToString(reusedDigest[:])
	if _, _, err := repository.ReplayBookingUpdate(
		ctx,
		reusedMetadata,
		original.ID,
	); domain.ErrorCodeOf(err) != domain.CodeIdempotencyKeyReused {
		t.Fatalf("preflight reused key error=%v", err)
	}
	if _, err := repository.UpdateBooking(
		ctx,
		reusedMetadata,
		original.ID,
		2,
		update,
		nil,
	); domain.ErrorCodeOf(err) != domain.CodeIdempotencyKeyReused {
		t.Fatalf("transactional reused key error=%v", err)
	}
	staleMetadata := testMetadata(
		organizationID,
		"stale-update-"+suffix,
		original.ID.String()+":stale",
	)
	if _, err := repository.UpdateBooking(
		ctx,
		staleMetadata,
		original.ID,
		1,
		update,
		nil,
	); domain.ErrorCodeOf(err) != domain.CodeBookingVersionConflict {
		t.Fatalf("stale update error=%v", err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM app.scheduling_audit
		WHERE org_id=$1 AND aggregate_id=$2
		  AND action='scheduling.booking.updated'`,
		organizationID,
		original.ID.String(),
	).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("update audit count=%d", auditCount)
	}
}

func TestPostgresPublicBookingActionIsConcurrentAndExactlyOnce(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	suffix := uuid.NewString()
	organizationID := "org_public_action_" + suffix
	partyID := "party_public_action_" + suffix
	branchID, serviceID := uuid.New(), uuid.New()
	resourceID, roomID := uuid.New(), uuid.New()
	seedSchedulingTenant(
		t,
		pool,
		organizationID,
		partyID,
		branchID,
		serviceID,
		resourceID,
		roomID,
	)
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	repository := NewPostgresRepository(pool)
	codec, err := NewHMACActionTokenCodec(
		[]byte("01234567890123456789012345678901"),
	)
	if err != nil {
		t.Fatal(err)
	}
	rawToken, tokenHash, err := codec.Issue()
	if err != nil {
		t.Fatal(err)
	}
	booking := testBooking(
		organizationID,
		uuid.New(),
		branchID,
		serviceID,
		partyID,
		resourceID,
		now.Add(24*time.Hour),
		domain.BookingPendingConfirmation,
	)
	bookingID := booking.ID
	token := domain.ActionToken{
		OrganizationID: organizationID,
		ID:             uuid.New(),
		BookingID:      &bookingID,
		Purpose:        domain.ActionConfirm,
		TokenHash:      tokenHash,
		ExpiresAt:      now.Add(time.Hour),
		CreatedAt:      now,
	}
	createMetadata := testMetadata(
		organizationID,
		"create-public-action-"+suffix,
		booking.ID.String(),
	)
	if _, err := repository.ReserveBookings(
		ctx,
		createMetadata,
		nil,
		[]domain.Booking{booking},
		[]domain.ActionToken{token},
		bookingEvents(
			createMetadata,
			booking,
			domain.EventBookingCreated,
		),
	); err != nil {
		t.Fatal(err)
	}
	service := NewService(
		repository,
		algorithmsFake{},
		codec,
		WithClock(func() time.Time { return now }),
	)
	actionMetadata := testMetadata(
		"",
		"confirm-public-action-"+suffix,
		booking.ID.String(),
	)
	results := make(chan domain.Booking, 2)
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, actionErr := service.ConsumeBookingAction(
				ctx,
				rawToken,
				domain.ActionConfirm,
				actionMetadata,
				1,
				nil,
				0,
				"",
			)
			results <- result
			errs <- actionErr
		}()
	}
	wait.Wait()
	close(results)
	close(errs)
	for actionErr := range errs {
		if actionErr != nil {
			t.Fatalf("concurrent action failed: %v", actionErr)
		}
	}
	for result := range results {
		if result.ID != booking.ID ||
			result.Status != domain.BookingConfirmed ||
			result.Version != 2 {
			t.Fatalf("unexpected concurrent action result: %+v", result)
		}
	}
	stored, err := repository.GetBooking(ctx, organizationID, booking.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.BookingConfirmed || stored.Version != 2 {
		t.Fatalf("booking was mutated more than once: %+v", stored)
	}
	storedToken, err := repository.FindActionToken(ctx, tokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if storedToken.ConsumedAt == nil ||
		storedToken.ResultBookingID == nil ||
		*storedToken.ResultBookingID != booking.ID {
		t.Fatalf("token was not committed with result: %+v", storedToken)
	}
	assertPublicActionEffects(
		t,
		pool,
		organizationID,
		booking.ID,
		domain.EventBookingConfirmed,
		actionMetadata.CorrelationID,
		1,
	)
}

func TestPostgresPublicBookingActionRollsBackBeforeTokenConsumption(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	suffix := uuid.NewString()
	organizationID := "org_public_rollback_" + suffix
	partyID := "party_public_rollback_" + suffix
	branchID, serviceID := uuid.New(), uuid.New()
	resourceID, roomID := uuid.New(), uuid.New()
	seedSchedulingTenant(
		t,
		pool,
		organizationID,
		partyID,
		branchID,
		serviceID,
		resourceID,
		roomID,
	)
	now := time.Date(2026, time.August, 21, 12, 0, 0, 0, time.UTC)
	repository := NewPostgresRepository(pool)
	codec, err := NewHMACActionTokenCodec(
		[]byte("01234567890123456789012345678901"),
	)
	if err != nil {
		t.Fatal(err)
	}
	rawToken, tokenHash, err := codec.Issue()
	if err != nil {
		t.Fatal(err)
	}
	booking := testBooking(
		organizationID,
		uuid.New(),
		branchID,
		serviceID,
		partyID,
		resourceID,
		now.Add(24*time.Hour),
		domain.BookingPendingConfirmation,
	)
	bookingID := booking.ID
	token := domain.ActionToken{
		OrganizationID: organizationID,
		ID:             uuid.New(),
		BookingID:      &bookingID,
		Purpose:        domain.ActionConfirm,
		TokenHash:      tokenHash,
		ExpiresAt:      now.Add(time.Hour),
		CreatedAt:      now,
	}
	createMetadata := testMetadata(
		organizationID,
		"create-public-rollback-"+suffix,
		booking.ID.String(),
	)
	if _, err := repository.ReserveBookings(
		ctx,
		createMetadata,
		nil,
		[]domain.Booking{booking},
		[]domain.ActionToken{token},
		bookingEvents(
			createMetadata,
			booking,
			domain.EventBookingCreated,
		),
	); err != nil {
		t.Fatal(err)
	}
	dropFailure := installPublicActionTokenFailure(
		t,
		pool,
		organizationID,
		tokenHash,
	)
	defer dropFailure()
	service := NewService(
		repository,
		algorithmsFake{},
		codec,
		WithClock(func() time.Time { return now }),
	)
	actionMetadata := testMetadata(
		"",
		"confirm-public-rollback-"+suffix,
		booking.ID.String(),
	)
	if _, err := service.ConsumeBookingAction(
		ctx,
		rawToken,
		domain.ActionConfirm,
		actionMetadata,
		1,
		nil,
		0,
		"",
	); err == nil {
		t.Fatal("fault injection unexpectedly committed the public action")
	}
	stored, err := repository.GetBooking(ctx, organizationID, booking.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.BookingPendingConfirmation || stored.Version != 1 {
		t.Fatalf("booking escaped the failed transaction: %+v", stored)
	}
	storedToken, err := repository.FindActionToken(ctx, tokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if storedToken.ConsumedAt != nil || storedToken.ResultBookingID != nil {
		t.Fatalf("failed action consumed its token: %+v", storedToken)
	}
	assertPublicActionEffects(
		t,
		pool,
		organizationID,
		booking.ID,
		domain.EventBookingConfirmed,
		actionMetadata.CorrelationID,
		0,
	)
	var idempotencyRecords int
	idempotencyTx, err := repositoryhelpers.BeginTenant(ctx, pool, organizationID)
	if err != nil {
		t.Fatal(err)
	}
	if err := idempotencyTx.QueryRow(ctx, `
		SELECT count(*)
		FROM app.idempotency_records
		WHERE org_id=$1
		  AND operation=$2
		  AND idempotency_key=$3`,
		organizationID,
		operationPublicBookingAction,
		"public-action:"+tokenHash,
	).Scan(&idempotencyRecords); err != nil {
		_ = idempotencyTx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := idempotencyTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if idempotencyRecords != 0 {
		t.Fatalf("failed action left %d idempotency rows", idempotencyRecords)
	}

	dropFailure()
	result, err := service.ConsumeBookingAction(
		ctx,
		rawToken,
		domain.ActionConfirm,
		actionMetadata,
		1,
		nil,
		0,
		"",
	)
	if err != nil {
		t.Fatalf("retry after rollback failed: %v", err)
	}
	if result.Status != domain.BookingConfirmed || result.Version != 2 {
		t.Fatalf("retry returned unexpected booking: %+v", result)
	}
}

func TestPostgresPublicWaitlistAcceptanceIsConcurrentAndExactlyOnce(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	suffix := uuid.NewString()
	organizationID := "org_waitlist_action_" + suffix
	partyID := "party_waitlist_action_" + suffix
	branchID, serviceID := uuid.New(), uuid.New()
	resourceID, roomID := uuid.New(), uuid.New()
	seedSchedulingTenant(
		t,
		pool,
		organizationID,
		partyID,
		branchID,
		serviceID,
		resourceID,
		roomID,
	)
	now := time.Date(2026, time.August, 22, 12, 0, 0, 0, time.UTC)
	repository := NewPostgresRepository(pool)
	codec, err := NewHMACActionTokenCodec(
		[]byte("01234567890123456789012345678901"),
	)
	if err != nil {
		t.Fatal(err)
	}
	waitlistAt := now.Add(24 * time.Hour)
	waitlist := domain.WaitlistEntry{
		OrganizationID: organizationID,
		ID:             uuid.New(),
		BranchID:       branchID,
		ServiceID:      serviceID,
		PartyID:        partyID,
		CustomerName:   "Waitlist Customer",
		PreferredFrom:  waitlistAt,
		PreferredUntil: waitlistAt.Add(2 * time.Hour),
		Participants:   1,
		Status:         domain.WaitlistPending,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	waitlistMetadata := testMetadata(
		organizationID,
		"create-waitlist-action-"+suffix,
		waitlist.ID.String(),
	)
	if _, err := repository.CreateWaitlistEntry(
		ctx,
		waitlistMetadata,
		waitlist,
		newEvent(
			waitlistMetadata,
			waitlist.ID.String(),
			"WaitlistCreated",
			map[string]any{"waitlist_id": waitlist.ID},
		),
	); err != nil {
		t.Fatal(err)
	}
	rawToken, tokenHash, err := codec.Issue()
	if err != nil {
		t.Fatal(err)
	}
	waitlistID := waitlist.ID
	token := domain.ActionToken{
		OrganizationID: organizationID,
		ID:             uuid.New(),
		WaitlistID:     &waitlistID,
		Purpose:        domain.ActionAcceptWaitlist,
		TokenHash:      tokenHash,
		ExpiresAt:      now.Add(time.Hour),
		CreatedAt:      now,
	}
	slot := domain.Slot{
		StartAt:       waitlistAt,
		EndAt:         waitlistAt.Add(time.Hour),
		OccupiesFrom:  waitlistAt,
		OccupiesUntil: waitlistAt.Add(time.Hour),
		Timezone:      "UTC",
		Allocations: []domain.Allocation{{
			ResourceID: resourceID,
			Mode:       domain.AllocationExclusive,
			Units:      1,
		}},
		Remaining: 1,
	}
	offered, err := repository.OfferWaitlist(
		ctx,
		organizationID,
		waitlist.ID,
		slot,
		now.Add(30*time.Minute),
		token,
		[]domain.Event{newEvent(
			waitlistMetadata,
			waitlist.ID.String(),
			domain.EventWaitlistOffered,
			map[string]any{"waitlist_id": waitlist.ID},
		)},
	)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(
		repository,
		algorithmsFake{slots: []domain.Slot{slot}},
		codec,
		WithClock(func() time.Time { return now }),
	)
	actionMetadata := testMetadata(
		"",
		"accept-waitlist-action-"+suffix,
		waitlist.ID.String(),
	)
	results := make(chan domain.WaitlistEntry, 2)
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, actionErr := service.ConsumeWaitlistAction(
				ctx,
				rawToken,
				actionMetadata,
				offered.Version,
			)
			results <- result
			errs <- actionErr
		}()
	}
	wait.Wait()
	close(results)
	close(errs)
	for actionErr := range errs {
		if actionErr != nil {
			t.Fatalf("concurrent waitlist action failed: %v", actionErr)
		}
	}
	var acceptedBookingID uuid.UUID
	for result := range results {
		if result.Status != domain.WaitlistAccepted ||
			result.Version != 3 ||
			result.AcceptedBookingID == nil {
			t.Fatalf("unexpected waitlist action result: %+v", result)
		}
		if acceptedBookingID == uuid.Nil {
			acceptedBookingID = *result.AcceptedBookingID
		} else if acceptedBookingID != *result.AcceptedBookingID {
			t.Fatalf(
				"concurrent waitlist actions created distinct bookings: %s and %s",
				acceptedBookingID,
				*result.AcceptedBookingID,
			)
		}
	}
	var bookingCount, acceptedEventCount, acceptedAuditCount int
	tx, err := repositoryhelpers.BeginTenant(ctx, pool, organizationID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM app.scheduling_bookings
		WHERE org_id=$1 AND starts_at=$2`,
		organizationID,
		waitlistAt,
	).Scan(&bookingCount); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM app.scheduling_events
		WHERE org_id=$1 AND aggregate_id=$2 AND event_type='WaitlistAccepted'`,
		organizationID,
		waitlist.ID.String(),
	).Scan(&acceptedEventCount); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM app.scheduling_audit
		WHERE org_id=$1 AND aggregate_id=$2
		  AND action='scheduling.waitlist.accepted'`,
		organizationID,
		waitlist.ID.String(),
	).Scan(&acceptedAuditCount); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if bookingCount != 1 ||
		acceptedEventCount != 1 ||
		acceptedAuditCount != 1 {
		t.Fatalf(
			"booking=%d event=%d audit=%d",
			bookingCount,
			acceptedEventCount,
			acceptedAuditCount,
		)
	}
}

func assertPublicActionEffects(
	t *testing.T,
	pool *pgxpool.Pool,
	organizationID string,
	bookingID uuid.UUID,
	eventType string,
	correlationID string,
	want int,
) {
	t.Helper()
	ctx := context.Background()
	tx, err := repositoryhelpers.BeginTenant(ctx, pool, organizationID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var events, audits, outbox int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM app.scheduling_events
		WHERE org_id=$1 AND aggregate_id=$2
		  AND event_type=$3 AND correlation_id=$4`,
		organizationID,
		bookingID.String(),
		eventType,
		correlationID,
	).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM app.scheduling_audit
		WHERE org_id=$1 AND aggregate_id=$2
		  AND action=$3 AND correlation_id=$4`,
		organizationID,
		bookingID.String(),
		"scheduling.booking."+string(domain.BookingConfirmed),
		correlationID,
	).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM app.outbox
		WHERE org_id=$1 AND correlation_id=$2`,
		organizationID,
		correlationID,
	).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if events != want || audits != want || outbox != 2*want {
		t.Fatalf(
			"public action effects events=%d audits=%d outbox=%d want=%d/%d/%d",
			events,
			audits,
			outbox,
			want,
			want,
			2*want,
		)
	}
}

func installPublicActionTokenFailure(
	t *testing.T,
	pool *pgxpool.Pool,
	organizationID string,
	tokenHash string,
) func() {
	t.Helper()
	ctx := context.Background()
	identifier := uuid.New()
	suffix := hex.EncodeToString(identifier[:])
	functionName := "test_fail_public_token_" + suffix
	triggerName := "test_fail_public_token_" + suffix
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE FUNCTION app.%s() RETURNS trigger
		LANGUAGE plpgsql
		AS $$
		BEGIN
		  IF NEW.org_id = '%s'
		     AND NEW.token_hash = '%s'
		     AND OLD.consumed_at IS NULL
		     AND NEW.consumed_at IS NOT NULL THEN
		    RAISE EXCEPTION 'injected public action token failure';
		  END IF;
		  RETURN NEW;
		END
		$$`,
		functionName,
		organizationID,
		tokenHash,
	)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TRIGGER %s
		BEFORE UPDATE ON app.scheduling_action_tokens
		FOR EACH ROW EXECUTE FUNCTION app.%s()`,
		triggerName,
		functionName,
	)); err != nil {
		_, _ = pool.Exec(
			ctx,
			fmt.Sprintf("DROP FUNCTION IF EXISTS app.%s()", functionName),
		)
		t.Fatal(err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if _, err := pool.Exec(
				ctx,
				fmt.Sprintf(
					"DROP TRIGGER IF EXISTS %s ON app.scheduling_action_tokens",
					triggerName,
				),
			); err != nil {
				t.Errorf("drop fault trigger: %v", err)
			}
			if _, err := pool.Exec(
				ctx,
				fmt.Sprintf("DROP FUNCTION IF EXISTS app.%s()", functionName),
			); err != nil {
				t.Errorf("drop fault function: %v", err)
			}
		})
	}
}

func assertSchedulingEventOwnership(
	t *testing.T,
	pool *pgxpool.Pool,
	organizationID string,
	bookingID uuid.UUID,
) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		t.Fatal(err)
	}
	var lifecycle int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM app.scheduling_events
		WHERE org_id=$1 AND aggregate_id=$2 AND event_type='BookingCreated'`,
		organizationID, bookingID.String(),
	).Scan(&lifecycle); err != nil {
		t.Fatal(err)
	}
	var integration, stolen int
	if err := tx.QueryRow(ctx, `
		SELECT
		  count(*) FILTER (WHERE topic IN ('NotificationRequested','CalendarSyncRequested')),
		  count(*) FILTER (WHERE topic='BookingCreated')
		FROM app.outbox
		WHERE org_id=$1 AND payload->>'booking_id'=$2`,
		organizationID, bookingID.String(),
	).Scan(&integration, &stolen); err != nil {
		t.Fatal(err)
	}
	if lifecycle != 1 || integration != 2 || stolen != 0 {
		t.Fatalf(
			"event ownership lifecycle=%d integration=%d stolen=%d",
			lifecycle, integration, stolen,
		)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func seedSchedulingTenant(
	t *testing.T,
	pool *pgxpool.Pool,
	organizationID, partyID string,
	branchID, serviceID, resourceID, roomID uuid.UUID,
) {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.organizations (id,name,slug,status)
		VALUES ($1,$2,$3,'ready')`,
		organizationID, "Scheduling "+suffix, "scheduling-"+suffix,
	); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.parties (org_id,id,kind,display_name)
		VALUES ($1,$2,'customer','Scheduling Customer')`,
		organizationID, partyID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.scheduling_branches (
			org_id,id,code,slug,name,timezone,address,active
		) VALUES ($1,$2,'main','main','Main','UTC','',true)`,
		organizationID, branchID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.scheduling_services (
			org_id,id,code,name,duration_minutes,slot_minutes,price,currency,
			fulfillment_mode,max_participants,allow_group,active
		) VALUES ($1,$2,'service','Service',60,30,100,'ARS','in_person',10,true,true)`,
		organizationID, serviceID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.scheduling_resources (
			org_id,id,branch_id,code,name,kind,capacity,timezone,active
		) VALUES ($1,$2,$3,'professional','Professional','professional',1,'UTC',true)`,
		organizationID, resourceID, branchID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.scheduling_resources (
			org_id,id,branch_id,code,name,kind,capacity,timezone,active
		) VALUES ($1,$2,$3,'room','Room','room',1,'UTC',true)`,
		organizationID, roomID, branchID,
	); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func testBooking(
	organizationID string,
	id, branchID, serviceID uuid.UUID,
	partyID string,
	resourceID uuid.UUID,
	startAt time.Time,
	status domain.BookingStatus,
) domain.Booking {
	return domain.Booking{
		OrganizationID: organizationID, ID: id, BranchID: branchID,
		ServiceID: serviceID, PartyID: partyID, Status: status, Participants: 1,
		StartAt: startAt, EndAt: startAt.Add(time.Hour),
		OccupiesFrom: startAt, OccupiesUntil: startAt.Add(time.Hour),
		Version: 1, ServiceName: "Service", Price: "100", Currency: "ARS",
		DurationMinutes: 60, Timezone: "UTC",
		Allocations: []domain.Allocation{{
			ResourceID: resourceID, Mode: domain.AllocationExclusive, Units: 1,
		}},
		CreatedBy: "test", CreatedAt: startAt.Add(-time.Hour), UpdatedAt: startAt.Add(-time.Hour),
	}
}

func testMetadata(organizationID, key, sourceID string) domain.CommandMetadata {
	digest := sha256.Sum256([]byte(key))
	return domain.CommandMetadata{
		OrganizationID: organizationID, IdempotencyKey: key, SourceID: sourceID,
		SourceVersion: 1, PayloadHash: hex.EncodeToString(digest[:]),
		RequestID: "request:" + key, CorrelationID: "correlation:" + key, ActorID: "test:actor",
	}
}
