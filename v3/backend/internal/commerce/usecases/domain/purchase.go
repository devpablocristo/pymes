package domain

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

var purchaseDecimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)

var purchaseVATRates = map[string]*big.Rat{
	"0":    big.NewRat(0, 1),
	"2.5":  big.NewRat(25, 10),
	"5":    big.NewRat(5, 1),
	"10.5": big.NewRat(105, 10),
	"21":   big.NewRat(21, 1),
	"27":   big.NewRat(27, 1),
}

// ValidateAccountingAmounts enforces the exact monetary representation that
// crosses the Pymes -> Accounting boundary. Invoice amounts have two decimal
// places; the exchange rate has up to six. No float64 conversion is allowed.
func (p Purchase) ValidateAccountingAmounts() error {
	issueDate, err := time.Parse(time.DateOnly, p.IssueDate)
	if err != nil || issueDate.Format(time.DateOnly) != p.IssueDate {
		return fmt.Errorf("VALIDATION_ERROR: issue_date must be YYYY-MM-DD")
	}
	total, err := purchaseDecimal("amount", p.Total.Amount, 2, false)
	if err != nil {
		return err
	}
	net, err := purchaseDecimal("net_amount", p.NetAmount, 2, true)
	if err != nil {
		return err
	}
	exempt, err := purchaseDecimal("exempt_amount", p.ExemptAmount, 2, true)
	if err != nil {
		return err
	}
	if p.Total.Currency != "ARS" && p.Total.Currency != "USD" && p.Total.Currency != "EUR" {
		return fmt.Errorf("VALIDATION_ERROR: unsupported purchase currency")
	}
	if err := ValidateExchangeRate(p.Total.Currency, p.ExchangeRate); err != nil {
		return err
	}

	baseTotal := new(big.Rat)
	taxTotal := new(big.Rat)
	seenRates := make(map[string]struct{}, len(p.VATBreakdown))
	for index, item := range p.VATBreakdown {
		rate, ok := purchaseVATRates[item.Rate]
		if !ok {
			return fmt.Errorf("VALIDATION_ERROR: vat_breakdown[%d].rate is unsupported", index)
		}
		if _, duplicate := seenRates[item.Rate]; duplicate {
			return fmt.Errorf("VALIDATION_ERROR: duplicate VAT rate %s", item.Rate)
		}
		seenRates[item.Rate] = struct{}{}
		base, parseErr := purchaseDecimal(fmt.Sprintf("vat_breakdown[%d].base_amount", index), item.BaseAmount, 2, false)
		if parseErr != nil {
			return parseErr
		}
		tax, parseErr := purchaseDecimal(fmt.Sprintf("vat_breakdown[%d].tax_amount", index), item.TaxAmount, 2, true)
		if parseErr != nil {
			return parseErr
		}
		expected := new(big.Rat).Mul(base, rate)
		expected.Quo(expected, big.NewRat(100, 1))
		if roundPurchaseMoney(expected).Cmp(roundPurchaseMoney(tax)) != 0 {
			return fmt.Errorf("VALIDATION_ERROR: vat_breakdown[%d].tax_amount does not match rate", index)
		}
		baseTotal.Add(baseTotal, base)
		taxTotal.Add(taxTotal, tax)
	}
	if baseTotal.Cmp(net) != 0 {
		return fmt.Errorf("VALIDATION_ERROR: net_amount must equal VAT bases")
	}
	components := new(big.Rat).Add(new(big.Rat).Set(net), exempt)
	components.Add(components, taxTotal)
	if components.Cmp(total) != 0 {
		return fmt.Errorf("VALIDATION_ERROR: amount must equal net + exempt + VAT")
	}
	return nil
}

// ValidateExchangeRate applies the same exact-decimal currency rule to every
// commercial document before it can enter an asynchronous accounting flow.
func ValidateExchangeRate(currency, raw string) error {
	switch currency {
	case "ARS":
		if raw == "" {
			return nil
		}
		rate, err := purchaseDecimal("exchange_rate", raw, 6, false)
		if err != nil {
			return err
		}
		if rate.Cmp(big.NewRat(1, 1)) != 0 {
			return fmt.Errorf("VALIDATION_ERROR: ARS exchange_rate must be one")
		}
		return nil
	case "USD", "EUR":
		if _, err := purchaseDecimal("exchange_rate", raw, 6, false); err != nil {
			return fmt.Errorf("VALIDATION_ERROR: foreign-currency %w", err)
		}
		return nil
	default:
		return fmt.Errorf("VALIDATION_ERROR: unsupported currency")
	}
}

func purchaseDecimal(field, raw string, scale int, zeroAllowed bool) (*big.Rat, error) {
	if !purchaseDecimalPattern.MatchString(raw) {
		return nil, fmt.Errorf("VALIDATION_ERROR: %s must be an unsigned base-ten decimal", field)
	}
	parts := strings.SplitN(raw, ".", 2)
	if len(parts[0]) > 14 || (len(parts) == 2 && len(parts[1]) > scale) {
		return nil, fmt.Errorf("VALIDATION_ERROR: %s exceeds numeric(20,%d)", field, scale)
	}
	value, ok := new(big.Rat).SetString(raw)
	if !ok || value.Sign() < 0 || (!zeroAllowed && value.Sign() == 0) {
		return nil, fmt.Errorf("VALIDATION_ERROR: %s must be positive", field)
	}
	return value, nil
}

func roundPurchaseMoney(value *big.Rat) *big.Int {
	numerator := new(big.Int).Mul(value.Num(), big.NewInt(100))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, value.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(value.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient
}
