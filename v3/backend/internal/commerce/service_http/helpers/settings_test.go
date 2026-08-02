package helpers

import (
	"testing"
	"time"
)

func TestDefaultSettingsPreserveBoundedClientPolicy(t *testing.T) {
	settings := DefaultSettings()
	if settings.FailureThreshold != 5 {
		t.Fatalf("FailureThreshold = %d", settings.FailureThreshold)
	}
	if settings.OpenFor != 15*time.Second {
		t.Fatalf("OpenFor = %s", settings.OpenFor)
	}
	if settings.RequestTimeout != 10*time.Second {
		t.Fatalf("RequestTimeout = %s", settings.RequestTimeout)
	}
}

func TestCircuitOpenHonorsCooldownBoundary(t *testing.T) {
	openedAt := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)
	if !CircuitOpen(openedAt, openedAt.Add(14*time.Second), 15*time.Second) {
		t.Fatal("circuit closed before cooldown elapsed")
	}
	if CircuitOpen(openedAt, openedAt.Add(15*time.Second), 15*time.Second) {
		t.Fatal("circuit remained open at cooldown boundary")
	}
	if CircuitOpen(time.Time{}, openedAt, 15*time.Second) {
		t.Fatal("zero opening time reported an open circuit")
	}
}
