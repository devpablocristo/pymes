package fiscal

import (
	"encoding/json"
	"testing"
)

func TestDecimalExactArithmeticAndRounding(t *testing.T) {
	t.Parallel()

	net := MustDecimal("100.01")
	rate := MustDecimal("21")
	tax, err := net.Mul(rate).Quo(NewDecimalFromInt(100), 2, RoundHalfAwayFromZero)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := tax.String(), "21"; got != want {
		t.Fatalf("tax = %s, want %s", got, want)
	}
	if got, want := net.Add(tax).String(), "121.01"; got != want {
		t.Fatalf("total = %s, want %s", got, want)
	}

	negative, err := MustDecimal("-1.005").Quantize(2, RoundHalfAwayFromZero)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := negative.String(), "-1.01"; got != want {
		t.Fatalf("negative half-away = %s, want %s", got, want)
	}

	even, err := MustDecimal("1.005").Quantize(2, RoundHalfEven)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := even.String(), "1"; got != want {
		t.Fatalf("half-even = %s, want %s", got, want)
	}
}

func TestDecimalJSONUsesStringsAndRejectsNumbers(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(MustDecimal("123.4500"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(raw), `"123.45"`; got != want {
		t.Fatalf("json = %s, want %s", got, want)
	}

	var decoded Decimal
	if err := json.Unmarshal([]byte(`"0.10"`), &decoded); err != nil {
		t.Fatal(err)
	}
	if got, want := decoded.String(), "0.1"; got != want {
		t.Fatalf("decoded = %s, want %s", got, want)
	}
	if err := json.Unmarshal([]byte(`0.1`), &decoded); err == nil {
		t.Fatal("expected JSON number to be rejected")
	}
}

func TestDecimalScaledInteger(t *testing.T) {
	t.Parallel()

	got, err := MustDecimal("123.456").ScaledInteger(2, RoundHalfAwayFromZero)
	if err != nil {
		t.Fatal(err)
	}
	if want := "12346"; got != want {
		t.Fatalf("scaled integer = %s, want %s", got, want)
	}
}
