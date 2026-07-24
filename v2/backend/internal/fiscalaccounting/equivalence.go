package fiscalaccounting

import (
	"strings"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

type equivalentLineKey struct {
	accountID          string
	partyID            string
	currency           string
	exchangeRate       string
	exchangeRateDate   string
	exchangeRateSource string
}

type equivalentAmounts struct {
	debit             accounting.Decimal
	credit            accounting.Decimal
	transactionDebit  accounting.Decimal
	transactionCredit accounting.Decimal
}

func entriesEquivalent(
	existing accounting.JournalEntry,
	expected accounting.JournalEntry,
) bool {
	if existing.Date.Format("2006-01-02") != expected.Date.Format("2006-01-02") ||
		existing.Kind != expected.Kind ||
		existing.PostingKind != expected.PostingKind ||
		existing.FunctionalCurrency.Code() != expected.FunctionalCurrency.Code() ||
		existing.Source.Type != expected.Source.Type ||
		existing.Source.ID != expected.Source.ID {
		return false
	}
	existingLines := aggregateEquivalentLines(existing.Lines)
	expectedLines := aggregateEquivalentLines(expected.Lines)
	if len(existingLines) != len(expectedLines) {
		return false
	}
	for key, existingAmount := range existingLines {
		expectedAmount, ok := expectedLines[key]
		if !ok ||
			!existingAmount.debit.Equal(expectedAmount.debit) ||
			!existingAmount.credit.Equal(expectedAmount.credit) ||
			!existingAmount.transactionDebit.Equal(expectedAmount.transactionDebit) ||
			!existingAmount.transactionCredit.Equal(expectedAmount.transactionCredit) {
			return false
		}
	}
	return true
}

func aggregateEquivalentLines(
	lines []accounting.JournalLine,
) map[equivalentLineKey]equivalentAmounts {
	result := make(map[equivalentLineKey]equivalentAmounts, len(lines))
	for _, line := range lines {
		partyID := ""
		if line.PartyID != nil {
			partyID = line.PartyID.String()
		}
		rateDate := ""
		if !line.ExchangeRateDate.IsZero() {
			rateDate = line.ExchangeRateDate.Format("2006-01-02")
		}
		key := equivalentLineKey{
			accountID:          line.AccountID.String(),
			partyID:            partyID,
			currency:           line.Currency.Code(),
			exchangeRate:       line.ExchangeRate.String(),
			exchangeRateDate:   rateDate,
			exchangeRateSource: strings.TrimSpace(line.ExchangeRateSource),
		}
		amount := result[key]
		amount.debit = amount.debit.Add(line.Debit)
		amount.credit = amount.credit.Add(line.Credit)
		amount.transactionDebit = amount.transactionDebit.Add(line.TransactionDebit)
		amount.transactionCredit = amount.transactionCredit.Add(line.TransactionCredit)
		result[key] = amount
	}
	return result
}
