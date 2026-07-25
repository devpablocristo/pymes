package accounting

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateFiscalYearBuildsTwelveContiguousMonths(t *testing.T) {
	t.Parallel()

	year, periods, err := GenerateFiscalYear(
		2023,
		time.July,
		uuid.New(),
		UUIDGenerator{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if year.Code != "2023/2024" ||
		year.StartDate.Format("2006-01-02") != "2023-07-01" ||
		year.EndDate.Format("2006-01-02") != "2024-06-30" {
		t.Fatalf("fiscal year = %+v", year)
	}
	if len(periods) != 12 {
		t.Fatalf("period count = %d", len(periods))
	}
	for index, period := range periods {
		if period.SequenceNo != index+1 ||
			period.FiscalYearID == nil ||
			*period.FiscalYearID != year.ID {
			t.Fatalf("period %d membership = %+v", index, period)
		}
		if index > 0 &&
			!period.StartDate.Equal(periods[index-1].EndDate.AddDate(0, 0, 1)) {
			t.Fatalf("period %d is not contiguous", index)
		}
	}
	if periods[7].EndDate.Format("2006-01-02") != "2024-02-29" {
		t.Fatalf("leap February end = %s", periods[7].EndDate)
	}
}

func TestServiceFiscalYearPeriodsCloseChronologically(t *testing.T) {
	t.Parallel()

	_, service, scope := serviceFixture(t)
	detail, err := service.CreateFiscalYear(
		context.Background(),
		scope,
		CreateFiscalYearCommand{
			StartYear:      2025,
			StartMonth:     time.January,
			IdempotencyKey: "fiscal-year-2025",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Periods) != 12 {
		t.Fatalf("periods = %d", len(detail.Periods))
	}
	if _, _, err := service.TransitionPeriod(
		context.Background(),
		scope,
		detail.Periods[1].ID,
		1,
		PeriodSoftClosed,
		"",
		"out-of-order-close",
	); !errors.Is(err, ErrPeriodSequence) {
		t.Fatalf("out-of-order close error = %v", err)
	}
	first, _, err := service.TransitionPeriod(
		context.Background(),
		scope,
		detail.Periods[0].ID,
		1,
		PeriodSoftClosed,
		"",
		"first-period-close",
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != PeriodSoftClosed {
		t.Fatalf("first period status = %s", first.Status)
	}
}

func TestAnnualCloseSequenceNeedsElevenLockedAndFinalSoftClosed(t *testing.T) {
	t.Parallel()

	yearID := uuid.New()
	_, periods, err := GenerateFiscalYear(
		2026,
		time.January,
		yearID,
		UUIDGenerator{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if annualCloseSequenceReady(periods) {
		t.Fatal("all-open year unexpectedly ready")
	}
	for index := 0; index < 11; index++ {
		periods[index].Status = PeriodLocked
	}
	periods[11].Status = PeriodSoftClosed
	if !annualCloseSequenceReady(periods) {
		t.Fatal("closing sequence unexpectedly rejected")
	}
}

func TestLegacyFiscalYearUsesItsExistingPeriodSet(t *testing.T) {
	t.Parallel()

	period := Period{
		ID:         uuid.New(),
		Name:       "2024",
		StartDate:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		Status:     PeriodSoftClosed,
		Version:    2,
		SequenceNo: 1,
		IsLegacy:   true,
	}
	if !annualCloseSequenceReady([]Period{period}) {
		t.Fatal("single preserved annual period should be ready to close")
	}
	year := FiscalYearSummary{
		ID:                uuid.New(),
		Code:              "2024",
		StartDate:         period.StartDate,
		EndDate:           period.EndDate,
		IsLegacy:          true,
		State:             FiscalYearClosing,
		Version:           1,
		PeriodCounts:      FiscalYearPeriodCounts{SoftClosed: 1},
		AnnualCloseStatus: AnnualCloseReady,
	}
	if err := validateStructuredPeriodTransition(
		period,
		PeriodLocked,
		[]Period{period},
		year,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	); !errors.Is(err, ErrAnnualClosePending) {
		t.Fatalf("legacy lock without annual close error = %v", err)
	}
	year.AnnualCloseStatus = AnnualCloseNotRequired
	if err := validateStructuredPeriodTransition(
		period,
		PeriodLocked,
		[]Period{period},
		year,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("legacy lock after annual close: %v", err)
	}
	if state := DeriveFiscalYearState(
		FiscalYearPeriodCounts{Locked: 1},
		AnnualCloseNotRequired,
	); state != FiscalYearClosed {
		t.Fatalf("legacy state = %s", state)
	}
}

func TestPostingAnnualClosingDraftLinksPostedEntryToFiscalYear(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	scope.CanPostAdjustments = true
	entry := manualEntryFixture(repository)
	draftID := uuid.New()
	yearID := uuid.New()
	period := repository.periods[repository.periodID]
	period.FiscalYearID = &yearID
	period.SequenceNo = 12
	period.Status = PeriodSoftClosed
	repository.periods[period.ID] = period
	draft := Draft{
		ID:                 draftID,
		Version:            1,
		IdempotencyKey:     "annual-draft-create",
		Date:               entry.Date,
		Kind:               EntryClosing,
		FunctionalCurrency: entry.FunctionalCurrency,
		Currency:           entry.Currency,
		ExchangeRate:       entry.ExchangeRate,
		Description:        "Cierre anual 2026",
		SourceType:         "annual_closing",
		SourceID:           yearID.String(),
		IsAdjustment:       true,
		Lines:              append([]JournalLine(nil), entry.Lines...),
	}
	repository.drafts[draft.ID] = draft
	repository.fiscalYears[yearID] = FiscalYearSummary{
		ID:                 yearID,
		Code:               "2025/2026",
		StartDate:          time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            period.EndDate,
		State:              FiscalYearClosing,
		Version:            1,
		PeriodCounts:       FiscalYearPeriodCounts{Locked: 11, SoftClosed: 1},
		AnnualCloseStatus:  AnnualCloseDraft,
		AnnualCloseDraftID: &draftID,
	}

	posted, err := service.PostDraft(
		context.Background(),
		scope,
		draft.ID,
		draft.Version,
		"annual-draft-post",
	)
	if err != nil {
		t.Fatal(err)
	}
	year := repository.fiscalYears[yearID]
	if year.AnnualCloseStatus != AnnualClosePosted ||
		year.AnnualCloseEntryID == nil ||
		*year.AnnualCloseEntryID != posted.ID {
		t.Fatalf("annual close link = %+v", year)
	}
}

func TestAnnualCloseFreezesFinalPeriodPostingAndDirectReopening(t *testing.T) {
	t.Parallel()

	for _, status := range []AnnualCloseStatus{
		AnnualCloseDraft,
		AnnualClosePosted,
		AnnualCloseNotRequired,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			repository, service, scope := serviceFixture(t)
			scope.CanPostAdjustments = true
			scope.CanReopenPeriods = true
			yearID := uuid.New()
			period := repository.periods[repository.periodID]
			period.FiscalYearID = &yearID
			period.SequenceNo = 1
			period.IsLegacy = true
			period.Status = PeriodSoftClosed
			period.Version = 2
			repository.periods[period.ID] = period
			year := FiscalYearSummary{
				ID:                yearID,
				Code:              "2025/2026",
				StartDate:         time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
				EndDate:           period.EndDate,
				IsLegacy:          true,
				State:             FiscalYearClosing,
				Version:           3,
				PeriodCounts:      FiscalYearPeriodCounts{SoftClosed: 1},
				AnnualCloseStatus: status,
			}
			if status == AnnualCloseDraft {
				draftID := uuid.New()
				year.AnnualCloseDraftID = &draftID
			}
			if status == AnnualClosePosted {
				entryID := uuid.New()
				year.AnnualCloseEntryID = &entryID
			}
			repository.fiscalYears[yearID] = year

			entry := manualEntryFixture(repository)
			entry.Kind = EntryAdjustment
			entry.IsAdjustment = true
			entry.Source.ID = uuid.New()
			entry.Source.Event = "adjustment"
			entry.Source.IdempotencyKey = "late-adjustment-" + string(status)
			if _, err := service.PostEntry(
				context.Background(),
				scope,
				entry,
			); !errors.Is(err, ErrAnnualClosePending) {
				t.Fatalf("post after annual close %s error = %v", status, err)
			}

			if _, _, err := service.TransitionPeriod(
				context.Background(),
				scope,
				period.ID,
				period.Version,
				PeriodOpen,
				"Reapertura directa",
				"direct-reopen-"+string(status),
			); !errors.Is(err, ErrAnnualClosePending) {
				t.Fatalf("direct reopen after annual close %s error = %v", status, err)
			}
		})
	}
}

func TestDiscardAnnualClosingDraftRestoresReadyFiscalYear(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	yearID := uuid.New()
	draftID := uuid.New()
	repository.drafts[draftID] = Draft{
		ID:             draftID,
		Version:        2,
		SourceType:     "annual_closing",
		SourceID:       yearID.String(),
		IdempotencyKey: "annual-close-draft",
	}
	repository.fiscalYears[yearID] = FiscalYearSummary{
		ID:                 yearID,
		Code:               "2026",
		StartDate:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		State:              FiscalYearClosing,
		Version:            4,
		PeriodCounts:       FiscalYearPeriodCounts{Locked: 11, SoftClosed: 1},
		AnnualCloseStatus:  AnnualCloseDraft,
		AnnualCloseDraftID: &draftID,
	}

	if err := service.DiscardDraft(
		context.Background(),
		scope,
		draftID,
		2,
		"Debe recalcularse",
		"discard-annual-close-draft",
	); err != nil {
		t.Fatalf("discard annual closing draft: %v", err)
	}
	if _, ok := repository.drafts[draftID]; ok {
		t.Fatal("annual closing draft was not discarded")
	}
	year := repository.fiscalYears[yearID]
	if year.AnnualCloseStatus != AnnualCloseReady ||
		year.AnnualCloseDraftID != nil ||
		year.Version != 5 {
		t.Fatalf("fiscal year after discard = %+v", year)
	}
}

func TestAnnualCloseNotRequiredReplayPrecedesVersionConflict(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	yearID := uuid.New()
	const idempotencyKey = "annual-close-not-required"
	repository.fiscalYears[yearID] = FiscalYearSummary{
		ID:                yearID,
		Code:              "2026",
		StartDate:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		State:             FiscalYearClosing,
		Version:           6,
		PeriodCounts:      FiscalYearPeriodCounts{Locked: 11, SoftClosed: 1},
		AnnualCloseStatus: AnnualCloseNotRequired,
	}
	repository.fiscalYearTransitions[yearID.String()+":"+string(AnnualCloseNotRequired)+":"+idempotencyKey] = true

	result, err := service.PrepareFiscalYearAnnualClose(
		context.Background(),
		scope,
		FiscalYearAnnualClosingCommand{
			FiscalYearID:       yearID,
			ExpectedVersion:    5,
			FunctionalCurrency: MustCurrency("ARS"),
			IdempotencyKey:     idempotencyKey,
		},
	)
	if err != nil {
		t.Fatalf("replay annual close not required: %v", err)
	}
	if result.FiscalYear.AnnualCloseStatus != AnnualCloseNotRequired ||
		result.Draft != nil {
		t.Fatalf("not-required replay = %+v", result)
	}
	if _, err := service.PrepareFiscalYearAnnualClose(
		context.Background(),
		scope,
		FiscalYearAnnualClosingCommand{
			FiscalYearID:       yearID,
			ExpectedVersion:    6,
			FunctionalCurrency: MustCurrency("ARS"),
			IdempotencyKey:     "different-annual-close-request",
		},
	); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("different not-required replay error = %v", err)
	}
}

func TestFiscalYearCapabilitiesUseOrganizationLocalDate(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	detail, err := service.CreateFiscalYear(
		context.Background(),
		scope,
		CreateFiscalYearCommand{
			StartYear:      2025,
			StartMonth:     time.January,
			IdempotencyKey: "local-date-fiscal-year",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	repository.accountingDate = time.Date(
		2025,
		time.January,
		15,
		0,
		0,
		0,
		0,
		time.UTC,
	)
	reloaded, err := service.GetFiscalYear(
		context.Background(),
		scope,
		detail.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Periods[0].Capabilities.CanSoftClose {
		t.Fatal("future local period unexpectedly can be closed")
	}
	if len(reloaded.Periods[0].Capabilities.Blockers) == 0 ||
		reloaded.Periods[0].Capabilities.Blockers[0] != "future_period" {
		t.Fatalf(
			"local-date blockers = %v",
			reloaded.Periods[0].Capabilities.Blockers,
		)
	}
}

func TestReopenFiscalYearReversesOnlyItsLinkedAnnualClosingEntry(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	scope.CanPostAdjustments = true
	scope.CanReopenPeriods = true
	yearID := uuid.New()
	draftID := uuid.New()
	period := repository.periods[repository.periodID]
	period.FiscalYearID = &yearID
	period.SequenceNo = 12
	period.Status = PeriodLocked
	period.Version = 3
	repository.periods[period.ID] = period

	original := manualEntryFixture(repository)
	original.ID = uuid.New()
	original.Number = 77
	original.Date = period.EndDate
	original.Kind = EntryClosing
	original.PostingKind = "primary"
	original.Source = EntrySource{
		Type:           "manual_draft",
		ID:             draftID,
		Event:          "primary",
		IdempotencyKey: "annual-close-post",
	}
	original.DraftID = &draftID
	for index := range original.Lines {
		original.Lines[index].ID = uuid.New()
		original.Lines[index].LineNo = index + 1
	}
	repository.entries[original.ID] = cloneEntry(original)
	before := cloneEntry(repository.entries[original.ID])
	repository.fiscalYears[yearID] = FiscalYearSummary{
		ID:                 yearID,
		Code:               "2025/2026",
		StartDate:          time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            period.EndDate,
		State:              FiscalYearClosed,
		Version:            8,
		PeriodCounts:       FiscalYearPeriodCounts{Locked: 12},
		AnnualCloseStatus:  AnnualClosePosted,
		AnnualCloseDraftID: &draftID,
		AnnualCloseEntryID: &original.ID,
	}

	updated, err := service.ReopenFiscalYear(
		context.Background(),
		scope,
		ReopenFiscalYearCommand{
			FiscalYearID:    yearID,
			ExpectedVersion: 8,
			Reason:          "Ajuste posterior autorizado",
			IdempotencyKey:  "reopen-2025-2026",
		},
	)
	if err != nil {
		t.Fatalf("reopen posted fiscal year: %v", err)
	}
	if updated.AnnualCloseStatus != AnnualCloseReversed ||
		updated.AnnualCloseReversalEntryID == nil {
		t.Fatalf("reopened fiscal year = %+v", updated)
	}
	reversal := repository.entries[*updated.AnnualCloseReversalEntryID]
	if reversal.ReversesEntryID == nil ||
		*reversal.ReversesEntryID != original.ID ||
		reversal.Kind != EntryReversal {
		t.Fatalf("annual closing reversal = %+v", reversal)
	}
	for index := range original.Lines {
		if !reversal.Lines[index].Debit.Equal(original.Lines[index].Credit) ||
			!reversal.Lines[index].Credit.Equal(original.Lines[index].Debit) {
			t.Fatalf("reversal line %d = %+v", index+1, reversal.Lines[index])
		}
	}
	if !reflect.DeepEqual(before, repository.entries[original.ID]) {
		t.Fatalf(
			"annual closing entry mutated\nbefore: %+v\nafter: %+v",
			before,
			repository.entries[original.ID],
		)
	}
	if repository.periods[period.ID].Status != PeriodSoftClosed {
		t.Fatalf("last period status = %s", repository.periods[period.ID].Status)
	}
}

func TestAnnualCloseInternalReversalRejectsUnlinkedClosingEntry(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	draftID := uuid.New()
	linkedDraftID := uuid.New()
	entry := manualEntryFixture(repository)
	entry.ID = uuid.New()
	entry.Kind = EntryClosing
	entry.Source = EntrySource{
		Type:           "manual_draft",
		ID:             draftID,
		Event:          "primary",
		IdempotencyKey: "closing-entry",
	}
	entry.DraftID = &draftID
	repository.entries[entry.ID] = cloneEntry(entry)
	year := FiscalYearSummary{
		ID:                 uuid.New(),
		AnnualCloseStatus:  AnnualClosePosted,
		AnnualCloseDraftID: &linkedDraftID,
		AnnualCloseEntryID: &entry.ID,
	}
	if _, err := service.reverseAnnualClosingEntryInTx(
		context.Background(),
		repository,
		scope,
		year,
		ReverseEntryCommand{
			EntryID:        entry.ID,
			Date:           entry.Date,
			Reason:         "No autorizado",
			IdempotencyKey: "unlinked-closing-reversal",
		},
	); !errors.Is(err, ErrReversalNotAllowed) {
		t.Fatalf("unlinked annual closing reversal error = %v", err)
	}
}
