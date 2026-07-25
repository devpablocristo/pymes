package accounting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Service) ListFiscalYears(
	ctx context.Context,
	scope Scope,
	filter FiscalYearFilter,
) (FiscalYearPage, error) {
	if err := filter.Validate(); err != nil {
		return FiscalYearPage{}, err
	}
	filter = filter.normalized()
	var page FiscalYearPage
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		page, err = repos.ListFiscalYears(ctx, filter)
		return err
	})
	return page, err
}

func (s *Service) GetFiscalYear(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
) (FiscalYearDetail, error) {
	if id == uuid.Nil {
		return FiscalYearDetail{}, fmt.Errorf(
			"%w: fiscal year id is required",
			ErrInvalidArgument,
		)
	}
	var detail FiscalYearDetail
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		year, err := repos.GetFiscalYear(ctx, id, false)
		if err != nil {
			return err
		}
		periods, err := repos.ListFiscalYearPeriods(ctx, id, false)
		if err != nil {
			return err
		}
		events, err := repos.ListFiscalYearEvents(ctx, id, 50)
		if err != nil {
			return err
		}
		localDate, err := repos.AccountingLocalDate(ctx)
		if err != nil {
			return err
		}
		applyPeriodCapabilities(periods, year, localDate)
		year.Capabilities = DeriveFiscalYearCapabilities(
			year.State,
			year.PeriodCounts,
			year.AnnualCloseStatus,
		)
		detail = FiscalYearDetail{
			FiscalYearSummary: year,
			Periods:           periods,
			RecentEvents:      events,
		}
		return nil
	})
	return detail, err
}

func (s *Service) CreateFiscalYear(
	ctx context.Context,
	scope Scope,
	command CreateFiscalYearCommand,
) (FiscalYearDetail, error) {
	if command.StartYear < 1900 || command.StartYear > 9998 ||
		strings.TrimSpace(command.IdempotencyKey) == "" {
		return FiscalYearDetail{}, fmt.Errorf(
			"%w: fiscal year start is required",
			ErrInvalidArgument,
		)
	}
	var detail FiscalYearDetail
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		startMonth := command.StartMonth
		if startMonth == 0 {
			var err error
			startMonth, err = repos.FiscalYearStartMonth(ctx)
			if err != nil {
				return err
			}
		}
		year, periods, err := GenerateFiscalYear(
			command.StartYear,
			startMonth,
			s.ids.NewID(),
			s.ids,
		)
		if err != nil {
			return err
		}
		year.CreatedBy = scope.ActorID
		year.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
		year.CreatedAt = s.clock.Now()
		year.UpdatedAt = year.CreatedAt
		created, err := repos.CreateFiscalYear(ctx, year, periods)
		if err != nil {
			return err
		}
		periods, err = repos.ListFiscalYearPeriods(ctx, created.ID, false)
		if err != nil {
			return err
		}
		localDate, err := repos.AccountingLocalDate(ctx)
		if err != nil {
			return err
		}
		applyPeriodCapabilities(periods, created, localDate)
		created.Capabilities = DeriveFiscalYearCapabilities(
			created.State,
			created.PeriodCounts,
			created.AnnualCloseStatus,
		)
		detail = FiscalYearDetail{
			FiscalYearSummary: created,
			Periods:           periods,
		}
		return nil
	})
	return detail, err
}

func (s *Service) GetPeriodDetail(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
) (PeriodDetail, error) {
	if id == uuid.Nil {
		return PeriodDetail{}, fmt.Errorf(
			"%w: period id is required",
			ErrInvalidArgument,
		)
	}
	var detail PeriodDetail
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		period, err := repos.GetPeriod(ctx, id, false)
		if err != nil {
			return err
		}
		period.Checklist, err = repos.PreviewCloseChecklist(ctx, id)
		if err != nil {
			return err
		}
		events, err := repos.ListPeriodEvents(ctx, id, 50)
		if err != nil {
			return err
		}
		if period.FiscalYearID != nil {
			year, getErr := repos.GetFiscalYear(ctx, *period.FiscalYearID, false)
			if getErr != nil {
				return getErr
			}
			periods, listErr := repos.ListFiscalYearPeriods(
				ctx,
				*period.FiscalYearID,
				false,
			)
			if listErr != nil {
				return listErr
			}
			localDate, dateErr := repos.AccountingLocalDate(ctx)
			if dateErr != nil {
				return dateErr
			}
			applyPeriodCapabilities(periods, year, localDate)
			for _, candidate := range periods {
				if candidate.ID == period.ID {
					period.Capabilities = candidate.Capabilities
					break
				}
			}
		}
		detail = PeriodDetail{Period: period, RecentEvents: events}
		return nil
	})
	return detail, err
}

