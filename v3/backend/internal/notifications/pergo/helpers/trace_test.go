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

func TestIngressIdempotencyKeyNamespacesTenantLocalIdentity(t *testing.T) {
	first, err := IngressIdempotencyKey("org-a", "booking:confirmation:1")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := IngressIdempotencyKey("org-a", "booking:confirmation:1")
	if err != nil {
		t.Fatal(err)
	}
	otherTenant, err := IngressIdempotencyKey(
		"org-b",
		"booking:confirmation:1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if first != replay || first == otherTenant || len(first) > 255 {
		t.Fatalf(
			"tenant namespace failed: first=%q replay=%q other=%q",
			first,
			replay,
			otherTenant,
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
