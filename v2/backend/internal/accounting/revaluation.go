package accounting

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ClosingExchangeRate struct {
	Date               time.Time `json:"date"`
	Currency           Currency  `json:"currency"`
	FunctionalCurrency Currency  `json:"functional_currency"`
	Rate               Decimal   `json:"rate"`
	Source             string    `json:"source"`
	SourceReference    string    `json:"source_reference,omitempty"`
	SourceChecksum     string    `json:"source_checksum"`
}

func NewClosingExchangeRate(
	date time.Time,
	currency Currency,
	functionalCurrency Currency,
	rate Decimal,
	source string,
	sourceReference string,
	sourceDocument []byte,
) (ClosingExchangeRate, error) {
	hash := sha256.Sum256(sourceDocument)
	result := ClosingExchangeRate{
		Date:               date,
		Currency:           currency,
		FunctionalCurrency: functionalCurrency,
		Rate:               rate,
		Source:             strings.TrimSpace(source),
		SourceReference:    strings.TrimSpace(sourceReference),
		SourceChecksum:     hex.EncodeToString(hash[:]),
	}
	if err := result.Validate(); err != nil {
		return ClosingExchangeRate{}, err
	}
	return result, nil
}

func (rate ClosingExchangeRate) Validate() error {
	if rate.Date.IsZero() || rate.Currency.Code() == rate.FunctionalCurrency.Code() ||
		rate.Rate.Sign() <= 0 || strings.TrimSpace(rate.Source) == "" {
		return fmt.Errorf("%w: invalid closing exchange rate", ErrInvalidArgument)
	}
	if err := validateExchangeRateBounds(rate.Rate); err != nil {
		return err
	}
	if !validSHA256(rate.SourceChecksum) {
		return fmt.Errorf("%w: closing exchange rate checksum must be lowercase SHA-256", ErrInvalidArgument)
	}
	return nil
}

// CurrencyRevaluationPosition uses debit-positive signed amounts. Assets
// normally carry positive amounts; liabilities normally carry negative ones.
// This representation also handles abnormal balances without special cases.
type CurrencyRevaluationPosition struct {
	AccountID      uuid.UUID     `json:"account_id"`
	AccountCode    string        `json:"account_code"`
	AccountName    string        `json:"account_name"`
	NormalBalance  NormalBalance `json:"normal_balance"`
	Currency       Currency      `json:"currency"`
	CurrencyAmount Decimal       `json:"currency_amount"`
	CarryingAmount Decimal       `json:"carrying_amount"`
}

type CurrencyRevaluationLine struct {
	AccountID          uuid.UUID     `json:"account_id"`
	AccountCode        string        `json:"account_code"`
	AccountName        string        `json:"account_name"`
	NormalBalance      NormalBalance `json:"normal_balance"`
	Currency           Currency      `json:"currency"`
	CurrencyAmount     Decimal       `json:"currency_amount"`
	CarryingAmount     Decimal       `json:"carrying_amount"`
	ClosingRate        Decimal       `json:"closing_rate"`
	RevaluedAmount     Decimal       `json:"revalued_amount"`
	ExchangeDifference Decimal       `json:"exchange_difference"`
}

type CurrencyRevaluationWorkpaper struct {
	ID                 uuid.UUID                 `json:"id"`
	ClosingDate        time.Time                 `json:"closing_date"`
	FunctionalCurrency Currency                  `json:"functional_currency"`
	SourceChecksum     string                    `json:"source_checksum"`
	Rates              []ClosingExchangeRate     `json:"rates,omitempty"`
	Lines              []CurrencyRevaluationLine `json:"lines"`
	TotalGain          Decimal                   `json:"total_gain"`
	TotalLoss          Decimal                   `json:"total_loss"`
	NetResult          Decimal                   `json:"net_result"`
	Draft              Draft                     `json:"draft"`
	CreatedBy          string                    `json:"created_by"`
	CreatedAt          time.Time                 `json:"created_at"`
}

func BuildCurrencyRevaluationWorkpaper(
	closingDate time.Time,
	functionalCurrency Currency,
	positions []CurrencyRevaluationPosition,
	rates []ClosingExchangeRate,
	gainAccountID uuid.UUID,
	lossAccountID uuid.UUID,
	actor string,
	now time.Time,
) (CurrencyRevaluationWorkpaper, error) {
	return buildCurrencyRevaluationWorkpaper(
		closingDate,
		functionalCurrency,
		positions,
		rates,
		gainAccountID,
		lossAccountID,
		actor,
		now,
		UUIDGenerator{},
	)
}

