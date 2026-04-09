package scheduling

import (
	"testing"
	"time"

	"github.com/google/uuid"

	corescheduling "github.com/devpablocristo/core/scheduling/go"
	schedulingdomain "github.com/devpablocristo/modules/scheduling/go/domain"
)

func TestCanTransitionBooking(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from schedulingdomain.BookingStatus
		to   schedulingdomain.BookingStatus
		want bool
	}{
		{name: "hold to confirmed", from: schedulingdomain.BookingStatusHold, to: schedulingdomain.BookingStatusConfirmed, want: true},
		{name: "pending to completed invalid", from: schedulingdomain.BookingStatusPendingConfirmation, to: schedulingdomain.BookingStatusCompleted, want: false},
		{name: "confirmed to checked_in", from: schedulingdomain.BookingStatusConfirmed, to: schedulingdomain.BookingStatusCheckedIn, want: true},
		{name: "completed to confirmed invalid", from: schedulingdomain.BookingStatusCompleted, to: schedulingdomain.BookingStatusConfirmed, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := canTransitionBooking(tt.from, tt.to); got != tt.want {
				t.Fatalf("canTransitionBooking(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestCanTransitionQueueTicket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from schedulingdomain.QueueTicketStatus
		to   schedulingdomain.QueueTicketStatus
		want bool
	}{
		{name: "waiting to called", from: schedulingdomain.QueueTicketStatusWaiting, to: schedulingdomain.QueueTicketStatusCalled, want: true},
		{name: "called to completed invalid", from: schedulingdomain.QueueTicketStatusCalled, to: schedulingdomain.QueueTicketStatusCompleted, want: false},
		{name: "serving to completed", from: schedulingdomain.QueueTicketStatusServing, to: schedulingdomain.QueueTicketStatusCompleted, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := canTransitionQueueTicket(tt.from, tt.to); got != tt.want {
				t.Fatalf("canTransitionQueueTicket(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestGenerateSlotsForResourceAppliesIntersectionAndBlocks(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("America/Argentina/Tucuman")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	day := time.Date(2026, 4, 6, 0, 0, 0, 0, loc) // Monday
	branch := schedulingdomain.Branch{ID: uuid.New(), Timezone: "America/Argentina/Tucuman"}
	resource := schedulingdomain.Resource{ID: uuid.New(), Name: "Profesional Demo"}
	service := schedulingdomain.Service{
		ID:                     uuid.New(),
		Code:                   "consulta",
		DefaultDurationMinutes: 30,
		BufferBeforeMinutes:    0,
		BufferAfterMinutes:     0,
		SlotGranularityMinutes: 30,
	}
	rules := []schedulingdomain.AvailabilityRule{
		{
			Kind:                   schedulingdomain.AvailabilityRuleKindBranch,
			Weekday:                1,
			StartTime:              "09:00",
			EndTime:                "12:00",
			SlotGranularityMinutes: intPtr(30),
		},
		{
			Kind:                   schedulingdomain.AvailabilityRuleKindResource,
			Weekday:                1,
			StartTime:              "10:00",
			EndTime:                "12:00",
			SlotGranularityMinutes: intPtr(30),
		},
	}
	blocked := []schedulingdomain.BlockedRange{
		{
			StartAt: time.Date(2026, 4, 6, 10, 30, 0, 0, loc).UTC(),
			EndAt:   time.Date(2026, 4, 6, 11, 0, 0, 0, loc).UTC(),
		},
	}

	slots := generateSlotsForResource(loc, branch, resource, service, day, rules, blocked)

	if len(slots) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(slots))
	}
	if slots[0].StartAt.In(loc).Format("15:04") != "10:00" {
		t.Fatalf("expected first slot at 10:00, got %s", slots[0].StartAt.In(loc).Format("15:04"))
	}
	if slots[1].StartAt.In(loc).Format("15:04") != "11:00" {
		t.Fatalf("expected second slot at 11:00, got %s", slots[1].StartAt.In(loc).Format("15:04"))
	}
	if slots[2].StartAt.In(loc).Format("15:04") != "11:30" {
		t.Fatalf("expected third slot at 11:30, got %s", slots[2].StartAt.In(loc).Format("15:04"))
	}
}

func TestExpandRecurringBookingStartsWeekly(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("America/Argentina/Tucuman")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	startAt := time.Date(2026, 4, 6, 10, 0, 0, 0, loc) // Monday

	items, err := expandRecurringBookingStarts(startAt.UTC(), &schedulingdomain.BookingRecurrence{
		Freq:      "weekly",
		Interval:  1,
		Count:     4,
		ByWeekday: []int{1, 3},
	}, "America/Argentina/Tucuman")
	if err != nil {
		t.Fatalf("expand recurring weekly: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 occurrences, got %d", len(items))
	}

	got := []string{
		items[0].In(loc).Format("2006-01-02 15:04"),
		items[1].In(loc).Format("2006-01-02 15:04"),
		items[2].In(loc).Format("2006-01-02 15:04"),
		items[3].In(loc).Format("2006-01-02 15:04"),
	}
	want := []string{
		"2026-04-06 10:00",
		"2026-04-08 10:00",
		"2026-04-13 10:00",
		"2026-04-15 10:00",
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("occurrence %d = %s, want %s", index, got[index], want[index])
		}
	}
}

func TestAddMonthsPreservingDayClampsToMonthEnd(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 1, 31, 9, 30, 0, 0, time.UTC)
	got := addMonthsPreservingDay(base, 1)
	if got.Format("2006-01-02 15:04") != "2026-02-28 09:30" {
		t.Fatalf("addMonthsPreservingDay() = %s, want %s", got.Format("2006-01-02 15:04"), "2026-02-28 09:30")
	}
}

func intPtr(v int) *int { return &v }

func TestRangeFitsAnyWindow(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("America/Argentina/Tucuman")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	day := time.Date(2026, 4, 6, 0, 0, 0, 0, loc)
	morning := corescheduling.Window{
		Start: time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, loc),
		End:   time.Date(day.Year(), day.Month(), day.Day(), 13, 0, 0, 0, loc),
	}
	afternoon := corescheduling.Window{
		Start: time.Date(day.Year(), day.Month(), day.Day(), 15, 0, 0, 0, loc),
		End:   time.Date(day.Year(), day.Month(), day.Day(), 19, 0, 0, 0, loc),
	}
	windows := []corescheduling.Window{morning, afternoon}

	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		want  bool
	}{
		{
			name:  "fully inside morning",
			start: time.Date(2026, 4, 6, 10, 0, 0, 0, loc),
			end:   time.Date(2026, 4, 6, 11, 0, 0, 0, loc),
			want:  true,
		},
		{
			name:  "exactly matches afternoon",
			start: time.Date(2026, 4, 6, 15, 0, 0, 0, loc),
			end:   time.Date(2026, 4, 6, 19, 0, 0, 0, loc),
			want:  true,
		},
		{
			name:  "spans the lunch break",
			start: time.Date(2026, 4, 6, 12, 30, 0, 0, loc),
			end:   time.Date(2026, 4, 6, 16, 0, 0, 0, loc),
			want:  false,
		},
		{
			name:  "before any window",
			start: time.Date(2026, 4, 6, 7, 0, 0, 0, loc),
			end:   time.Date(2026, 4, 6, 8, 0, 0, 0, loc),
			want:  false,
		},
		{
			name:  "after any window",
			start: time.Date(2026, 4, 6, 20, 0, 0, 0, loc),
			end:   time.Date(2026, 4, 6, 21, 0, 0, 0, loc),
			want:  false,
		},
		{
			name:  "starts inside morning but ends past it",
			start: time.Date(2026, 4, 6, 12, 0, 0, 0, loc),
			end:   time.Date(2026, 4, 6, 13, 30, 0, 0, loc),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := rangeFitsAnyWindow(tt.start, tt.end, windows); got != tt.want {
				t.Fatalf("rangeFitsAnyWindow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateBlockedRangeFields(t *testing.T) {
	t.Parallel()

	branchID := uuid.New()
	start := time.Date(2026, 4, 6, 14, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 6, 16, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		in      schedulingdomain.BlockedRange
		wantErr bool
		wantKind schedulingdomain.BlockedRangeKind
	}{
		{
			name:     "happy path manual",
			in:       schedulingdomain.BlockedRange{BranchID: branchID, Kind: schedulingdomain.BlockedRangeKindManual, StartAt: start, EndAt: end},
			wantKind: schedulingdomain.BlockedRangeKindManual,
		},
		{
			name:     "kind is normalized from upper case",
			in:       schedulingdomain.BlockedRange{BranchID: branchID, Kind: "HOLIDAY", StartAt: start, EndAt: end},
			wantKind: schedulingdomain.BlockedRangeKindHoliday,
		},
		{
			name:    "missing branch_id",
			in:      schedulingdomain.BlockedRange{Kind: schedulingdomain.BlockedRangeKindManual, StartAt: start, EndAt: end},
			wantErr: true,
		},
		{
			name:    "invalid kind",
			in:      schedulingdomain.BlockedRange{BranchID: branchID, Kind: "made-up", StartAt: start, EndAt: end},
			wantErr: true,
		},
		{
			name:    "missing start_at",
			in:      schedulingdomain.BlockedRange{BranchID: branchID, Kind: schedulingdomain.BlockedRangeKindManual, EndAt: end},
			wantErr: true,
		},
		{
			name:    "missing end_at",
			in:      schedulingdomain.BlockedRange{BranchID: branchID, Kind: schedulingdomain.BlockedRangeKindManual, StartAt: start},
			wantErr: true,
		},
		{
			name:    "end before start",
			in:      schedulingdomain.BlockedRange{BranchID: branchID, Kind: schedulingdomain.BlockedRangeKindManual, StartAt: end, EndAt: start},
			wantErr: true,
		},
		{
			name:    "end equal to start",
			in:      schedulingdomain.BlockedRange{BranchID: branchID, Kind: schedulingdomain.BlockedRangeKindManual, StartAt: start, EndAt: start},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kind, err := validateBlockedRangeFields(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if kind != tt.wantKind {
				t.Fatalf("kind = %q, want %q", kind, tt.wantKind)
			}
		})
	}
}
