package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	projectionmodels "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/calendar_projection/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
)

func Event(
	metadata domain.CommandMetadata,
	booking domain.Booking,
	operation string,
	eventID uuid.UUID,
	now time.Time,
) domain.Event {
	snapshot := Snapshot(booking, operation)
	snapshotPayload, _ := json.Marshal(snapshot)
	snapshotHash := sha256.Sum256(snapshotPayload)
	snapshotDigest := hex.EncodeToString(snapshotHash[:])

	request := projectionmodels.CalendarSyncRequested{
		SchemaVersion:  1,
		CommandID:      eventID.String(),
		BookingID:      booking.ID.String(),
		Operation:      operation,
		SourceVersion:  booking.Version,
		SnapshotDigest: snapshotDigest,
		CorrelationID:  metadata.CorrelationID,
		Summary:        snapshot.Summary,
		Description:    snapshot.Description,
		Location:       snapshot.Location,
		Start:          snapshot.Start,
		End:            snapshot.End,
		TimeZone:       snapshot.TimeZone,
		AttendeeEmails: append([]string(nil), snapshot.AttendeeEmails...),
		MeetRequested:  snapshot.MeetRequested,
	}
	payload, _ := json.Marshal(request)
	payloadHash := sha256.Sum256(payload)
	return domain.Event{
		ID:             eventID,
		OrganizationID: metadata.OrganizationID,
		Type:           domain.EventCalendarSyncRequested,
		AggregateID:    booking.ID.String(),
		Payload:        payload,
		PayloadHash:    hex.EncodeToString(payloadHash[:]),
		IdempotencyKey: fmt.Sprintf(
			"scheduling:%s:%s:%s:%d",
			domain.EventCalendarSyncRequested,
			booking.ID,
			operation,
			booking.Version,
		),
		RequestID:     metadata.RequestID,
		CorrelationID: metadata.CorrelationID,
		ActorID:       metadata.ActorID,
		SourceVersion: booking.Version,
		AvailableAt:   now.UTC(),
	}
}

func Snapshot(
	booking domain.Booking,
	operation string,
) projectionmodels.CalendarSnapshot {
	result := projectionmodels.CalendarSnapshot{
		SchemaVersion: 1,
		BookingID:     booking.ID.String(),
		Operation:     operation,
		SourceVersion: booking.Version,
	}
	if operation != "upsert" {
		return result
	}
	result.Summary = booking.ServiceName
	result.Start = booking.StartAt.UTC().Format(time.RFC3339Nano)
	result.End = booking.EndAt.UTC().Format(time.RFC3339Nano)
	result.TimeZone = booking.Timezone
	result.MeetRequested = booking.MeetRequested
	return result
}
