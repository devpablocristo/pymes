package accounting

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDecimalArithmeticIsExact(t *testing.T) {
	t.Parallel()

	oneTenth := MustDecimal("0.1")
	twoTenths := MustDecimal("0.2")
	if got := oneTenth.Add(twoTenths).String(); got != "0.3" {
		t.Fatalf("0.1 + 0.2 = %s, want 0.3", got)
	}
	if got := MustDecimal("10.50").Mul(MustDecimal("21")).String(); got != "220.5" {
		t.Fatalf("10.50 * 21 = %s, want 220.5", got)
	}
	quotient, err := MustDecimal("1").Quo(MustDecimal("3"), 6)
	if err != nil {
		t.Fatal(err)
	}
	if got := quotient.String(); got != "0.333333" {
		t.Fatalf("1 / 3 = %s, want 0.333333", got)
	}
}

func TestDecimalRoundsHalfAwayFromZero(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"1.005":  "1.01",
		"1.004":  "1",
		"-1.005": "-1.01",
		"-1.004": "-1",
	}
	for input, expected := range cases {
		if actual := MustDecimal(input).Round(2).String(); actual != expected {
			t.Errorf("%s rounded to 2 = %s, want %s", input, actual, expected)
		}
	}
}

func TestDecimalJSONRequiresString(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(MustDecimal("123.45"))
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `"123.45"` {
		t.Fatalf("encoded decimal = %s", encoded)
	}

	var fromString Decimal
	if err := json.Unmarshal([]byte(`"123.450"`), &fromString); err != nil {
		t.Fatal(err)
	}
	if fromString.String() != "123.45" {
		t.Fatalf("decoded decimal = %s", fromString)
	}

	var fromNumber Decimal
	err = json.Unmarshal([]byte(`123.45`), &fromNumber)
	if !errors.Is(err, ErrInvalidDecimal) {
		t.Fatalf("numeric JSON error = %v, want ErrInvalidDecimal", err)
	}
}

func TestDecimalRejectsLossyDatabaseScan(t *testing.T) {
	t.Parallel()

	var decimal Decimal
	if err := decimal.Scan(float64(0.1)); !errors.Is(err, ErrInvalidDecimal) {
		t.Fatalf("float scan error = %v, want ErrInvalidDecimal", err)
	}
	if err := decimal.Scan([]byte("42.250000")); err != nil {
		t.Fatal(err)
	}
	if decimal.String() != "42.25" {
		t.Fatalf("scanned decimal = %s", decimal)
	}
}

func TestAmountAndExchangeRateDatabaseBounds(t *testing.T) {
	t.Parallel()

	if _, err := ParseAmount("1.0000001"); !errors.Is(err, ErrInvalidDecimal) {
		t.Fatalf("amount scale error = %v", err)
	}
	if _, err := ParseExchangeRate("0"); !errors.Is(err, ErrInvalidDecimal) {
		t.Fatalf("zero rate error = %v", err)
	}
	if _, err := ParseAmount("1000000000000000000"); !errors.Is(err, ErrInvalidDecimal) {
		t.Fatalf("amount integer-width error = %v", err)
	}
	if amount, err := ParseAmount("999999999999999999.999999"); err != nil ||
		amount.String() != "999999999999999999.999999" {
		t.Fatalf("maximum amount = %v, %v", amount, err)
	}
	if _, err := ParseExchangeRate("100000000000000"); !errors.Is(err, ErrInvalidDecimal) {
		t.Fatalf("rate integer-width error = %v", err)
	}
	if rate, err := ParseExchangeRate("1234.1234567890"); err != nil || rate.String() != "1234.123456789" {
		t.Fatalf("rate = %v, %v", rate, err)
	}
}
