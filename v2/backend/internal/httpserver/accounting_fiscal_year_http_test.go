package httpserver

import (
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/google/uuid"
)

func TestAPIFiscalYearSummaryPreservesLegacyShape(t *testing.T) {
	t.Parallel()

	fiscalYear := accounting.FiscalYearSummary{
		ID:                uuid.New(),
		Code:              "2024",
		StartDate:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		IsLegacy:          true,
		State:             accounting.FiscalYearClosing,
		Version:           3,
		PeriodCounts:      accounting.FiscalYearPeriodCounts{SoftClosed: 1},
		AnnualCloseStatus: accounting.AnnualCloseReady,
	}

	response := apiFiscalYearSummary(fiscalYear)
	if !response.IsLegacy {
		t.Fatal("legacy fiscal year was exposed as a twelve-period fiscal year")
	}
	if response.Code != fiscalYear.Code || response.Version != fiscalYear.Version {
		t.Fatalf("summary = %+v", response)
	}
}

func TestAPIPeriodEventPreservesAuditVersions(t *testing.T) {
	t.Parallel()

	fromVersion := int64(4)
	event := accounting.PeriodAudit{
		ID:          uuid.New(),
		PeriodID:    uuid.New(),
		FromStatus:  accounting.PeriodOpen,
		ToStatus:    accounting.PeriodSoftClosed,
		FromVersion: &fromVersion,
		ToVersion:   5,
		ActorID:     "user_test",
		OccurredAt:  time.Now().UTC(),
	}

	response := apiPeriodEvent(event)
	if response.FromVersion == nil || *response.FromVersion != fromVersion {
		t.Fatalf("from version = %v", response.FromVersion)
	}
	if response.ToVersion != event.ToVersion {
		t.Fatalf("to version = %d", response.ToVersion)
	}
	if response.Reason != nil {
		t.Fatalf("empty reason must stay omitted, got %q", *response.Reason)
	}
}

func TestAPIFiscalYearDetailPreservesEventVersions(t *testing.T) {
	t.Parallel()

	fromVersion := int64(2)
	fiscalYearID := uuid.New()
	response := apiFiscalYearDetail(accounting.FiscalYearDetail{
		FiscalYearSummary: accounting.FiscalYearSummary{
			ID:                fiscalYearID,
			Code:              "2026",
			StartDate:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			EndDate:           time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			State:             accounting.FiscalYearClosing,
			Version:           3,
			PeriodCounts:      accounting.FiscalYearPeriodCounts{Locked: 11, SoftClosed: 1},
			AnnualCloseStatus: accounting.AnnualCloseReady,
		},
		RecentEvents: []accounting.FiscalYearEvent{{
			ID:           uuid.New(),
			FiscalYearID: fiscalYearID,
			EventType:    "annual_close_transition",
			FromStatus:   "not_ready",
			ToStatus:     "ready",
			FromVersion:  &fromVersion,
			ToVersion:    3,
			ActorID:      "user_test",
			OccurredAt:   time.Now().UTC(),
		}},
	})

	if len(response.RecentEvents) != 1 {
		t.Fatalf("events = %d", len(response.RecentEvents))
	}
	event := response.RecentEvents[0]
	if event.FromVersion == nil || *event.FromVersion != fromVersion ||
		event.ToVersion != 3 {
		t.Fatalf("event versions = %v -> %d", event.FromVersion, event.ToVersion)
	}
}
