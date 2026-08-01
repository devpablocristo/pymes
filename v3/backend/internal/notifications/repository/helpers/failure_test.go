package helpers

import "testing"

func TestStableFailureCodeRejectsUntrustedText(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input string
		want  string
	}{
		{input: " PERGO_TIMEOUT ", want: "PERGO_TIMEOUT"},
		{input: "provider leaked details", want: "PERGO_DELIVERY_FAILED"},
		{input: "", want: "PERGO_DELIVERY_FAILED"},
	} {
		if got := StableFailureCode(test.input); got != test.want {
			t.Fatalf("StableFailureCode(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}
