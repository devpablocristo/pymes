package domain

import (
	"strings"
	"testing"
)

func TestPurchaseValidateAccountingAmounts(t *testing.T) {
	valid := Purchase{
		IssueDate:    "2026-07-31",
		Total:        Money{Amount: "126.00", Currency: "ARS"},
		NetAmount:    "100.00",
		ExemptAmount: "5.00",
		VATBreakdown: []VATBreakdownItem{{
			Rate: "21", BaseAmount: "100.00", TaxAmount: "21.00",
		}},
	}
	if err := valid.ValidateAccountingAmounts(); err != nil {
		t.Fatalf("valid purchase: %v", err)
	}

	foreign := valid
	foreign.Total.Currency = "USD"
	if err := foreign.ValidateAccountingAmounts(); err == nil || !strings.Contains(err.Error(), "exchange_rate") {
		t.Fatalf("expected required exchange rate, got %v", err)
	}
	foreign.ExchangeRate = "1025.123456"
	if err := foreign.ValidateAccountingAmounts(); err != nil {
		t.Fatalf("valid foreign purchase: %v", err)
	}
}

func TestPurchaseRejectsInconsistentVATAndTotals(t *testing.T) {
	tests := []struct {
		name     string
		purchase Purchase
		want     string
	}{
		{
			name: "tax does not match rate",
			purchase: Purchase{
				IssueDate: "2026-07-31", Total: Money{Amount: "120.00", Currency: "ARS"}, NetAmount: "100.00", ExemptAmount: "0",
				VATBreakdown: []VATBreakdownItem{{Rate: "21", BaseAmount: "100.00", TaxAmount: "20.00"}},
			},
			want: "does not match rate",
		},
		{
			name: "bases differ from net",
			purchase: Purchase{
				IssueDate: "2026-07-31", Total: Money{Amount: "121.00", Currency: "ARS"}, NetAmount: "99.00", ExemptAmount: "1.00",
				VATBreakdown: []VATBreakdownItem{{Rate: "21", BaseAmount: "100.00", TaxAmount: "21.00"}},
			},
			want: "net_amount",
		},
		{
			name: "duplicate rate",
			purchase: Purchase{
				IssueDate: "2026-07-31", Total: Money{Amount: "121.00", Currency: "ARS"}, NetAmount: "100.00", ExemptAmount: "0",
				VATBreakdown: []VATBreakdownItem{
					{Rate: "21", BaseAmount: "50.00", TaxAmount: "10.50"},
					{Rate: "21", BaseAmount: "50.00", TaxAmount: "10.50"},
				},
			},
			want: "duplicate",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.purchase.ValidateAccountingAmounts()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}