func (s *Service) ListPeriodEvents(
	ctx context.Context,
	scope Scope,
	periodID uuid.UUID,
	limit int,
) ([]PeriodAudit, error) {
	if periodID == uuid.Nil || limit < 0 || limit > 200 {
		return nil, fmt.Errorf(
			"%w: invalid period event query",
			ErrInvalidArgument,
		)
	}
	if limit == 0 {
		limit = 50
	}
	var events []PeriodAudit
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		events, err = repos.ListPeriodEvents(ctx, periodID, limit)
		return err
	})
	return events, err
}

func (s *Service) PrepareFiscalYearAnnualClose(
	ctx context.Context,
	scope Scope,
	command FiscalYearAnnualClosingCommand,
) (FiscalYearAnnualCloseResult, error) {
	if command.FiscalYearID == uuid.Nil || command.ExpectedVersion <= 0 ||
		strings.TrimSpace(command.IdempotencyKey) == "" {
		return FiscalYearAnnualCloseResult{}, fmt.Errorf(
			"%w: fiscal year, version and idempotency key are required",
			ErrInvalidArgument,
		)
	}
	var result FiscalYearAnnualCloseResult
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		if existing, findErr := repos.FindDraftByIdempotency(
			ctx,
			command.IdempotencyKey,
		); findErr == nil {
			if existing.SourceType != "annual_closing" ||
				existing.SourceID != command.FiscalYearID.String() {
				return ErrIdempotencyConflict
			}
			year, getErr := repos.GetFiscalYear(ctx, command.FiscalYearID, false)
			if getErr != nil {
				return getErr
			}
			result = FiscalYearAnnualCloseResult{
				FiscalYear: year,
				Draft:      &existing,
			}
			return nil
		} else if !errors.Is(findErr, ErrNotFound) {
			return findErr
		}

		year, err := repos.GetFiscalYear(ctx, command.FiscalYearID, true)
		if err != nil {
			return err
		}
		if year.AnnualCloseStatus == AnnualCloseNotRequired {
			applied, replayErr := repos.FiscalYearTransitionWasApplied(
				ctx,
				year.ID,
				AnnualCloseNotRequired,
				command.IdempotencyKey,
			)
			if replayErr != nil {
				return replayErr
			}
			if !applied {
				return ErrIdempotencyConflict
			}
			result.FiscalYear = year
			return nil
		}
		if year.Version != command.ExpectedVersion {
			return ErrVersionConflict
		}
		periods, err := repos.ListFiscalYearPeriods(ctx, year.ID, true)
		if err != nil {
			return err
		}
		if !annualCloseSequenceReady(periods) {
			return ErrFiscalYearNotReady
		}
		last := periods[len(periods)-1]
		checklist, err := repos.CloseChecklist(ctx, last.ID)
		if err != nil {
			return err
		}
		if checklist.BlockingCount() != 0 {
			return fmt.Errorf(
				"%w: close checklist has %d blocking items",
				ErrFiscalYearNotReady,
				checklist.BlockingCount(),
			)
		}
		if year.AnnualCloseStatus == AnnualCloseNotReady ||
			year.AnnualCloseStatus == AnnualCloseReversed {
			year, err = repos.UpdateFiscalYearAnnualClose(
				ctx,
				FiscalYearAnnualCloseUpdate{
					FiscalYearID:    year.ID,
					ExpectedVersion: year.Version,
					Status:          AnnualCloseReady,
					Reason:          "annual closing prerequisites satisfied",
					IdempotencyKey:  command.IdempotencyKey,
				},
			)
			if err != nil {
				return err
			}
		}
		if year.AnnualCloseStatus != AnnualCloseReady {
			return ErrAnnualClosePending
		}
		workpaper, err := s.buildAnnualClosingInTx(
			ctx,
			repos,
			scope,
			AnnualClosingCommand{
				From:               year.StartDate,
				To:                 year.EndDate,
				FunctionalCurrency: command.FunctionalCurrency,
				IdempotencyKey:     command.IdempotencyKey,
			},
		)
		if errors.Is(err, ErrAnnualCloseNotRequired) {
			updated, updateErr := repos.UpdateFiscalYearAnnualClose(
				ctx,
				FiscalYearAnnualCloseUpdate{
					FiscalYearID:    year.ID,
					ExpectedVersion: year.Version,
					Status:          AnnualCloseNotRequired,
					Reason:          "no temporary balances",
					IdempotencyKey:  command.IdempotencyKey,
				},
			)
			if updateErr != nil {
				return updateErr
			}
			result.FiscalYear = updated
			return nil
		}
		if err != nil {
			return err
		}
		workpaper.Draft.SourceType = "annual_closing"
		workpaper.Draft.SourceID = year.ID.String()
		workpaper.Draft.Description = "Cierre anual " + year.Code
		created, err := repos.CreateDraft(ctx, workpaper.Draft)
		if err != nil {
			return err
		}
		updated, err := repos.UpdateFiscalYearAnnualClose(
			ctx,
			FiscalYearAnnualCloseUpdate{
				FiscalYearID:    year.ID,
				ExpectedVersion: year.Version,
				Status:          AnnualCloseDraft,
				DraftID:         &created.ID,
				IdempotencyKey:  command.IdempotencyKey,
			},
		)
		if err != nil {
			return err
		}
		result = FiscalYearAnnualCloseResult{
			FiscalYear: updated,
			Draft:      &created,
		}
		return nil
	})
	return result, err
}

