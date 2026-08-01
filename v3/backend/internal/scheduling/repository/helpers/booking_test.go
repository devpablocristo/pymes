package helpers

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
)

func TestBookingResponseUsesStableJSONAndRoundTripsDomain(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	seriesID, sessionID, previousID := uuid.New(), uuid.New(), uuid.New()
	holdExpiresAt := now.Add(15 * time.Minute)
	original := domain.Booking{
		OrganizationID:     "org-roundtrip",
		ID:                 uuid.New(),
		SeriesID:           &seriesID,
		SessionID:          &sessionID,
		SupersedesID:       &previousID,
		Occurrence:         3,
		BranchID:           uuid.New(),
		ServiceID:          uuid.New(),
		PartyID:            "party-roundtrip",
		Status:             domain.BookingHeld,
		SubstateCode:       "awaiting_customer",
		Participants:       2,
		StartAt:            now.Add(time.Hour),
		EndAt:              now.Add(2 * time.Hour),
		OccupiesFrom:       now.Add(45 * time.Minute),
		OccupiesUntil:      now.Add(135 * time.Minute),
		HoldExpiresAt:      &holdExpiresAt,
		Version:            7,
		ServiceName:        "Consulta",
		Price:              "1250.00",
		Currency:           "ARS",
		DurationMinutes:    60,
		Timezone:           "America/Argentina/Buenos_Aires",
		CustomerName:       "Ada",
		CustomerEmail:      "ada@example.com",
		CustomerPhone:      "+541155555555",
		Notes:              "Nota",
		CancellationReason: "",
		Allocations: []domain.Allocation{{
			ResourceID: uuid.New(),
			Mode:       domain.AllocationCapacity,
			Units:      2,
		}},
		CreatedBy: "actor",
		CreatedAt: now,
		UpdatedAt: now.Add(time.Minute),
	}
	model := BookingResponseFromDomain(original)
	encoded, err := Encode(model)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"organization_id":"org-roundtrip"`)) ||
		!bytes.Contains(encoded, []byte(`"duration_minutes":60`)) ||
		bytes.Contains(encoded, []byte(`"OrganizationID"`)) ||
		bytes.Contains(encoded, []byte(`"DurationMinutes"`)) {
		t.Fatalf("unstable booking response JSON: %s", encoded)
	}
	var decoded repositorymodels.BookingResponse
	if err := Decode(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	roundTrip := BookingResponseToDomain(decoded)
	if !reflect.DeepEqual(roundTrip, original) {
		t.Fatalf("round trip mismatch:\noriginal=%+v\nround_trip=%+v", original, roundTrip)
	}
}
