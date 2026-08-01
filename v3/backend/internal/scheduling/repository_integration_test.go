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