func (s *Service) ReopenFiscalYear(
	ctx context.Context,
	scope Scope,
	command ReopenFiscalYearCommand,
) (FiscalYearSummary, error) {
	if command.FiscalYearID == uuid.Nil || command.ExpectedVersion <= 0 ||
		strings.TrimSpace(command.Reason) == "" ||
		strings.TrimSpace(command.IdempotencyKey) == "" {
		return FiscalYearSummary{}, fmt.Errorf(
			"%w: fiscal year, version, reason and idempotency key are required",
			ErrInvalidArgument,
		)
	}
	if !scope.CanReopenPeriods {
		return FiscalYearSummary{}, fmt.Errorf(
			"%w: fiscal year reopen permission is required",
			ErrConflict,
		)
	}
	var updated FiscalYearSummary
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		year, err := repos.GetFiscalYear(ctx, command.FiscalYearID, true)
		if err != nil {
			return err
		}
		if year.AnnualCloseStatus == AnnualCloseReversed &&
			year.AnnualCloseReversalEntryID != nil {
			applied, replayErr := repos.FiscalYearTransitionWasApplied(
				ctx,
				year.ID,
				AnnualCloseReversed,
				command.IdempotencyKey,
			)
			if replayErr != nil {
				return replayErr
			}
			if !applied {
				return ErrIdempotencyConflict
			}
			updated = year
			return nil
		}
		if year.AnnualCloseStatus == AnnualCloseNotReady &&
			year.PeriodCounts.Total() > 0 &&
			year.PeriodCounts.Locked == year.PeriodCounts.Total()-1 &&
			year.PeriodCounts.SoftClosed == 1 {
			applied, replayErr := repos.FiscalYearTransitionWasApplied(
				ctx,
				year.ID,
				AnnualCloseNotReady,
				command.IdempotencyKey,
			)
			if replayErr != nil {
				return replayErr
			}
			if !applied {
				return ErrIdempotencyConflict
			}
			updated = year
			return nil
		}
		if year.Version != command.ExpectedVersion {
			return ErrVersionConflict
		}
		if year.State != FiscalYearClosed {
			return ErrFiscalYearReopenOrder
		}
		latest, err := repos.LatestClosedFiscalYear(ctx, true)
		if err != nil {
			return err
		}
		if latest.ID != year.ID {
			return ErrFiscalYearReopenOrder
		}
		periods, err := repos.ListFiscalYearPeriods(ctx, year.ID, true)
		if err != nil {
			return err
		}
		if len(periods) == 0 ||
			periods[len(periods)-1].Status != PeriodLocked {
			return ErrFiscalYearReopenOrder
		}
		last := periods[len(periods)-1]
		last.Status = PeriodSoftClosed
		last.StatusChangedBy = scope.ActorID
		last.TransitionReason = strings.TrimSpace(command.Reason)
		last.Version++
		if _, err := repos.UpdatePeriod(
			ctx,
			last,
			last.Version-1,
			command.IdempotencyKey+":period-reopen",
		); err != nil {
			return err
		}

		status := AnnualCloseNotReady
		var reversalID *uuid.UUID
		if year.AnnualCloseStatus == AnnualClosePosted {
			if year.AnnualCloseEntryID == nil {
				return ErrAnnualClosePending
			}
			reversal, reverseErr := s.reverseAnnualClosingEntryInTx(
				ctx,
				repos,
				scope,
				year,
				ReverseEntryCommand{
					EntryID:        *year.AnnualCloseEntryID,
					Date:           year.EndDate,
					Reason:         strings.TrimSpace(command.Reason),
					IdempotencyKey: command.IdempotencyKey,
				},
			)
			if reverseErr != nil {
				return reverseErr
			}
			status = AnnualCloseReversed
			reversalID = &reversal.ID
		} else if year.AnnualCloseStatus != AnnualCloseNotRequired {
			return ErrAnnualClosePending
		}
		updated, err = repos.UpdateFiscalYearAnnualClose(
			ctx,
			FiscalYearAnnualCloseUpdate{
				FiscalYearID:    year.ID,
				ExpectedVersion: year.Version,
				Status:          status,
				ReversalEntryID: reversalID,
				Reason:          strings.TrimSpace(command.Reason),
				IdempotencyKey:  command.IdempotencyKey,
			},
		)
		return err
	})
	return updated, err
}

