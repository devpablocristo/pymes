package helpers

import (
	"encoding/json"
	"strings"
	"testing"
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
	valid := map[string]any{
		"command_id": "cmd", "booking_id": "booking",
		"operation": "upsert", "source_version": 1,
		"snapshot_digest": strings.Repeat("a", 64),
		"correlation_id":  "correlation", "summary": "Turno",
		"start": "2026-08-01T10:00:00Z",
		"end":   "2026-08-01T11:00:00Z", "time_zone": "UTC",
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
}
