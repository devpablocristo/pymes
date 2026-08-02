package helpers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
)

func TestDeterministicIDsAreGoogleBase32HexAndIndependent(t *testing.T) {
	t.Parallel()
	eventID := EventID("org", "connection", "booking")
	meetID := MeetRequestID("org", "connection", "booking")
	if eventID == meetID || eventID != EventID("org", "connection", "booking") {
		t.Fatalf("event=%q meet=%q", eventID, meetID)
	}
	if len(eventID) != 52 {
		t.Fatalf("event id length = %d", len(eventID))
	}
	for _, character := range eventID {
		if !strings.ContainsRune("0123456789abcdefghijklmnopqrstuv", character) {
			t.Fatalf("event id contains %q", character)
		}
	}
}

func TestDecodeSyncRequestedRejectsUnknownAndInvalidFields(t *testing.T) {
	t.Parallel()
	command := domain.CalendarSyncCommand{
		CommandID: "cmd", OrganizationID: "org", BookingID: "booking",
		Operation: domain.SyncUpsert, SourceVersion: 1,
		CorrelationID: "correlation", Summary: "Turno",
		Start:    time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC),
		TimeZone: "UTC",
	}
	valid := map[string]any{
		"schema_version": 1,
		"command_id":     command.CommandID, "booking_id": command.BookingID,
		"operation": string(command.Operation), "source_version": command.SourceVersion,
		"snapshot_digest": SnapshotDigest(command),
		"correlation_id":  command.CorrelationID, "summary": command.Summary,
		"start": command.Start.Format(time.RFC3339),
		"end":   command.End.Format(time.RFC3339), "time_zone": command.TimeZone,
	}
	payload, _ := json.Marshal(valid)
	if _, err := DecodeSyncRequested("org", payload); err != nil {
		t.Fatal(err)
	}
	valid["unexpected"] = "forbidden"
	payload, _ = json.Marshal(valid)
	if _, err := DecodeSyncRequested("org", payload); err == nil {
		t.Fatal("unknown provider payload field was accepted")
	}
	delete(valid, "unexpected")
	valid["schema_version"] = 2
	payload, _ = json.Marshal(valid)
	if _, err := DecodeSyncRequested("org", payload); err == nil {
		t.Fatal("unsupported calendar schema version was accepted")
	}
}

func TestDecodeSyncRequestedRejectsTamperedAndNonHexDigest(t *testing.T) {
	t.Parallel()
	command := domain.CalendarSyncCommand{
		CommandID: "cmd", OrganizationID: "org", BookingID: "booking",
		Operation: domain.SyncDelete, SourceVersion: 2,
		CorrelationID: "correlation",
	}
	payload := map[string]any{
		"schema_version": 1,
		"command_id":     command.CommandID, "booking_id": command.BookingID,
		"operation": string(command.Operation), "source_version": command.SourceVersion,
		"snapshot_digest": SnapshotDigest(command),
		"correlation_id":  command.CorrelationID,
	}
	payload["snapshot_digest"] = strings.Repeat("g", 64)
	encoded, _ := json.Marshal(payload)
	if _, err := DecodeSyncRequested("org", encoded); err == nil {
		t.Fatal("non-hex snapshot digest was accepted")
	}
	payload["snapshot_digest"] = strings.Repeat("a", 64)
	encoded, _ = json.Marshal(payload)
	if _, err := DecodeSyncRequested("org", encoded); err == nil {
		t.Fatal("mismatched snapshot digest was accepted")
	}
}
