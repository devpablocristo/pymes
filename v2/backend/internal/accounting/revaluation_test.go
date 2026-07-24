package accounting

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCurrencyRevaluationProducesSignedExactBalancedDraft(t *testing.T) {
	t.Parallel()

	closingDate := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	functional := MustCurrency("ARS")
	usdRate, err := NewClosingExchangeRate(
		closingDate,
		MustCurrency("USD"),
		functional,
		MustDecimal("1000"),
		"BNA",
		"https://example.test/usd",
		[]byte("usd-rate"),
	)
	if err != nil {
		t.Fatal(err)
	}
	eurRate, err := NewClosingExchangeRate(
		closingDate,
		MustCurrency("EUR"),
		functional,
		MustDecimal("1000"),
		"BNA",
		"https://example.test/eur",
		[]byte("eur-rate"),
	)
	if err != nil {
		t.Fatal(err)
	}
	assetID := uuid.New()
	liabilityID := uuid.New()
	gainID := uuid.New()
	lossID := uuid.New()
	positions := []CurrencyRevaluationPosition{
		{
			AccountID:      assetID,
			AccountCode:    "1.1.20",
			AccountName:    "Banco USD",
			NormalBalance:  NormalDebit,
			Currency:       MustCurrency("USD"),
			CurrencyAmount: MustDecimal("100"),
			CarryingAmount: MustDecimal("90000"),
		},
		{
			AccountID:      liabilityID,
			AccountCode:    "2.1.20",
			AccountName:    "Proveedor EUR",
			NormalBalance:  NormalCredit,
			Currency:       MustCurrency("EUR"),
			CurrencyAmount: MustDecimal("-50"),
			CarryingAmount: MustDecimal("-45000"),
		},
	}
	workpaper, err := BuildCurrencyRevaluationWorkpaper(
		closingDate,
		functional,
		positions,
		[]ClosingExchangeRate{usdRate, eurRate},
		gainID,
		lossID,
		"contador_1",
		closingDate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(workpaper.Lines) != 2 {
		t.Fatalf("line count = %d, want 2", len(workpaper.Lines))
	}
	if workpaper.Lines[0].AccountID != assetID ||
		workpaper.Lines[0].ExchangeDifference.String() != "10000" {
		t.Fatalf("asset calculation = %+v", workpaper.Lines[0])
	}
	if workpaper.Lines[1].AccountID != liabilityID ||
		workpaper.Lines[1].ExchangeDifference.String() != "-5000" {
		t.Fatalf("liability calculation = %+v", workpaper.Lines[1])
	}
	if workpaper.TotalGain.String() != "10000" ||
		workpaper.TotalLoss.String() != "5000" ||
		workpaper.NetResult.String() != "5000" {
		t.Fatalf(
			"revaluation totals = gain %s loss %s net %s",
			workpaper.TotalGain,
			workpaper.TotalLoss,
			workpaper.NetResult,
		)
	}
	entry := workpaper.Draft.ToEntry(EntrySource{
		Type:           "currency_revaluation",
		ID:             workpaper.ID,
		Event:          "primary",
		IdempotencyKey: workpaper.Draft.IdempotencyKey,
	}, "contador_1")
	if err := entry.ValidateForPosting(); err != nil {
		t.Fatalf("revaluation draft is not postable: %v", err)
	}
	assertLine(t, entry.Lines, assetID, "10000", "0")
	assertLine(t, entry.Lines, liabilityID, "0", "5000")
	assertLine(t, entry.Lines, gainID, "0", "10000")
	assertLine(t, entry.Lines, lossID, "5000", "0")
}

func TestCurrencyRevaluationChecksumAndLineOrderAreInputOrderIndependent(t *testing.T) {
	t.Parallel()

	date := dateFixture()
	functional := MustCurrency("ARS")
	usd, err := NewClosingExchangeRate(date, MustCurrency("USD"), functional, MustDecimal("2"), "source", "", []byte("usd"))
	if err != nil {
		t.Fatal(err)
	}
	eur, err := NewClosingExchangeRate(date, MustCurrency("EUR"), functional, MustDecimal("3"), "source", "", []byte("eur"))
	if err != nil {
		t.Fatal(err)
	}
	firstID := uuid.New()
	secondID := uuid.New()
	positions := []CurrencyRevaluationPosition{
		{AccountID: secondID, AccountCode: "2", AccountName: "Second", NormalBalance: NormalDebit, Currency: MustCurrency("USD"), CurrencyAmount: MustDecimal("10"), CarryingAmount: MustDecimal("10")},
		{AccountID: firstID, AccountCode: "1", AccountName: "First", NormalBalance: NormalDebit, Currency: MustCurrency("EUR"), CurrencyAmount: MustDecimal("10"), CarryingAmount: MustDecimal("10")},
	}
	gainID := uuid.New()
	lossID := uuid.New()
	first, err := BuildCurrencyRevaluationWorkpaper(
		date,
		functional,
		positions,
		[]ClosingExchangeRate{usd, eur},
		gainID,
		lossID,
		"actor",
		date,
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildCurrencyRevaluationWorkpaper(
		date,
		functional,
		[]CurrencyRevaluationPosition{positions[1], positions[0]},
		[]ClosingExchangeRate{eur, usd},
		gainID,
		lossID,
		"actor",
		date,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.SourceChecksum != second.SourceChecksum {
		t.Fatalf("checksums differ: %s != %s", first.SourceChecksum, second.SourceChecksum)
	}
	if first.Lines[0].AccountCode != "1" || second.Lines[0].AccountCode != "1" {
		t.Fatalf("calculation order = %+v / %+v", first.Lines, second.Lines)
	}
}

func TestCurrencyRevaluationRequiresEveryClosingRate(t *testing.T) {
	t.Parallel()

	_, err := BuildCurrencyRevaluationWorkpaper(
		dateFixture(),
		MustCurrency("ARS"),
		[]CurrencyRevaluationPosition{{
			AccountID:      uuid.New(),
			AccountCode:    "1",
			AccountName:    "Banco USD",
			NormalBalance:  NormalDebit,
			Currency:       MustCurrency("USD"),
			CurrencyAmount: MustDecimal("1"),
			CarryingAmount: MustDecimal("1"),
		}},
		nil,
		uuid.New(),
		uuid.New(),
		"actor",
		dateFixture(),
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing rate error = %v", err)
	}
}
