package fiscal

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// Decimal is an exact base-10 number. Its zero value is the number zero.
//
// Monetary values cross API boundaries as JSON strings and database boundaries
// as PostgreSQL-compatible numeric strings. Decimal deliberately has no
// constructor or conversion involving binary floating-point values.
type Decimal struct {
	coefficient big.Int
	scale       int32
}

// RoundingMode makes every lossy decimal operation explicit.
type RoundingMode uint8

const (
	RoundTowardZero RoundingMode = iota
	RoundHalfAwayFromZero
	RoundHalfEven
)

const maxDecimalScale = 28

var (
	errInvalidDecimal = errors.New("invalid decimal")
	bigTen            = big.NewInt(10)
)

// ParseDecimal parses a plain base-10 number without exponent notation.
func ParseDecimal(raw string) (Decimal, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return Decimal{}, fmt.Errorf("%w: empty value", errInvalidDecimal)
	}

	negative := false
	switch value[0] {
	case '-':
		negative = true
		value = value[1:]
	case '+':
		value = value[1:]
	}
	if value == "" {
		return Decimal{}, fmt.Errorf("%w: sign without digits", errInvalidDecimal)
	}
	if strings.Count(value, ".") > 1 {
		return Decimal{}, fmt.Errorf("%w: multiple decimal separators", errInvalidDecimal)
	}

	integerPart, fractionalPart, hasFraction := strings.Cut(value, ".")
	if integerPart == "" {
		integerPart = "0"
	}
	if hasFraction && fractionalPart == "" {
		return Decimal{}, fmt.Errorf("%w: missing fractional digits", errInvalidDecimal)
	}
	if !decimalDigits(integerPart) || (hasFraction && !decimalDigits(fractionalPart)) {
		return Decimal{}, fmt.Errorf("%w: %q", errInvalidDecimal, raw)
	}
	if len(fractionalPart) > maxDecimalScale {
		return Decimal{}, fmt.Errorf("%w: scale exceeds %d digits", errInvalidDecimal, maxDecimalScale)
	}

	digits := strings.TrimLeft(integerPart+fractionalPart, "0")
	if digits == "" {
		return Decimal{}, nil
	}
	var coefficient big.Int
	if _, ok := coefficient.SetString(digits, 10); !ok {
		return Decimal{}, fmt.Errorf("%w: %q", errInvalidDecimal, raw)
	}
	if negative {
		coefficient.Neg(&coefficient)
	}
	return normalizeDecimal(coefficient, int32(len(fractionalPart))), nil
}

// MustDecimal is intended for constants, fixtures, and other fail-fast setup.
func MustDecimal(raw string) Decimal {
	value, err := ParseDecimal(raw)
	if err != nil {
		panic(err)
	}
	return value
}

// NewDecimalFromInt constructs an exact integer decimal.
func NewDecimalFromInt(value int64) Decimal {
	var coefficient big.Int
	coefficient.SetInt64(value)
	return Decimal{coefficient: coefficient}
}

