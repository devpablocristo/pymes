package accounting

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// Decimal is an immutable base-10 decimal. Its zero value is 0.
//
// The canonical representation is kept as text on purpose: it makes accidental
// conversion through binary floating point impossible at API and database
// boundaries. Arithmetic is performed with math/big integers.
type Decimal struct {
	canonical string
}

var (
	Zero = Decimal{}
	One  = MustDecimal("1")
)

// ParseDecimal parses a plain base-10 number. Scientific notation, NaN and
// infinities are deliberately unsupported.
func ParseDecimal(value string) (Decimal, error) {
	canonical, err := canonicalDecimal(value)
	if err != nil {
		return Decimal{}, err
	}
	if canonical == "0" {
		return Decimal{}, nil
	}
	return Decimal{canonical: canonical}, nil
}

// ParseAmount parses a value accepted by accounting numeric(24,6) columns.
func ParseAmount(value string) (Decimal, error) {
	decimal, err := ParseDecimal(value)
	if err != nil {
		return Decimal{}, err
	}
	if err := validateAmountBounds(decimal); err != nil {
		return Decimal{}, err
	}
	return decimal, nil
}

// ParseExchangeRate parses a value accepted by accounting numeric(24,10)
// columns. Exchange rates must be greater than zero.
func ParseExchangeRate(value string) (Decimal, error) {
	decimal, err := ParseDecimal(value)
	if err != nil {
		return Decimal{}, err
	}
	if decimal.Sign() <= 0 {
		return Decimal{}, fmt.Errorf("%w: exchange rate must be positive", ErrInvalidDecimal)
	}
	if err := validateExchangeRateBounds(decimal); err != nil {
		return Decimal{}, err
	}
	return decimal, nil
}

func validateAmountBounds(decimal Decimal) error {
	if !decimal.fitsNumeric(24, 6) {
		return fmt.Errorf("%w: amount is outside numeric(24,6)", ErrInvalidDecimal)
	}
	return nil
}

func validateExchangeRateBounds(decimal Decimal) error {
	if !decimal.fitsNumeric(24, 10) {
		return fmt.Errorf("%w: exchange rate is outside numeric(24,10)", ErrInvalidDecimal)
	}
	return nil
}

func MustDecimal(value string) Decimal {
	decimal, err := ParseDecimal(value)
	if err != nil {
		panic(err)
	}
	return decimal
}

func (d Decimal) String() string {
	if d.canonical == "" {
		return "0"
	}
	return d.canonical
}

func (d Decimal) IsZero() bool {
	return d.canonical == "" || d.canonical == "0"
}

func (d Decimal) Sign() int {
	if d.IsZero() {
		return 0
	}
	if strings.HasPrefix(d.canonical, "-") {
		return -1
	}
	return 1
}

func (d Decimal) Scale() int {
	text := d.String()
	if dot := strings.IndexByte(text, '.'); dot >= 0 {
		return len(text) - dot - 1
	}
	return 0
}

func (d Decimal) Precision() int {
	text := strings.TrimPrefix(d.String(), "-")
	text = strings.ReplaceAll(text, ".", "")
	return len(strings.TrimLeft(text, "0"))
}

func (d Decimal) fitsNumeric(precision, scale int) bool {
	if d.Scale() > scale {
		return false
	}
	text := strings.TrimPrefix(d.String(), "-")
	if dot := strings.IndexByte(text, '.'); dot >= 0 {
		text = text[:dot]
	}
	text = strings.TrimLeft(text, "0")
	integerDigits := len(text)
	return integerDigits <= precision-scale
}

func (d Decimal) Neg() Decimal {
	if d.IsZero() {
		return Zero
	}
	if strings.HasPrefix(d.canonical, "-") {
		return Decimal{canonical: strings.TrimPrefix(d.canonical, "-")}
	}
	return Decimal{canonical: "-" + d.canonical}
}

func (d Decimal) Abs() Decimal {
	if d.Sign() < 0 {
		return d.Neg()
	}
	return d
}

func (d Decimal) Add(other Decimal) Decimal {
	left, leftScale := d.coefficient()
	right, rightScale := other.coefficient()
	scale := max(leftScale, rightScale)
	left.Mul(left, powerOfTen(scale-leftScale))
	right.Mul(right, powerOfTen(scale-rightScale))
	left.Add(left, right)
	return decimalFromCoefficient(left, scale)
}

func (d Decimal) Sub(other Decimal) Decimal {
	return d.Add(other.Neg())
}

func (d Decimal) Mul(other Decimal) Decimal {
	left, leftScale := d.coefficient()
	right, rightScale := other.coefficient()
	left.Mul(left, right)
	return decimalFromCoefficient(left, leftScale+rightScale)
}

// Quo divides d by other and rounds half away from zero at scale decimal
// places.
func (d Decimal) Quo(other Decimal, scale int) (Decimal, error) {
	if scale < 0 {
		return Decimal{}, fmt.Errorf("%w: negative division scale", ErrInvalidDecimal)
	}
	if other.IsZero() {
		return Decimal{}, ErrDivisionByZero
	}
	left, leftScale := d.coefficient()
	right, rightScale := other.coefficient()

	numerator := new(big.Int).Mul(left, powerOfTen(rightScale+scale))
	denominator := new(big.Int).Mul(right, powerOfTen(leftScale))
	coefficient := roundedQuotient(numerator, denominator)
	return decimalFromCoefficient(coefficient, scale), nil
}