func (s *Service) syncFiscalYearAnnualClosePosted(
	ctx context.Context,
	repos Repositories,
	draft Draft,
	entry JournalEntry,
) error {
	if draft.SourceType != "annual_closing" {
		return nil
	}
	fiscalYearID, err := uuid.Parse(strings.TrimSpace(draft.SourceID))
	if err != nil {
		// Date-keyed annual closing drafts predate fiscal years. Keep them
		// postable during the compatibility window without inventing a link.
		return nil
	}
	year, err := repos.GetFiscalYear(ctx, fiscalYearID, true)
	if err != nil {
		return err
	}
	if year.AnnualCloseStatus == AnnualClosePosted &&
		year.AnnualCloseEntryID != nil &&
		*year.AnnualCloseEntryID == entry.ID {
		return nil
	}
	if year.AnnualCloseStatus != AnnualCloseDraft ||
		year.AnnualCloseDraftID == nil ||
		*year.AnnualCloseDraftID != draft.ID {
		return ErrAnnualClosePending
	}
	_, err = repos.UpdateFiscalYearAnnualClose(
		ctx,
		FiscalYearAnnualCloseUpdate{
			FiscalYearID:    year.ID,
			ExpectedVersion: year.Version,
			Status:          AnnualClosePosted,
			DraftID:         &draft.ID,
			EntryID:         &entry.ID,
			Reason:          "annual closing draft posted",
			IdempotencyKey:  entry.Source.IdempotencyKey,
		},
	)
	return err
}

func annualCloseSequenceReady(periods []Period) bool {
	if len(periods) == 0 {
		return false
	}
	legacy := false
	for _, period := range periods {
		legacy = legacy || period.IsLegacy
	}
	if !legacy && len(periods) != 12 {
		return false
	}
	for index, period := range periods {
		if !legacy && period.SequenceNo != index+1 {
			return false
		}
		if index < len(periods)-1 && period.Status != PeriodLocked {
			return false
		}
	}
	return periods[len(periods)-1].Status == PeriodSoftClosed
}

