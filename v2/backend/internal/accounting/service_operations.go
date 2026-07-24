package accounting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Service) ImportStatement(
	ctx context.Context,
	scope Scope,
	financialAccountID uuid.UUID,
	fileName string,
	format StatementFormat,
	content []byte,
	currency Currency,
) (StatementImport, error) {
	statement, err := NewStatementImport(
		financialAccountID,
		fileName,
		format,
		content,
		currency,
		scope.ActorID,
		s.clock.Now(),
	)
	if err != nil {
		return StatementImport{}, err
	}
	var imported StatementImport
	err = s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		existing, findErr := repos.FindStatementImportByHash(ctx, financialAccountID, statement.SHA256)
		if findErr == nil {
			imported = existing
			return nil
		}
		if !errors.Is(findErr, ErrNotFound) {
			return findErr
		}
		var createErr error
		imported, createErr = repos.CreateStatementImport(ctx, statement)
		return createErr
	})
	return imported, err
}

type CreateReconciliationCommand struct {
	FinancialAccountID uuid.UUID
	PeriodStart        time.Time
	PeriodEnd          time.Time
	StatementOpening   Decimal
	StatementClosing   Decimal
	Matches            []ReconciliationMatch
}

func (s *Service) CreateReconciliation(
	ctx context.Context,
	scope Scope,
	command CreateReconciliationCommand,
) (Reconciliation, error) {
	if command.FinancialAccountID == uuid.Nil || command.PeriodStart.IsZero() ||
		command.PeriodEnd.Before(command.PeriodStart) {
		return Reconciliation{}, fmt.Errorf("%w: invalid reconciliation period", ErrInvalidArgument)
	}
	reconciliation := Reconciliation{
		ID:                 s.ids.NewID(),
		FinancialAccountID: command.FinancialAccountID,
		PeriodStart:        command.PeriodStart,
		PeriodEnd:          command.PeriodEnd,
		StatementOpening:   command.StatementOpening,
		StatementClosing:   command.StatementClosing,
		Status:             ReconciliationOpen,
		Version:            1,
		Matches:            append([]ReconciliationMatch(nil), command.Matches...),
	}
	normalizeReconciliationMatches(&reconciliation, scope.ActorID, s.clock.Now(), s.ids)
	var created Reconciliation
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		created, err = repos.CreateReconciliation(ctx, reconciliation)
		return err
	})
	return created, err
}

func (s *Service) GetReconciliation(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
) (Reconciliation, error) {
	var reconciliation Reconciliation
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		reconciliation, err = repos.GetReconciliation(ctx, id, false)
		return err
	})
	return reconciliation, err
}

func (s *Service) ListReconciliations(
	ctx context.Context,
	scope Scope,
	page PageRequest,
) (PageResult[Reconciliation], error) {
	var result PageResult[Reconciliation]
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		result, err = repos.ListReconciliations(ctx, page.normalized())
		return err
	})
	return result, err
}

func (s *Service) SaveReconciliation(
	ctx context.Context,
	scope Scope,
	reconciliation Reconciliation,
	expectedVersion int64,
) (Reconciliation, error) {
	if reconciliation.ID == uuid.Nil || expectedVersion <= 0 {
		return Reconciliation{}, fmt.Errorf("%w: reconciliation id and version are required", ErrInvalidArgument)
	}
	if reconciliation.Status != ReconciliationOpen {
		return Reconciliation{}, ErrReconciliationClosed
	}
	normalizeReconciliationMatches(&reconciliation, scope.ActorID, s.clock.Now(), s.ids)
	var saved Reconciliation
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		current, err := repos.GetReconciliation(ctx, reconciliation.ID, true)
		if err != nil {
			return err
		}
		if current.Status == ReconciliationClosed {
			return ErrReconciliationClosed
		}
		if current.Version != expectedVersion {
			return ErrVersionConflict
		}
		saved, err = repos.SaveReconciliation(ctx, reconciliation, expectedVersion)
		return err
	})
	return saved, err
}

