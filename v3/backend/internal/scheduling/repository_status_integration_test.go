package scheduling

import (
	"context"
	"os"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBookingStatusCustomizationPreservesTenantAndLifecycleInvariants(t *testing.T) {
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
	organizationID := "org_sched_status_" + suffix
	partyID := "party_sched_status_" + suffix
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
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	repository.now = func() time.Time { return now }
	configuration := domain.BookingStatusConfiguration{
		OrganizationID: organizationID,
		Status:         domain.BookingConfirmed,
		Label:          "Agendado",
		Substates: []domain.BookingSubstateDefinition{{
			Code: "first_visit", Label: "Primera visita", Active: true, SortOrder: 10,
		}},
	}
	configMetadata := testMetadata(
		organizationID,
		"configure-status-"+suffix,
		string(configuration.Status),
	)
	configured, err := repository.ConfigureBookingStatus(ctx, configMetadata, configuration)
	if err != nil {
		t.Fatal(err)
	}
	if configured.Label != "Agendado" || len(configured.Substates) != 1 {
		t.Fatalf("configuration=%+v", configured)
	}
	replayed, err := repository.ConfigureBookingStatus(ctx, configMetadata, configuration)
	if err != nil || replayed.UpdatedAt != configured.UpdatedAt {
		t.Fatalf("idempotent configuration replay=%+v err=%v", replayed, err)
	}
	configurations, err := repository.ListBookingStatusConfigurations(ctx, organizationID)
	if err != nil || len(configurations) != 1 || len(configurations[0].Substates) != 1 {
		t.Fatalf("configurations=%+v err=%v", configurations, err)
	}

	booking := testBooking(
		organizationID,
		uuid.New(),
		branchID,
		serviceID,
		partyID,
		resourceID,
		now.Add(24*time.Hour),
		domain.BookingConfirmed,
	)
	createMetadata := testMetadata(organizationID, "create-status-booking-"+suffix, booking.ID.String())
	if _, err := repository.ReserveBookings(
		ctx,
		createMetadata,
		nil,
		[]domain.Booking{booking},
		nil,
		nil,
	); err != nil {
		t.Fatal(err)
	}
	substateMetadata := testMetadata(
		organizationID,
		"set-substate-"+suffix,
		booking.ID.String(),
	)
	withSubstate, err := repository.SetBookingSubstate(
		ctx,
		substateMetadata,
		booking.ID,
		1,
		"first_visit",
	)
	if err != nil || withSubstate.SubstateCode != "first_visit" || withSubstate.Version != 2 {
		t.Fatalf("booking=%+v err=%v", withSubstate, err)
	}
	substateReplay, err := repository.SetBookingSubstate(
		ctx,
		substateMetadata,
		booking.ID,
		1,
		"first_visit",
	)
	if err != nil || substateReplay.ID != withSubstate.ID ||
		substateReplay.Version != withSubstate.Version {
		t.Fatalf("substate replay=%+v err=%v", substateReplay, err)
	}
	invalidSubstateMetadata := testMetadata(
		organizationID,
		"invalid-substate-"+suffix,
		booking.ID.String(),
	)
	invalidSubstateMetadata.SourceVersion = 2
	if _, err := repository.SetBookingSubstate(
		ctx,
		invalidSubstateMetadata,
		booking.ID,
		2,
		"not_configured",
	); domain.ErrorCodeOf(err) != domain.CodeBookingStateInvalid {
		t.Fatalf("unconfigured substate error=%v", err)
	}
	if _, err := repository.GetBooking(
		ctx,
		"org-other-"+suffix,
		booking.ID,
	); domain.ErrorCodeOf(err) != domain.CodeNotFound {
		t.Fatalf("cross-tenant booking read error=%v", err)
	}

	transitioned, err := repository.TransitionBooking(
		ctx,
		testMetadata(organizationID, "transition-status-"+suffix, booking.ID.String()),
		booking.ID,
		2,
		domain.BookingCheckedIn,
		"",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if transitioned.Status != domain.BookingCheckedIn || transitioned.SubstateCode != "" {
		t.Fatalf("internal transition retained an invalid custom substate: %+v", transitioned)
	}
}