func validateStructuredPeriodTransition(
	period Period,
	target PeriodStatus,
	periods []Period,
	year FiscalYearSummary,
	now time.Time,
) error {
	index := periodPosition(periods, period.ID)
	if index < 0 {
		return ErrPeriodSequence
	}
	legacy := year.IsLegacy || period.IsLegacy
	if !legacy &&
		(len(periods) != 12 ||
			period.SequenceNo != index+1 ||
			period.SequenceNo < 1 ||
			period.SequenceNo > 12) {
		return ErrPeriodSequence
	}
	isClosing := (period.Status == PeriodOpen && target == PeriodSoftClosed) ||
		(period.Status == PeriodSoftClosed && target == PeriodLocked)
	if isClosing && period.EndDate.After(dateOnly(now)) {
		return ErrPeriodInFuture
	}
	switch {
	case period.Status == PeriodOpen && target == PeriodSoftClosed:
		if !noPriorOpenAt(periods, index) {
			return ErrPeriodSequence
		}
	case period.Status == PeriodSoftClosed && target == PeriodLocked:
		if !allPriorLockedAt(periods, index) {
			return ErrPeriodSequence
		}
		if index == len(periods)-1 &&
			year.AnnualCloseStatus != AnnualClosePosted &&
			year.AnnualCloseStatus != AnnualCloseNotRequired {
			return ErrAnnualClosePending
		}
	case period.Status == PeriodSoftClosed && target == PeriodOpen:
		if index == len(periods)-1 &&
			(year.AnnualCloseStatus == AnnualCloseDraft ||
				year.AnnualCloseStatus == AnnualClosePosted ||
				year.AnnualCloseStatus == AnnualCloseNotRequired) {
			return ErrAnnualClosePending
		}
		if latestNonOpenPosition(periods) != index {
			return ErrPeriodSequence
		}
	case period.Status == PeriodLocked && target == PeriodSoftClosed:
		if index == len(periods)-1 &&
			(year.AnnualCloseStatus == AnnualCloseDraft ||
				year.AnnualCloseStatus == AnnualClosePosted ||
				year.AnnualCloseStatus == AnnualCloseNotRequired) {
			return ErrFiscalYearReopenOrder
		}
		if year.State == FiscalYearClosed {
			return ErrFiscalYearReopenOrder
		}
		if latestNonOpenPosition(periods) != index {
			return ErrPeriodSequence
		}
	default:
		return ErrPeriodSequence
	}
	return nil
}

func applyPeriodCapabilities(
	periods []Period,
	year FiscalYearSummary,
	now time.Time,
) {
	latestNonOpen := latestNonOpenPosition(periods)
	for index := range periods {
		period := &periods[index]
		capabilities := PeriodCapabilities{}
		if period.EndDate.After(dateOnly(now)) {
			capabilities.Blockers = append(capabilities.Blockers, "future_period")
		}
		switch period.Status {
		case PeriodOpen:
			capabilities.CanSoftClose = !period.EndDate.After(dateOnly(now)) &&
				noPriorOpenAt(periods, index)
			if capabilities.CanSoftClose {
				capabilities.Targets = append(capabilities.Targets, string(PeriodSoftClosed))
			}
		case PeriodSoftClosed:
			capabilities.CanLock = allPriorLockedAt(periods, index)
			if index == len(periods)-1 &&
				year.AnnualCloseStatus != AnnualClosePosted &&
				year.AnnualCloseStatus != AnnualCloseNotRequired {
				capabilities.CanLock = false
				capabilities.Blockers = append(
					capabilities.Blockers,
					"annual_close_pending",
				)
			}
			capabilities.CanReopen = latestNonOpen == index
			if index == len(periods)-1 &&
				(year.AnnualCloseStatus == AnnualCloseDraft ||
					year.AnnualCloseStatus == AnnualClosePosted ||
					year.AnnualCloseStatus == AnnualCloseNotRequired) {
				capabilities.CanReopen = false
				capabilities.Blockers = append(
					capabilities.Blockers,
					"annual_close_frozen",
				)
			}
			if capabilities.CanLock {
				capabilities.Targets = append(capabilities.Targets, string(PeriodLocked))
			}
			if capabilities.CanReopen {
				capabilities.Targets = append(capabilities.Targets, string(PeriodOpen))
			}
		case PeriodLocked:
			capabilities.CanReopen = latestNonOpen == index &&
				year.State != FiscalYearClosed
			if capabilities.CanReopen {
				capabilities.Targets = append(capabilities.Targets, string(PeriodSoftClosed))
			}
		}
		period.Capabilities = capabilities
	}
}

func noPriorOpenAt(periods []Period, position int) bool {
	for index, period := range periods {
		if index < position && period.Status == PeriodOpen {
			return false
		}
	}
	return true
}

func allPriorLockedAt(periods []Period, position int) bool {
	for index, period := range periods {
		if index < position && period.Status != PeriodLocked {
			return false
		}
	}
	return true
}

func latestNonOpenPosition(periods []Period) int {
	latest := -1
	for index, period := range periods {
		if period.Status != PeriodOpen {
			latest = index
		}
	}
	return latest
}

func periodPosition(periods []Period, id uuid.UUID) int {
	for index := range periods {
		if periods[index].ID == id {
			return index
		}
	}
	return -1
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}