func (s *Service) CloseReconciliation(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
	expectedVersion int64,
) (Reconciliation, error) {
	var closed Reconciliation
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		reconciliation, err := repos.GetReconciliation(ctx, id, true)
		if err != nil {
			return err
		}
		if reconciliation.Version != expectedVersion {
			return ErrVersionConflict
		}
		if reconciliation.Status == ReconciliationClosed {
			closed = reconciliation
			return nil
		}
		if !reconciliation.Difference().IsZero() {
			return fmt.Errorf("%w: reconciliation difference is %s", ErrConflict, reconciliation.Difference())
		}
		now := s.clock.Now()
		reconciliation.Status = ReconciliationClosed
		reconciliation.ClosedAt = &now
		reconciliation.ClosedBy = scope.ActorID
		closed, err = repos.SaveReconciliation(ctx, reconciliation, expectedVersion)
		return err
	})
	return closed, err
}

func (s *Service) ReopenReconciliation(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
	expectedVersion int64,
	reason string,
) (Reconciliation, error) {
	if !scope.CanManageAccounting || strings.TrimSpace(reason) == "" {
		return Reconciliation{}, fmt.Errorf("%w: permission and reason are required", ErrInvalidArgument)
	}
	var reopened Reconciliation
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		reconciliation, err := repos.GetReconciliation(ctx, id, true)
		if err != nil {
			return err
		}
		if reconciliation.Version != expectedVersion {
			return ErrVersionConflict
		}
		if reconciliation.Status != ReconciliationClosed {
			return ErrConflict
		}
		now := s.clock.Now()
		reconciliation.Status = ReconciliationOpen
		reconciliation.ReopenedAt = &now
		reconciliation.ReopenedBy = scope.ActorID
		reconciliation.ReopenedReason = strings.TrimSpace(reason)
		reopened, err = repos.SaveReconciliation(ctx, reconciliation, expectedVersion)
		return err
	})
	return reopened, err
}

func (s *Service) SuggestReconciliationMatches(
	ctx context.Context,
	scope Scope,
	importID uuid.UUID,
	financialAccountID uuid.UUID,
	from, to time.Time,
	maxDays int,
) ([]MatchSuggestion, error) {
	var suggestions []MatchSuggestion
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		movements, err := repos.ListStatementMovements(ctx, importID)
		if err != nil {
			return err
		}
		candidates, err := repos.ListReconciliationCandidates(ctx, financialAccountID, from, to)
		if err != nil {
			return err
		}
		suggestions = SuggestReconciliationMatches(movements, candidates, maxDays)
		return nil
	})
	return suggestions, err
}

func (s *Service) ImportInflationIndices(
	ctx context.Context,
	scope Scope,
	indices []InflationIndex,
) error {
	if len(indices) == 0 {
		return fmt.Errorf("%w: inflation indices are required", ErrInvalidArgument)
	}
	return s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		return repos.UpsertInflationIndices(ctx, indices)
	})
}

func (s *Service) PreviewInflationAdjustment(
	ctx context.Context,
	scope Scope,
	closingDate time.Time,
	functionalCurrency Currency,
) (InflationWorkpaper, error) {
	var workpaper InflationWorkpaper
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		workpaper, err = s.buildInflationWorkpaperInTx(
			ctx,
			repos,
			scope,
			closingDate,
			functionalCurrency,
		)
		return err
	})
	return workpaper, err
}

func (s *Service) CreateInflationAdjustment(
	ctx context.Context,
	scope Scope,
	closingDate time.Time,
	functionalCurrency Currency,
) (InflationWorkpaper, error) {
	var created InflationWorkpaper
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		workpaper, err := s.buildInflationWorkpaperInTx(
			ctx,
			repos,
			scope,
			closingDate,
			functionalCurrency,
		)
		if err != nil {
			return err
		}
		created, err = repos.CreateInflationWorkpaper(ctx, workpaper)
		return err
	})
	return created, err
}

func (s *Service) GetInflationWorkpaper(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
) (InflationWorkpaper, error) {
	var workpaper InflationWorkpaper
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		workpaper, err = repos.GetInflationWorkpaper(ctx, id)
		return err
	})
	return workpaper, err
}

