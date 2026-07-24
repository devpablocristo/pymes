package accounting

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDraftSaveAllowsIncompleteAndUnbalancedWork(t *testing.T) {
	t.Parallel()

	base := Draft{
		ID:                 uuid.New(),
		Version:            1,
		IdempotencyKey:     "draft-workflow",
		Date:               dateFixture(),
		Kind:               EntryManual,
		FunctionalCurrency: MustCurrency("ARS"),
		Currency:           MustCurrency("ARS"),
		ExchangeRate:       One,
		Description:        "",
	}
	if err := base.ValidateForSave(); err != nil {
		t.Fatalf("save empty draft: %v", err)
	}

	unbalanced := base
	unbalanced.Lines = []JournalLine{
		functionalLine(
			uuid.New(),
			debitSide,
			MustDecimal("125.50"),
			MustCurrency("ARS"),
			nil,
			"",
		),
		functionalLine(
			uuid.New(),
			creditSide,
			MustDecimal("100"),
			MustCurrency("ARS"),
			nil,
			"",
		),
	}
	if err := unbalanced.ValidateForSave(); err != nil {
		t.Fatalf("save unbalanced draft: %v", err)
	}
}

func TestDraftSaveRejectsInvalidPersistedLineAndMixedCurrencyMetadata(t *testing.T) {
	t.Parallel()

	draft := Draft{
		ID:                 uuid.New(),
		Version:            1,
		IdempotencyKey:     "invalid-line",
		Date:               dateFixture(),
		Kind:               EntryManual,
		FunctionalCurrency: MustCurrency("ARS"),
		Currency:           MustCurrency("ARS"),
		ExchangeRate:       One,
		Lines: []JournalLine{
			functionalLine(
				uuid.New(),
				debitSide,
				MustDecimal("100"),
				MustCurrency("ARS"),
				nil,
				"",
			),
		},
	}
	draft.Lines[0].TransactionDebit = MustDecimal("60")
	draft.Lines[0].TransactionCredit = MustDecimal("40")
	if err := draft.ValidateForSave(); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("two-sided transaction amount error = %v", err)
	}

	foreign := draft
	foreign.IdempotencyKey = "mixed-currency"
	foreign.Currency = MustCurrency("USD")
	foreign.ExchangeRate = MustDecimal("1250")
	foreign.ExchangeRateDate = foreign.Date
	foreign.ExchangeRateSource = "BNA"
	foreign.Lines = []JournalLine{
		functionalLine(
			uuid.New(),
			debitSide,
			MustDecimal("100"),
			MustCurrency("ARS"),
			nil,
			"",
		),
	}
	if err := foreign.ValidateForSave(); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("mixed header/line currency error = %v", err)
	}
}

func TestFunctionalCurrencyHeaderHasNoExchangeRateNoise(t *testing.T) {
	t.Parallel()

	draft := Draft{
		ID:                 uuid.New(),
		Version:            1,
		IdempotencyKey:     "functional-rate",
		Date:               dateFixture(),
		Kind:               EntryManual,
		FunctionalCurrency: MustCurrency("ARS"),
		Currency:           MustCurrency("ARS"),
		ExchangeRate:       MustDecimal("1.01"),
		ExchangeRateDate:   time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		ExchangeRateSource: "BNA",
	}
	if err := draft.ValidateForSave(); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("functional currency metadata error = %v", err)
	}
}

func TestPostedEntryRejectsFutureLineExchangeRateDate(t *testing.T) {
	t.Parallel()

	entryDate := dateFixture()
	entry := JournalEntry{
		ID:                 uuid.New(),
		Date:               entryDate,
		Kind:               EntryManual,
		PostingKind:        "primary",
		FunctionalCurrency: MustCurrency("ARS"),
		Currency:           MustCurrency("USD"),
		ExchangeRate:       MustDecimal("100"),
		ExchangeRateDate:   entryDate,
		ExchangeRateSource: "BNA",
		Source: EntrySource{
			Type:           "manual",
			ID:             uuid.New(),
			Event:          "primary",
			IdempotencyKey: "future-line-rate",
		},
		Description: "Asiento en moneda extranjera",
		CreatedBy:   "actor",
		Lines: []JournalLine{
			lineWithAmounts(
				uuid.New(),
				debitSide,
				MustDecimal("100"),
				MustDecimal("1"),
				MustCurrency("USD"),
				MustDecimal("100"),
				entryDate.AddDate(0, 0, 1),
				"BNA",
				nil,
				"",
			),
			functionalLine(
				uuid.New(),
				creditSide,
				MustDecimal("100"),
				MustCurrency("ARS"),
				nil,
				"",
			),
		},
	}
	if err := entry.ValidateForPosting(); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("future line exchange-rate date error = %v", err)
	}
}
