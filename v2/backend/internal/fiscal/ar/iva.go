package ar

import (
	"errors"
	"fmt"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

type TaxCategory string

const (
	Taxable TaxCategory = "taxable"
	Exempt  TaxCategory = "exempt"
	Untaxed TaxCategory = "untaxed"
)

type TaxableAmount struct {
	Category TaxCategory
	Amount   fiscal.Decimal
	Rate     fiscal.Decimal
}

type VATBreakdown struct {
	ID         int
	Rate       fiscal.Decimal
	BaseAmount fiscal.Decimal
	Amount     fiscal.Decimal
}

type Tribute struct {
	ID          int
	Description string
	BaseAmount  fiscal.Decimal
	Rate        fiscal.Decimal
	Amount      fiscal.Decimal
}

type Totals struct {
	NetTaxed   fiscal.Decimal
	NetUntaxed fiscal.Decimal
	Exempt     fiscal.Decimal
	Tributes   fiscal.Decimal
	VAT        fiscal.Decimal
	Total      fiscal.Decimal
	VATLines   []VATBreakdown
}

func CalculateTotals(voucherType VoucherType, amounts []TaxableAmount, tributes []Tribute) (Totals, error) {
	if !voucherType.ValidMVP() {
		return Totals{}, fmt.Errorf("unsupported Argentina voucher type %d", voucherType)
	}
	if len(amounts) == 0 {
		return Totals{}, errors.New("at least one taxable amount is required")
	}

	var totals Totals
	type accumulator struct {
		rate fiscal.Decimal
		base fiscal.Decimal
	}
	vatByRate := make(map[string]accumulator)
	order := make([]string, 0, len(amounts))

	for index, amount := range amounts {
		rounded, err := amount.Amount.Quantize(2, fiscal.RoundHalfAwayFromZero)
		if err != nil {
			return Totals{}, err
		}
		if rounded.IsNegative() {
			return Totals{}, fmt.Errorf("amount %d cannot be negative", index)
		}
		if voucherType.IsTypeC() {
			totals.NetTaxed = totals.NetTaxed.Add(rounded)
			continue
		}
		switch amount.Category {
		case Taxable:
			if _, supported := VATIDForRate(amount.Rate); !supported {
				return Totals{}, fmt.Errorf("unsupported ARCA VAT rate %s", amount.Rate)
			}
			key := amount.Rate.String()
			entry, found := vatByRate[key]
			if !found {
				entry.rate = amount.Rate
				order = append(order, key)
			}
			entry.base = entry.base.Add(rounded)
			vatByRate[key] = entry
			totals.NetTaxed = totals.NetTaxed.Add(rounded)
		case Exempt:
			totals.Exempt = totals.Exempt.Add(rounded)
		case Untaxed:
			totals.NetUntaxed = totals.NetUntaxed.Add(rounded)
		default:
			return Totals{}, fmt.Errorf("amount %d has invalid tax category %q", index, amount.Category)
		}
	}

	if !voucherType.IsTypeC() {
		for _, key := range order {
			entry := vatByRate[key]
			base, err := entry.base.Quantize(2, fiscal.RoundHalfAwayFromZero)
			if err != nil {
				return Totals{}, err
			}
			tax, err := base.Mul(entry.rate).Quo(
				fiscal.NewDecimalFromInt(100), 2, fiscal.RoundHalfAwayFromZero,
			)
			if err != nil {
				return Totals{}, err
			}
			id, _ := VATIDForRate(entry.rate)
			totals.VATLines = append(totals.VATLines, VATBreakdown{
				ID: id, Rate: entry.rate, BaseAmount: base, Amount: tax,
			})
			totals.VAT = totals.VAT.Add(tax)
		}
	}

	for index, tribute := range tributes {
		if tribute.ID <= 0 || tribute.Amount.IsNegative() {
			return Totals{}, fmt.Errorf("tribute %d is invalid", index)
		}
		if tribute.ID == 99 && tribute.Description == "" {
			return Totals{}, errors.New("ARCA tribute 99 requires a description")
		}
		amount, err := tribute.Amount.Quantize(2, fiscal.RoundHalfAwayFromZero)
		if err != nil {
			return Totals{}, err
		}
		totals.Tributes = totals.Tributes.Add(amount)
	}

	totals.Total = totals.NetTaxed.
		Add(totals.NetUntaxed).
		Add(totals.Exempt).
		Add(totals.VAT).
		Add(totals.Tributes)
	return totals, nil
}