// Round returns d rounded half away from zero at scale decimal places.
func (d Decimal) Round(scale int) Decimal {
	if scale < 0 {
		panic("accounting.Decimal.Round: negative scale")
	}
	coefficient, currentScale := d.coefficient()
	if currentScale <= scale {
		return d
	}
	divisor := powerOfTen(currentScale - scale)
	return decimalFromCoefficient(roundedQuotient(coefficient, divisor), scale)
}

func (d Decimal) Cmp(other Decimal) int {
	left, leftScale := d.coefficient()
	right, rightScale := other.coefficient()
	scale := max(leftScale, rightScale)
	left.Mul(left, powerOfTen(scale-leftScale))
	right.Mul(right, powerOfTen(scale-rightScale))
	return left.Cmp(right)
}

func (d Decimal) Equal(other Decimal) bool {
	return d.Cmp(other) == 0
}

func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Decimal) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.New("accounting.Decimal: UnmarshalJSON on nil pointer")
	}
	var encoded string
	if len(data) == 0 || data[0] != '"' {
		return fmt.Errorf("%w: decimal JSON value must be a string", ErrInvalidDecimal)
	}
	if err := json.Unmarshal(data, &encoded); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidDecimal, err)
	}
	parsed, err := ParseDecimal(encoded)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Scan implements sql.Scanner and accepts only exact database encodings.
func (d *Decimal) Scan(src any) error {
	if d == nil {
		return errors.New("accounting.Decimal: Scan on nil pointer")
	}
	var value string
	switch typed := src.(type) {
	case nil:
		*d = Zero
		return nil
	case string:
		value = typed
	case []byte:
		value = string(typed)
	case int64:
		value = fmt.Sprintf("%d", typed)
	default:
		return fmt.Errorf("%w: cannot scan %T without risking precision loss", ErrInvalidDecimal, src)
	}
	parsed, err := ParseDecimal(value)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Value implements driver.Valuer using the exact decimal string.
func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}

func (d Decimal) coefficient() (*big.Int, int) {
	text := d.String()
	negative := strings.HasPrefix(text, "-")
	text = strings.TrimPrefix(text, "-")
	scale := 0
	if dot := strings.IndexByte(text, '.'); dot >= 0 {
		scale = len(text) - dot - 1
		text = text[:dot] + text[dot+1:]
	}
	coefficient := new(big.Int)
	coefficient.SetString(text, 10)
	if negative {
		coefficient.Neg(coefficient)
	}
	return coefficient, scale
}

func decimalFromCoefficient(coefficient *big.Int, scale int) Decimal {
	if coefficient.Sign() == 0 {
		return Zero
	}
	negative := coefficient.Sign() < 0
	digits := new(big.Int).Abs(new(big.Int).Set(coefficient)).String()
	if scale > 0 {
		if len(digits) <= scale {
			digits = strings.Repeat("0", scale-len(digits)+1) + digits
		}
		dot := len(digits) - scale
		digits = digits[:dot] + "." + digits[dot:]
	}
	if negative {
		digits = "-" + digits
	}
	parsed, err := ParseDecimal(digits)
	if err != nil {
		panic(err)
	}
	return parsed
}

func roundedQuotient(numerator, denominator *big.Int) *big.Int {
	if denominator.Sign() == 0 {
		panic("accounting.roundedQuotient: zero denominator")
	}
	negative := numerator.Sign()*denominator.Sign() < 0
	absNumerator := new(big.Int).Abs(new(big.Int).Set(numerator))
	absDenominator := new(big.Int).Abs(new(big.Int).Set(denominator))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(absNumerator, absDenominator, remainder)
	twiceRemainder := new(big.Int).Lsh(remainder, 1)
	if twiceRemainder.Cmp(absDenominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if negative {
		quotient.Neg(quotient)
	}
	return quotient
}

func canonicalDecimal(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%w: empty value", ErrInvalidDecimal)
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
		return "", fmt.Errorf("%w: missing digits", ErrInvalidDecimal)
	}
	if strings.ContainsAny(value, "eE") {
		return "", fmt.Errorf("%w: scientific notation is not supported", ErrInvalidDecimal)
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
		return "", fmt.Errorf("%w: malformed decimal", ErrInvalidDecimal)
	}
	for _, part := range parts {
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return "", fmt.Errorf("%w: malformed decimal", ErrInvalidDecimal)
			}
		}
	}
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	if integer == "0" && fraction == "" {
		return "0", nil
	}
	canonical := integer
	if fraction != "" {
		canonical += "." + fraction
	}
	if negative {
		canonical = "-" + canonical
	}
	return canonical, nil
}

func powerOfTen(exponent int) *big.Int {
	if exponent < 0 {
		panic("accounting.powerOfTen: negative exponent")
	}
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil)
}
