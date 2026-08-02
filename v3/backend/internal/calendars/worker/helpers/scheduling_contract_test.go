package helpers_test

import (
	"testing"
	"time"

	calendarshelpers "github.com/devpablocristo/pymes/v3/backend/internal/calendars/worker/helpers"
	scheduling "github.com/devpablocristo/pymes/v3/backend/internal/scheduling"
	schedulingdomain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
)

func TestSchedulingCalendarProjectionMatchesCalendarConsumer(t *testing.T) {
	t.Parallel()
	metadata := schedulingdomain.CommandMetadata{
		OrganizationID: "org-contract",
		IdempotencyKey: "booking-contract",
		SourceID:       "booking-contract",
		SourceVersion:  99,
		PayloadHash:    string(make([]byte, 64)),
		RequestID:      "request-contract",
		CorrelationID:  "correlation-contract",
		ActorID:        "actor-contract",
	}
	booking := schedulingdomain.Booking{
		OrganizationID: metadata.OrganizationID,
		ID:             uuid.New(),
		Status:         schedulingdomain.BookingConfirmed,
		Version:        3,
		ServiceName:    "Consulta virtual",
		StartAt:        time.Date(2026, 8, 3, 15, 0, 0, 0, time.UTC),
		EndAt:          time.Date(2026, 8, 3, 16, 0, 0, 0, time.UTC),
		Timezone:       "America/Argentina/Buenos_Aires",
		MeetRequested:  true,
		CustomerEmail:  "customer@example.com",
	}
	projection := scheduling.NewCalendarProjectionAdapter()

	upsert := projection.Upsert(metadata, booking)
	upsertCommand, err := calendarshelpers.DecodeSyncRequested(
		metadata.OrganizationID,
		upsert.Payload,
	)
	if err != nil {
		t.Fatalf("decode scheduling upsert: %v", err)
	}
	if upsertCommand.CommandID != upsert.ID.String() ||
		upsertCommand.SourceVersion != booking.Version ||
		!upsertCommand.MeetRequested ||
		len(upsertCommand.AttendeeEmails) != 0 {
		t.Fatalf("unexpected upsert command: %+v", upsertCommand)
	}

	booking.Status = schedulingdomain.BookingCancelled
	booking.Version++
	deletion := projection.Delete(metadata, booking)
	deleteCommand, err := calendarshelpers.DecodeSyncRequested(
		metadata.OrganizationID,
		deletion.Payload,
	)
	if err != nil {
		t.Fatalf("decode scheduling delete: %v", err)
	}
	if deleteCommand.CommandID != deletion.ID.String() ||
		deleteCommand.SourceVersion != booking.Version ||
		!deleteCommand.Start.IsZero() ||
		deleteCommand.MeetRequested {
		t.Fatalf("unexpected delete command: %+v", deleteCommand)
	}
}
