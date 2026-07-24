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

type InflationIndex struct {
	Period   time.Time `json:"period"`
	Value    Decimal   `json:"value"`
	Source   string    `json:"source"`
	Checksum string    `json:"checksum"`
}

func NewInflationIndex(period time.Time, value Decimal, source, sourceDocument string) (InflationIndex, error) {
	if period.IsZero() || value.Sign() <= 0 || strings.TrimSpace(source) == "" {
		return InflationIndex{}, fmt.Errorf("%w: invalid inflation index", ErrInvalidArgument)
	}
	if err := validateExchangeRateBounds(value); err != nil {
		return InflationIndex{}, err
	}
	period = time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
	hash := sha256.Sum256([]byte(sourceDocument))
	return InflationIndex{
		Period:   period,
		Value:    value,
		Source:   strings.TrimSpace(source),
		Checksum: hex.EncodeToString(hash[:]),
	}, nil
}

type InflationPosition struct {
	AccountID      uuid.UUID              `json:"account_id"`
	AccountCode    string                 `json:"account_code"`
	AccountName    string                 `json:"account_name"`
	NormalBalance  NormalBalance          `json:"normal_balance"`
	Classification MonetaryClassification `json:"monetary_classification"`
	OriginDate     time.Time              `json:"origin_date"`
	Balance        Decimal                `json:"balance"`
}

type InflationCalculationLine struct {
	AccountID     uuid.UUID     `json:"account_id"`
	AccountCode   string        `json:"account_code"`
	AccountName   string        `json:"account_name"`
	OriginDate    time.Time     `json:"origin_date"`
	OriginIndex   Decimal       `json:"origin_index"`
	ClosingIndex  Decimal       `json:"closing_index"`
	Coefficient   Decimal       `json:"coefficient"`
	Historical    Decimal       `json:"historical_amount"`
	Restated      Decimal       `json:"restated_amount"`
	Adjustment    Decimal       `json:"adjustment"`
	NormalBalance NormalBalance `json:"normal_balance"`
}

type InflationWorkpaper struct {
	ID                 uuid.UUID                  `json:"id"`
	ClosingDate        time.Time                  `json:"closing_date"`
	FunctionalCurrency Currency                   `json:"functional_currency"`
	Source             string                     `json:"source"`
	SourceChecksum     string                     `json:"source_checksum"`
	Lines              []InflationCalculationLine `json:"lines"`
	RECPAM             Decimal                    `json:"recpam"`
	Draft              Draft                      `json:"draft"`
	CreatedBy          string                     `json:"created_by"`
	CreatedAt          time.Time                  `json:"created_at"`
}

func BuildInflationWorkpaper(
	closingDate time.Time,
	functionalCurrency Currency,
	indices []InflationIndex,
	positions []InflationPosition,
	recpamAccountID uuid.UUID,
	actor string,
	now time.Time,
) (InflationWorkpaper, error) {
	if closingDate.IsZero() || recpamAccountID == uuid.Nil || strings.TrimSpace(actor) == "" {
		return InflationWorkpaper{}, fmt.Errorf("%w: incomplete inflation calculation", ErrInvalidArgument)
	}
	indexByMonth := make(map[string]InflationIndex, len(indices))
	for _, index := range indices {
		indexByMonth[indexMonth(index.Period)] = index
	}
	closingIndex, ok := indexByMonth[indexMonth(closingDate)]
	if !ok {
		return InflationWorkpaper{}, fmt.Errorf("%w: closing month", ErrInflationIncomplete)
	}
	workpaper := InflationWorkpaper{
		ID:                 uuid.New(),
		ClosingDate:        closingDate,
		FunctionalCurrency: functionalCurrency,
		Source:             closingIndex.Source,
		SourceChecksum:     closingIndex.Checksum,
		CreatedBy:          actor,
		CreatedAt:          now,
	}
	draft := Draft{
		ID:                 uuid.New(),
		IdempotencyKey:     "inflation:" + workpaper.ID.String(),
		Date:               closingDate,
		Kind:               EntryInflation,
		FunctionalCurrency: functionalCurrency,
		Currency:           functionalCurrency,
		ExchangeRate:       One,
		Description:        "Ajuste por inflación",
		IsAdjustment:       true,
		CreatedBy:          actor,
		UpdatedBy:          actor,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	var netDebit Decimal
	for _, position := range positions {
		if position.Classification != NonMonetary || position.Balance.IsZero() {
			continue
		}
		originIndex, found := indexByMonth[indexMonth(position.OriginDate)]
		if !found {
			return InflationWorkpaper{}, fmt.Errorf("%w: account %s origin month", ErrInflationIncomplete, position.AccountCode)
		}
		coefficient, err := closingIndex.Value.Quo(originIndex.Value, 10)
		if err != nil {
			return InflationWorkpaper{}, err
		}
		restated := functionalCurrency.Round(position.Balance.Mul(coefficient))
		adjustment := restated.Sub(position.Balance)
		if adjustment.IsZero() {
			continue
		}
		workpaper.Lines = append(workpaper.Lines, InflationCalculationLine{
			AccountID:     position.AccountID,
			AccountCode:   position.AccountCode,
			AccountName:   position.AccountName,
			OriginDate:    position.OriginDate,
			OriginIndex:   originIndex.Value,
			ClosingIndex:  closingIndex.Value,
			Coefficient:   coefficient,
			Historical:    position.Balance,
			Restated:      restated,
			Adjustment:    adjustment,
			NormalBalance: position.NormalBalance,
		})
		side := debitSide
		if position.NormalBalance == NormalCredit {
			side = creditSide
		}
		if adjustment.Sign() < 0 {
			if side == debitSide {
				side = creditSide
			} else {
				side = debitSide
			}
		}
		draft.Lines = append(draft.Lines, functionalLine(
			position.AccountID,
			side,
			adjustment.Abs(),
			functionalCurrency,
			nil,
			"Ajuste por inflación "+position.AccountCode,
		))
		if side == debitSide {
			netDebit = netDebit.Add(adjustment.Abs())
		} else {
			netDebit = netDebit.Sub(adjustment.Abs())
		}
	}
	if !netDebit.IsZero() {
		recpamSide := creditSide
		if netDebit.Sign() < 0 {
			recpamSide = debitSide
		}
		draft.Lines = append(draft.Lines, functionalLine(
			recpamAccountID,
			recpamSide,
			netDebit.Abs(),
			functionalCurrency,
			nil,
			"RECPAM",
		))
		workpaper.RECPAM = netDebit.Neg()
	}
	for index := range draft.Lines {
		draft.Lines[index].LineNo = index + 1
	}
	sort.Slice(workpaper.Lines, func(i, j int) bool {
		return workpaper.Lines[i].AccountCode < workpaper.Lines[j].AccountCode
	})
	workpaper.Draft = draft
	return workpaper, nil
}

func indexMonth(value time.Time) string {
	return value.UTC().Format("2006-01")
}