func buildCurrencyRevaluationWorkpaper(
	closingDate time.Time,
	functionalCurrency Currency,
	positions []CurrencyRevaluationPosition,
	rates []ClosingExchangeRate,
	gainAccountID uuid.UUID,
	lossAccountID uuid.UUID,
	actor string,
	now time.Time,
	ids IDGenerator,
) (CurrencyRevaluationWorkpaper, error) {
	if closingDate.IsZero() || gainAccountID == uuid.Nil || lossAccountID == uuid.Nil ||
		gainAccountID == lossAccountID || strings.TrimSpace(actor) == "" || ids == nil {
		return CurrencyRevaluationWorkpaper{}, fmt.Errorf("%w: incomplete currency revaluation", ErrInvalidArgument)
	}
	ratesByCurrency := make(map[string]ClosingExchangeRate, len(rates))
	for _, rate := range rates {
		rate.Source = strings.TrimSpace(rate.Source)
		rate.SourceReference = strings.TrimSpace(rate.SourceReference)
		if err := rate.Validate(); err != nil {
			return CurrencyRevaluationWorkpaper{}, err
		}
		if rate.FunctionalCurrency.Code() != functionalCurrency.Code() {
			return CurrencyRevaluationWorkpaper{}, fmt.Errorf("%w: exchange rate functional currency", ErrInvalidArgument)
		}
		if rate.Date.After(closingDate) {
			return CurrencyRevaluationWorkpaper{}, fmt.Errorf("%w: future closing exchange rate", ErrInvalidArgument)
		}
		code := rate.Currency.Code()
		if _, duplicate := ratesByCurrency[code]; duplicate {
			return CurrencyRevaluationWorkpaper{}, fmt.Errorf("%w: duplicate closing rate for %s", ErrDuplicate, code)
		}
		ratesByCurrency[code] = rate
	}

	orderedPositions := append([]CurrencyRevaluationPosition(nil), positions...)
	sort.Slice(orderedPositions, func(i, j int) bool {
		if orderedPositions[i].AccountCode == orderedPositions[j].AccountCode {
			if orderedPositions[i].Currency.Code() == orderedPositions[j].Currency.Code() {
				return orderedPositions[i].AccountID.String() < orderedPositions[j].AccountID.String()
			}
			return orderedPositions[i].Currency.Code() < orderedPositions[j].Currency.Code()
		}
		return orderedPositions[i].AccountCode < orderedPositions[j].AccountCode
	})

	usedRates := make(map[string]ClosingExchangeRate)
	seenPositions := make(map[string]struct{}, len(orderedPositions))
	calculationLines := make([]CurrencyRevaluationLine, 0, len(orderedPositions))
	for _, position := range orderedPositions {
		if position.AccountID == uuid.Nil || strings.TrimSpace(position.AccountCode) == "" ||
			strings.TrimSpace(position.AccountName) == "" || !position.NormalBalance.Valid() {
			return CurrencyRevaluationWorkpaper{}, fmt.Errorf("%w: invalid revaluation position", ErrInvalidArgument)
		}
		positionKey := position.AccountID.String() + ":" + position.Currency.Code()
		if _, duplicate := seenPositions[positionKey]; duplicate {
			return CurrencyRevaluationWorkpaper{}, fmt.Errorf("%w: duplicate revaluation position", ErrDuplicate)
		}
		seenPositions[positionKey] = struct{}{}
		if position.Currency.Code() == functionalCurrency.Code() {
			return CurrencyRevaluationWorkpaper{}, fmt.Errorf("%w: revaluation position is already functional currency", ErrInvalidArgument)
		}
		if err := validateAmountBounds(position.CurrencyAmount); err != nil {
			return CurrencyRevaluationWorkpaper{}, err
		}
		if err := validateAmountBounds(position.CarryingAmount); err != nil {
			return CurrencyRevaluationWorkpaper{}, err
		}
		if position.CurrencyAmount.IsZero() && position.CarryingAmount.IsZero() {
			continue
		}
		rate, ok := ratesByCurrency[position.Currency.Code()]
		if !ok {
			return CurrencyRevaluationWorkpaper{}, fmt.Errorf(
				"%w: closing rate for %s",
				ErrNotFound,
				position.Currency.Code(),
			)
		}
		usedRates[position.Currency.Code()] = rate
		revalued := functionalCurrency.Round(position.CurrencyAmount.Mul(rate.Rate))
		difference := revalued.Sub(position.CarryingAmount)
		if err := validateAmountBounds(revalued); err != nil {
			return CurrencyRevaluationWorkpaper{}, err
		}
		if err := validateAmountBounds(difference); err != nil {
			return CurrencyRevaluationWorkpaper{}, err
		}
		if difference.IsZero() {
			continue
		}
		calculationLines = append(calculationLines, CurrencyRevaluationLine{
			AccountID:          position.AccountID,
			AccountCode:        position.AccountCode,
			AccountName:        position.AccountName,
			NormalBalance:      position.NormalBalance,
			Currency:           position.Currency,
			CurrencyAmount:     position.CurrencyAmount,
			CarryingAmount:     position.CarryingAmount,
			ClosingRate:        rate.Rate,
			RevaluedAmount:     revalued,
			ExchangeDifference: difference,
		})
	}
	if len(calculationLines) == 0 {
		return CurrencyRevaluationWorkpaper{}, fmt.Errorf("%w: revaluation produces no adjustment", ErrConflict)
	}

	orderedRates := make([]ClosingExchangeRate, 0, len(usedRates))
	for _, rate := range usedRates {
		orderedRates = append(orderedRates, rate)
	}
	sort.Slice(orderedRates, func(i, j int) bool {
		return orderedRates[i].Currency.Code() < orderedRates[j].Currency.Code()
	})
	sourceChecksum := revaluationChecksum(closingDate, orderedRates, orderedPositions)
	workpaper := CurrencyRevaluationWorkpaper{
		ID:                 ids.NewID(),
		ClosingDate:        closingDate,
		FunctionalCurrency: functionalCurrency,
		SourceChecksum:     sourceChecksum,
		Rates:              orderedRates,
		Lines:              calculationLines,
		CreatedBy:          actor,
		CreatedAt:          now,
	}
	draft := Draft{
		ID:                 ids.NewID(),
		Version:            1,
		IdempotencyKey:     "revaluation:" + sourceChecksum,
		Date:               closingDate,
		Kind:               EntryRevaluation,
		FunctionalCurrency: functionalCurrency,
		Currency:           functionalCurrency,
		ExchangeRate:       One,
		Description:        "Revaluación de saldos en moneda extranjera",
		IsAdjustment:       true,
		SourceType:         "currency_revaluation",
		SourceID:           workpaper.ID.String(),
		CreatedBy:          actor,
		UpdatedBy:          actor,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	for _, line := range calculationLines {
		side := debitSide
		if line.ExchangeDifference.Sign() < 0 {
			side = creditSide
			workpaper.TotalLoss = workpaper.TotalLoss.Add(line.ExchangeDifference.Abs())
		} else {
			workpaper.TotalGain = workpaper.TotalGain.Add(line.ExchangeDifference)
		}
		draft.Lines = append(draft.Lines, functionalLine(
			line.AccountID,
			side,
			line.ExchangeDifference.Abs(),
			functionalCurrency,
			nil,
			"Revaluación "+line.Currency.Code()+" "+line.AccountCode,
		))
	}
	if !workpaper.TotalLoss.IsZero() {
		draft.Lines = append(draft.Lines, functionalLine(
			lossAccountID,
			debitSide,
			workpaper.TotalLoss,
			functionalCurrency,
			nil,
			"Diferencia de cambio perdida por revaluación",
		))
	}
	if !workpaper.TotalGain.IsZero() {
		draft.Lines = append(draft.Lines, functionalLine(
			gainAccountID,
			creditSide,
			workpaper.TotalGain,
			functionalCurrency,
			nil,
			"Diferencia de cambio ganada por revaluación",
		))
	}
	for index := range draft.Lines {
		draft.Lines[index].LineNo = index + 1
	}
	workpaper.NetResult = workpaper.TotalGain.Sub(workpaper.TotalLoss)
	workpaper.Draft = draft
	if err := draft.ToEntry(EntrySource{
		Type:           "currency_revaluation",
		ID:             workpaper.ID,
		Event:          "primary",
		IdempotencyKey: draft.IdempotencyKey,
	}, actor).ValidateForPosting(); err != nil {
		return CurrencyRevaluationWorkpaper{}, fmt.Errorf("accounting: invalid revaluation draft: %w", err)
	}
	return workpaper, nil
}

func revaluationChecksum(
	closingDate time.Time,
	rates []ClosingExchangeRate,
	positions []CurrencyRevaluationPosition,
) string {
	var canonical strings.Builder
	canonical.WriteString(closingDate.UTC().Format("2006-01-02"))
	canonical.WriteByte('\n')
	for _, rate := range rates {
		fmt.Fprintf(
			&canonical,
			"R|%s|%s|%s|%s|%s\n",
			rate.Currency.Code(),
			rate.Date.UTC().Format("2006-01-02"),
			rate.Rate.String(),
			rate.Source,
			rate.SourceChecksum,
		)
	}
	for _, position := range positions {
		fmt.Fprintf(
			&canonical,
			"P|%s|%s|%s|%s|%s\n",
			position.AccountID,
			position.AccountCode,
			position.Currency.Code(),
			position.CurrencyAmount.String(),
			position.CarryingAmount.String(),
		)
	}
	hash := sha256.Sum256([]byte(canonical.String()))
	return hex.EncodeToString(hash[:])
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, current := range value {
		if (current < '0' || current > '9') && (current < 'a' || current > 'f') {
			return false
		}
	}
	return true
}
