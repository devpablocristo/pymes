package accounting

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AnnualClosingLine struct {
	AccountID    uuid.UUID    `json:"account_id"`
	AccountCode  string       `json:"account_code"`
	AccountName  string       `json:"account_name"`
	AccountClass AccountClass `json:"account_class"`
	Debit        Decimal      `json:"debit"`
	Credit       Decimal      `json:"credit"`
}

type AnnualClosingWorkpaper struct {
	ID                 uuid.UUID           `json:"id"`
	From               time.Time           `json:"from"`
	To                 time.Time           `json:"to"`
	FunctionalCurrency Currency            `json:"functional_currency"`
	NetIncome          Decimal             `json:"net_income"`
	Lines              []AnnualClosingLine `json:"lines"`
	Draft              Draft               `json:"draft"`
	CreatedBy          string              `json:"created_by"`
	CreatedAt          time.Time           `json:"created_at"`
}

type AnnualClosingCommand struct {
	From               time.Time `json:"from"`
	To                 time.Time `json:"to"`
	FunctionalCurrency Currency  `json:"functional_currency"`
	IdempotencyKey     string    `json:"-"`
}

func BuildAnnualClosingWorkpaper(
	trial TrialBalance,
	functionalCurrency Currency,
	resultAccountID uuid.UUID,
	actor string,
	idempotencyKey string,
	now time.Time,
) (AnnualClosingWorkpaper, error) {
	return buildAnnualClosingWorkpaper(
		trial,
		functionalCurrency,
		resultAccountID,
		actor,
		idempotencyKey,
		now,
		UUIDGenerator{},
	)
}

func BuildAnnualClosingDraft(
	trial TrialBalance,
	functionalCurrency Currency,
	resultAccountID uuid.UUID,
	actor string,
	idempotencyKey string,
	now time.Time,
) (Draft, error) {
	workpaper, err := BuildAnnualClosingWorkpaper(
		trial,
		functionalCurrency,
		resultAccountID,
		actor,
		idempotencyKey,
		now,
	)
	if err != nil {
		return Draft{}, err
	}
	return workpaper.Draft, nil
}

