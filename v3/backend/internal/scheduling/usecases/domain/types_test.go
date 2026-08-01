package domain

import (
	"testing"
	"time"
)

func TestBookingTransitionsPreserveInternalInvariants(t *testing.T) {
	tests := []struct {
		from BookingStatus
		to   BookingStatus
		want bool
	}{
		{BookingHeld, BookingConfirmed, true},
		{BookingHeld, BookingCompleted, false},
		{BookingPendingConfirmation, BookingConfirmed, true},
		{BookingConfirmed, BookingCheckedIn, true},
		{BookingConfirmed, BookingNoShow, true},
		{BookingCheckedIn, BookingCompleted, true},
		{BookingCompleted, BookingConfirmed, false},
		{BookingCancelled, BookingConfirmed, false},
		{BookingRescheduled, BookingConfirmed, false},
	}
	for _, test := range tests {
		if got := test.from.CanTransition(test.to); got != test.want {
			t.Errorf("%s -> %s = %v, want %v", test.from, test.to, got, test.want)
		}
	}
	for _, status := range []BookingStatus{
		BookingHeld, BookingPendingConfirmation, BookingConfirmed, BookingCheckedIn,
	} {
		if !status.Active() {
			t.Errorf("%s should reserve capacity", status)
		}
	}
	for _, status := range []BookingStatus{
		BookingCompleted, BookingCancelled, BookingRescheduled, BookingNoShow,
	} {
		if status.Active() {
			t.Errorf("%s must release capacity", status)
		}
	}
}

func TestBookingStatusConfigurationCannotRedefineInternalStates(t *testing.T) {
	configuration := BookingStatusConfiguration{
		OrganizationID: "org-a",
		Status:         BookingConfirmed,
		Label:          "Agendado",
		Substates: []BookingSubstateDefinition{
			{Code: "first_visit", Label: "Primera visita", Active: true, SortOrder: 10},
			{Code: "vip", Label: "Prioridad", Active: true, SortOrder: 20},
		},
	}
	if err := configuration.Validate(); err != nil {
		t.Fatalf("valid customization rejected: %v", err)
	}
	configuration.Status = BookingStatus("custom_confirmed")
	if err := configuration.Validate(); ErrorCodeOf(err) != CodeValidation {
		t.Fatalf("custom internal state was accepted: %v", err)
	}
	configuration.Status = BookingConfirmed
	configuration.Substates[1].Code = configuration.Substates[0].Code
	if err := configuration.Validate(); ErrorCodeOf(err) != CodeValidation {
		t.Fatalf("duplicate substate code was accepted: %v", err)
	}
}

func TestExpandRecurrenceKeepsWallClockAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.March, 7, 9, 30, 0, 0, location)
	values, err := ExpandRecurrence(start, location.String(), RecurrenceRule{
		Frequency: RecurrenceDaily,
		Interval:  1,
		Count:     3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 3 {
		t.Fatalf("occurrences=%d", len(values))
	}
	for _, value := range values {
		local := value.In(location)
		if local.Hour() != 9 || local.Minute() != 30 {
			t.Fatalf("wall clock moved across DST: %s", local)
		}
	}
	if values[1].Sub(values[0]) != 23*time.Hour {
		t.Fatalf("spring DST interval=%s, want 23h", values[1].Sub(values[0]))
	}
}

func TestExpandWeeklyRecurrenceDeduplicatesWeekdays(t *testing.T) {
	start := time.Date(2026, time.August, 3, 10, 0, 0, 0, time.UTC)
	values, err := ExpandRecurrence(start, "UTC", RecurrenceRule{
		Frequency:  RecurrenceWeekly,
		Interval:   1,
		Count:      4,
		ByWeekdays: []time.Weekday{time.Monday, time.Wednesday, time.Monday},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Weekday{time.Monday, time.Wednesday, time.Monday, time.Wednesday}
	for index, value := range values {
		if value.Weekday() != want[index] {
			t.Fatalf("occurrence %d weekday=%s want=%s", index, value.Weekday(), want[index])
		}
	}
}
