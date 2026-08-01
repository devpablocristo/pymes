package helpers

import "testing"

func TestTraceIDRoundTripsTenantAndNotification(t *testing.T) {
	traceID, err := TraceID("org:argentina-1", "notification/2026-1")
	if err != nil {
		t.Fatal(err)
	}
	organizationID, notificationID, err := ParseTraceID(traceID)
	if err != nil {
		t.Fatal(err)
	}
	if organizationID != "org:argentina-1" ||
		notificationID != "notification/2026-1" {
		t.Fatalf(
			"round trip = %q %q",
			organizationID,
			notificationID,
		)
	}
}

func TestParseTraceIDRejectsUnscopedOrMalformedIdentity(t *testing.T) {
	for _, traceID := range []string{
		"notification-1",
		"pymes.v1.",
		"pymes.v1.bad",
		"pymes.v1.!.!",
	} {
		if _, _, err := ParseTraceID(traceID); err == nil {
			t.Fatalf("ParseTraceID(%q) accepted malformed identity", traceID)
		}
	}
}