func normalizeDecimal(coefficient big.Int, scale int32) Decimal {
	if coefficient.Sign() == 0 {
		return Decimal{}
	}
	for scale > 0 {
		var quotient, remainder big.Int
		quotient.QuoRem(&coefficient, bigTen, &remainder)
		if remainder.Sign() != 0 {
			break
		}
		coefficient.Set(&quotient)
		scale--
	}
	return Decimal{coefficient: coefficient, scale: scale}
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func pow10(exponent int32) *big.Int {
	if exponent <= 0 {
		return big.NewInt(1)
	}
	return new(big.Int).Exp(bigTen, big.NewInt(int64(exponent)), nil)
}

// String returns the canonical, non-exponent decimal representation.
func (d Decimal) String() string {
	if d.coefficient.Sign() == 0 {
		return "0"
	}
	negative := d.coefficient.Sign() < 0
	var absolute big.Int
	absolute.Abs(&d.coefficient)
	digits := absolute.String()
	if d.scale > 0 {
		scale := int(d.scale)
		if len(digits) <= scale {
			digits = strings.Repeat("0", scale-len(digits)+1) + digits
		}
		split := len(digits) - scale
		digits = digits[:split] + "." + digits[split:]
	}
	if negative {
		return "-" + digits
	}
	return digits
}

// FormatFixed renders the value with exactly scale fractional digits.
func (d Decimal) FormatFixed(scale int32, mode RoundingMode) (string, error) {
	rounded, err := d.Quantize(scale, mode)
	if err != nil {
		return "", err
	}
	value := rounded.String()
	if scale == 0 {
		if dot := strings.IndexByte(value, '.'); dot >= 0 {
			return value[:dot], nil
		}
		return value, nil
	}

	integerPart, fractionalPart, found := strings.Cut(value, ".")
	if !found {
		fractionalPart = ""
	}
	if len(fractionalPart) < int(scale) {
		fractionalPart += strings.Repeat("0", int(scale)-len(fractionalPart))
	}
	return integerPart + "." + fractionalPart, nil
}

// Add returns d + other.
func (d Decimal) Add(other Decimal) Decimal {
	scale := d.scale
	if other.scale > scale {
		scale = other.scale
	}
	left := d.scaledCoefficient(scale)
	right := other.scaledCoefficient(scale)
	var coefficient big.Int
	coefficient.Add(left, right)
	return normalizeDecimal(coefficient, scale)
}

// Sub returns d - other.
func (d Decimal) Sub(other Decimal) Decimal {
	return d.Add(other.Neg())
}

// Mul returns d * other without losing precision.
func (d Decimal) Mul(other Decimal) Decimal {
	var coefficient big.Int
	coefficient.Mul(&d.coefficient, &other.coefficient)
	return normalizeDecimal(coefficient, d.scale+other.scale)
}

// Quo divides d by other and rounds to the requested maximum scale.
func (d Decimal) Quo(other Decimal, scale int32, mode RoundingMode) (Decimal, error) {
	if other.IsZero() {
		return Decimal{}, errors.New("decimal division by zero")
	}
	if scale < 0 || scale > maxDecimalScale {
		return Decimal{}, fmt.Errorf("decimal scale must be between 0 and %d", maxDecimalScale)
	}

	var numerator, denominator big.Int
	numerator.Set(&d.coefficient)
	denominator.Set(&other.coefficient)
	exponent := scale + other.scale - d.scale
	if exponent >= 0 {
		numerator.Mul(&numerator, pow10(exponent))
	} else {
		denominator.Mul(&denominator, pow10(-exponent))
	}
	coefficient, err := roundedQuotient(&numerator, &denominator, mode)
	if err != nil {
		return Decimal{}, err
	}
	return normalizeDecimal(*coefficient, scale), nil
}

// Quantize rounds d to no more than scale fractional digits.
func (d Decimal) Quantize(scale int32, mode RoundingMode) (Decimal, error) {
	if scale < 0 || scale > maxDecimalScale {
		return Decimal{}, fmt.Errorf("decimal scale must be between 0 and %d", maxDecimalScale)
	}
	if d.scale <= scale {
		return d, nil
	}
	divisor := pow10(d.scale - scale)
	coefficient, err := roundedQuotient(&d.coefficient, divisor, mode)
	if err != nil {
		return Decimal{}, err
	}
	return normalizeDecimal(*coefficient, scale), nil
}

func roundedQuotient(numerator, denominator *big.Int, mode RoundingMode) (*big.Int, error) {
	if denominator.Sign() == 0 {
		return nil, errors.New("decimal division by zero")
	}
	var quotient, remainder big.Int
	quotient.QuoRem(numerator, denominator, &remainder)
	if remainder.Sign() == 0 || mode == RoundTowardZero {
		return &quotient, nil
	}
	if mode != RoundHalfAwayFromZero && mode != RoundHalfEven {
		return nil, fmt.Errorf("unsupported rounding mode %d", mode)
	}

	var twiceRemainder, absoluteDenominator big.Int
	twiceRemainder.Abs(&remainder)
	twiceRemainder.Mul(&twiceRemainder, big.NewInt(2))
	absoluteDenominator.Abs(denominator)
	comparison := twiceRemainder.Cmp(&absoluteDenominator)
	roundAway := comparison > 0
	if comparison == 0 {
		roundAway = mode == RoundHalfAwayFromZero || quotient.Bit(0) == 1
	}
	if roundAway {
		sign := numerator.Sign() * denominator.Sign()
		quotient.Add(&quotient, big.NewInt(int64(sign)))
	}
	return &quotient, nil
}

func (d Decimal) scaledCoefficient(scale int32) *big.Int {
	var coefficient big.Int
	coefficient.Set(&d.coefficient)
	if scale > d.scale {
		coefficient.Mul(&coefficient, pow10(scale-d.scale))
	}
	return &coefficient
}

// ScaledInteger returns the rounded integer representation at the requested
// scale. For example, 12.34 at scale 2 returns "1234".
func (d Decimal) ScaledInteger(scale int32, mode RoundingMode) (string, error) {
	rounded, err := d.Quantize(scale, mode)
	if err != nil {
		return "", err
	}
	return rounded.scaledCoefficient(scale).String(), nil
}

// Cmp compares d and other.
func (d Decimal) Cmp(other Decimal) int {
	scale := d.scale
	if other.scale > scale {
		scale = other.scale
	}
	return d.scaledCoefficient(scale).Cmp(other.scaledCoefficient(scale))
}

func (d Decimal) Equal(other Decimal) bool { return d.Cmp(other) == 0 }
func (d Decimal) IsZero() bool             { return d.coefficient.Sign() == 0 }
func (d Decimal) IsNegative() bool         { return d.coefficient.Sign() < 0 }

// Neg returns -d.
func (d Decimal) Neg() Decimal {
	var coefficient big.Int
	coefficient.Neg(&d.coefficient)
	return Decimal{coefficient: coefficient, scale: d.scale}
}

// Abs returns the absolute value of d.
func (d Decimal) Abs() Decimal {
	if d.IsNegative() {
		return d.Neg()
	}
	return d
}

// MarshalJSON enforces the public API contract: exact decimals are strings.
func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON rejects JSON numbers so callers cannot accidentally round
// through a binary floating-point decoder before reaching the domain.
func (d *Decimal) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.New("cannot unmarshal decimal into nil receiver")
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("decimal must be a JSON string")
	}
	parsed, err := ParseDecimal(value)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Decimal) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

func (d *Decimal) UnmarshalText(text []byte) error {
	parsed, err := ParseDecimal(string(text))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Value implements database/sql/driver.Valuer for PostgreSQL numeric columns.
func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}

// Scan implements database/sql.Scanner without accepting binary floats.
func (d *Decimal) Scan(source any) error {
	if d == nil {
		return errors.New("cannot scan decimal into nil receiver")
	}
	var raw string
	switch value := source.(type) {
	case string:
		raw = value
	case []byte:
		raw = string(value)
	case nil:
		return errors.New("cannot scan NULL into Decimal")
	default:
		return fmt.Errorf("cannot scan %T into Decimal", source)
	}
	parsed, err := ParseDecimal(raw)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