func buildAnnualClosingWorkpaper(
	trial TrialBalance,
	functionalCurrency Currency,
	resultAccountID uuid.UUID,
	actor string,
	idempotencyKey string,
	now time.Time,
	ids IDGenerator,
) (AnnualClosingWorkpaper, error) {
	if trial.From.IsZero() || trial.AsOf.IsZero() || trial.AsOf.Before(trial.From) ||
		resultAccountID == uuid.Nil || strings.TrimSpace(actor) == "" ||
		strings.TrimSpace(idempotencyKey) == "" || ids == nil {
		return AnnualClosingWorkpaper{}, fmt.Errorf("%w: incomplete annual closing", ErrInvalidArgument)
	}
	var computedDebit, computedCredit Decimal
	for _, row := range trial.Rows {
		if row.AccountID == uuid.Nil || strings.TrimSpace(row.Code) == "" ||
			!row.Class.Valid() || row.Debit.Sign() < 0 || row.Credit.Sign() < 0 {
			return AnnualClosingWorkpaper{}, fmt.Errorf("%w: invalid trial-balance row", ErrInvalidArgument)
		}
		if err := validateAmountBounds(row.Debit); err != nil {
			return AnnualClosingWorkpaper{}, err
		}
		if err := validateAmountBounds(row.Credit); err != nil {
			return AnnualClosingWorkpaper{}, err
		}
		computedDebit = computedDebit.Add(row.Debit)
		computedCredit = computedCredit.Add(row.Credit)
	}
	if !computedDebit.Equal(trial.TotalDebit) ||
		!computedCredit.Equal(trial.TotalCredit) ||
		!computedDebit.Equal(computedCredit) {
		return AnnualClosingWorkpaper{}, fmt.Errorf("%w: trial balance is not balanced", ErrUnbalancedEntry)
	}
	ordered := append([]TrialBalanceRow(nil), trial.Rows...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Code == ordered[j].Code {
			return ordered[i].AccountID.String() < ordered[j].AccountID.String()
		}
		return ordered[i].Code < ordered[j].Code
	})
	workpaper := AnnualClosingWorkpaper{
		ID:                 ids.NewID(),
		From:               trial.From,
		To:                 trial.AsOf,
		FunctionalCurrency: functionalCurrency,
		CreatedBy:          actor,
		CreatedAt:          now,
	}
	draft := Draft{
		ID:                 workpaper.ID,
		Version:            1,
		IdempotencyKey:     strings.TrimSpace(idempotencyKey),
		Date:               trial.AsOf,
		Kind:               EntryClosing,
		FunctionalCurrency: functionalCurrency,
		Currency:           functionalCurrency,
		ExchangeRate:       One,
		Description:        fmt.Sprintf("Cierre anual %d", trial.AsOf.Year()),
		IsAdjustment:       true,
		SourceType:         "annual_closing",
		SourceID:           annualClosingSourceID(trial.From, trial.AsOf),
		CreatedBy:          actor,
		UpdatedBy:          actor,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	var closingDebit, closingCredit Decimal
	for _, row := range ordered {
		switch row.Class {
		case AccountRevenue, AccountCost, AccountExpense:
		default:
			continue
		}
		if row.AccountID == resultAccountID {
			return AnnualClosingWorkpaper{}, fmt.Errorf("%w: result account cannot be a temporary account", ErrConflict)
		}
		netDebit := row.Debit.Sub(row.Credit)
		if netDebit.IsZero() {
			continue
		}
		line := AnnualClosingLine{
			AccountID:    row.AccountID,
			AccountCode:  row.Code,
			AccountName:  row.Name,
			AccountClass: row.Class,
		}
		side := creditSide
		if netDebit.Sign() < 0 {
			side = debitSide
			line.Debit = netDebit.Abs()
			closingDebit = closingDebit.Add(line.Debit)
		} else {
			line.Credit = netDebit
			closingCredit = closingCredit.Add(line.Credit)
		}
		workpaper.Lines = append(workpaper.Lines, line)
		draft.Lines = append(draft.Lines, functionalLine(
			row.AccountID,
			side,
			netDebit.Abs(),
			functionalCurrency,
			nil,
			"Cierre "+row.Code+" "+row.Name,
		))
	}
	if len(workpaper.Lines) == 0 {
		return AnnualClosingWorkpaper{}, fmt.Errorf("%w: no temporary balances to close", ErrConflict)
	}
	workpaper.NetIncome = closingDebit.Sub(closingCredit)
	expectedNetIncome := BuildIncomeStatement(trial).NetIncome
	if !workpaper.NetIncome.Equal(expectedNetIncome) {
		return AnnualClosingWorkpaper{}, fmt.Errorf(
			"%w: closing result %s differs from income statement %s",
			ErrConflict,
			workpaper.NetIncome,
			expectedNetIncome,
		)
	}
	if !workpaper.NetIncome.IsZero() {
		resultSide := creditSide
		resultLine := AnnualClosingLine{
			AccountID:    resultAccountID,
			AccountCode:  "RESULT",
			AccountName:  "Resultado del ejercicio",
			AccountClass: AccountEquity,
		}
		if workpaper.NetIncome.Sign() < 0 {
			resultSide = debitSide
			resultLine.Debit = workpaper.NetIncome.Abs()
		} else {
			resultLine.Credit = workpaper.NetIncome
		}
		workpaper.Lines = append(workpaper.Lines, resultLine)
		draft.Lines = append(draft.Lines, functionalLine(
			resultAccountID,
			resultSide,
			workpaper.NetIncome.Abs(),
			functionalCurrency,
			nil,
			"Resultado del ejercicio",
		))
	}
	for index := range draft.Lines {
		draft.Lines[index].LineNo = index + 1
	}
	workpaper.Draft = draft
	if err := draft.ToEntry(EntrySource{
		Type:           "annual_closing",
		ID:             draft.ID,
		Event:          "primary",
		IdempotencyKey: draft.IdempotencyKey,
	}, actor).ValidateForPosting(); err != nil {
		return AnnualClosingWorkpaper{}, fmt.Errorf("accounting: invalid annual closing draft: %w", err)
	}
	return workpaper, nil
}