func (s *Service) PreviewCurrencyRevaluation(
	ctx context.Context,
	scope Scope,
	closingDate time.Time,
	functionalCurrency Currency,
	rates []ClosingExchangeRate,
) (CurrencyRevaluationWorkpaper, error) {
	var workpaper CurrencyRevaluationWorkpaper
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		workpaper, err = s.buildCurrencyRevaluationInTx(
			ctx,
			repos,
			scope,
			closingDate,
			functionalCurrency,
			rates,
		)
		return err
	})
	return workpaper, err
}

func (s *Service) CreateCurrencyRevaluation(
	ctx context.Context,
	scope Scope,
	closingDate time.Time,
	functionalCurrency Currency,
	rates []ClosingExchangeRate,
) (CurrencyRevaluationWorkpaper, error) {
	var created CurrencyRevaluationWorkpaper
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		workpaper, err := s.buildCurrencyRevaluationInTx(
			ctx,
			repos,
			scope,
			closingDate,
			functionalCurrency,
			rates,
		)
		if err != nil {
			return err
		}
		created, err = repos.CreateCurrencyRevaluationWorkpaper(ctx, workpaper)
		return err
	})
	return created, err
}

func (s *Service) GetCurrencyRevaluationWorkpaper(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
) (CurrencyRevaluationWorkpaper, error) {
	if id == uuid.Nil {
		return CurrencyRevaluationWorkpaper{}, fmt.Errorf("%w: revaluation id is required", ErrInvalidArgument)
	}
	var workpaper CurrencyRevaluationWorkpaper
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		workpaper, err = repos.GetCurrencyRevaluationWorkpaper(ctx, id)
		return err
	})
	return workpaper, err
}

func (s *Service) buildInflationWorkpaperInTx(
	ctx context.Context,
	repos Repositories,
	scope Scope,
	closingDate time.Time,
	functionalCurrency Currency,
) (InflationWorkpaper, error) {
	positions, err := repos.ListInflationPositions(ctx, closingDate)
	if err != nil {
		return InflationWorkpaper{}, err
	}
	earliest := closingDate
	for _, position := range positions {
		if position.OriginDate.Before(earliest) {
			earliest = position.OriginDate
		}
	}
	indices, err := repos.ListInflationIndices(ctx, earliest, closingDate)
	if err != nil {
		return InflationWorkpaper{}, err
	}
	mappings, err := repos.GetMappings(ctx, []string{RoleRECPAM})
	if err != nil {
		return InflationWorkpaper{}, err
	}
	return BuildInflationWorkpaper(
		closingDate,
		functionalCurrency,
		indices,
		positions,
		mappings[RoleRECPAM].AccountID,
		scope.ActorID,
		s.clock.Now(),
	)
}

func (s *Service) buildCurrencyRevaluationInTx(
	ctx context.Context,
	repos Repositories,
	scope Scope,
	closingDate time.Time,
	functionalCurrency Currency,
	rates []ClosingExchangeRate,
) (CurrencyRevaluationWorkpaper, error) {
	if closingDate.IsZero() {
		return CurrencyRevaluationWorkpaper{}, fmt.Errorf("%w: revaluation closing date is required", ErrInvalidArgument)
	}
	positions, err := repos.ListCurrencyRevaluationPositions(ctx, closingDate, functionalCurrency)
	if err != nil {
		return CurrencyRevaluationWorkpaper{}, err
	}
	mappings, err := repos.GetMappings(ctx, []string{RoleFXGain, RoleFXLoss})
	if err != nil {
		return CurrencyRevaluationWorkpaper{}, err
	}
	return buildCurrencyRevaluationWorkpaper(
		closingDate,
		functionalCurrency,
		positions,
		rates,
		mappings[RoleFXGain].AccountID,
		mappings[RoleFXLoss].AccountID,
		scope.ActorID,
		s.clock.Now(),
		s.ids,
	)
}

func normalizeReconciliationMatches(
	reconciliation *Reconciliation,
	actor string,
	now time.Time,
	ids IDGenerator,
) {
	for index := range reconciliation.Matches {
		match := &reconciliation.Matches[index]
		if match.ID == uuid.Nil {
			match.ID = ids.NewID()
		}
		if match.CreatedAt.IsZero() {
			match.CreatedAt = now
		}
		match.CreatedBy = actor
	}
}
