package ledger

import (
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"
)

func TestDistributeTax(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		lines    []SaleEventLine
		taxTotal float64
		wantSum  float64
		wantLen  int
	}{
		{
			name:     "single rate",
			lines:    []SaleEventLine{{TaxRate: 21, Subtotal: 100}},
			taxTotal: 21,
			wantSum:  21,
			wantLen:  1,
		},
		{
			name:     "two rates",
			lines:    []SaleEventLine{{TaxRate: 21, Subtotal: 100}, {TaxRate: 10.5, Subtotal: 200}},
			taxTotal: 42, // 21 + 21
			wantSum:  42,
			wantLen:  2,
		},
		{
			name:     "residual cent goes to largest base",
			lines:    []SaleEventLine{{TaxRate: 21, Subtotal: 100}, {TaxRate: 10.5, Subtotal: 200}},
			taxTotal: 42.01, // un centavo de más por redondeo del documento
			wantSum:  42.01,
			wantLen:  2,
		},
		{
			name:     "no tax",
			lines:    []SaleEventLine{{TaxRate: 0, Subtotal: 100}},
			taxTotal: 0,
			wantLen:  0,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shares := distributeTax(tc.lines, tc.taxTotal)
			if len(shares) != tc.wantLen {
				t.Fatalf("len=%d want %d", len(shares), tc.wantLen)
			}
			sum := 0.0
			for _, s := range shares {
				sum += s.amount
			}
			if math.Abs(sum-tc.wantSum) > 0.001 {
				t.Fatalf("sum=%.2f want %.2f (must equal taxTotal exactly)", sum, tc.wantSum)
			}
		})
	}
}

func links(roles ...string) map[string]uuid.UUID {
	m := make(map[string]uuid.UUID, len(roles))
	for _, r := range roles {
		m[r] = uuid.New()
	}
	return m
}

func TestBuildSaleEntry(t *testing.T) {
	t.Parallel()
	org := uuid.New()
	sale := uuid.New()

	t.Run("contado cash single rate balances", func(t *testing.T) {
		t.Parallel()
		evt := SaleEvent{
			OrgID: org, SaleID: sale, Number: "VTA-1", Currency: "ARS",
			PaymentStatus: "paid", PaymentMethod: "cash",
			Subtotal: 100, TaxTotal: 21, Total: 121,
			Lines: []SaleEventLine{{TaxRate: 21, Subtotal: 100}},
		}
		entry, err := buildSaleEntry(evt, links("cash", "revenue", "vat_payable_21"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entry.Lines) != 3 {
			t.Fatalf("want 3 lines, got %d", len(entry.Lines))
		}
		var d, c float64
		for _, l := range entry.Lines {
			d += l.Debit
			c += l.Credit
		}
		if math.Abs(d-121) > 0.001 || math.Abs(c-121) > 0.001 {
			t.Fatalf("not balanced: debit=%.2f credit=%.2f", d, c)
		}
	})

	t.Run("multi-alicuota balances and anchors to total", func(t *testing.T) {
		t.Parallel()
		evt := SaleEvent{
			OrgID: org, SaleID: sale, Number: "VTA-2", Currency: "ARS",
			PaymentStatus: "paid", PaymentMethod: "transfer",
			Subtotal: 200, TaxTotal: 31.5, Total: 231.5,
			Lines: []SaleEventLine{{TaxRate: 21, Subtotal: 100}, {TaxRate: 10.5, Subtotal: 100}},
		}
		entry, err := buildSaleEntry(evt, links("bank", "revenue", "vat_payable_21", "vat_payable_105"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var d, c float64
		for _, l := range entry.Lines {
			d += l.Debit
			c += l.Credit
		}
		if math.Abs(d-231.5) > 0.001 || math.Abs(c-231.5) > 0.001 {
			t.Fatalf("not balanced: debit=%.2f credit=%.2f", d, c)
		}
	})

	t.Run("missing account link fails", func(t *testing.T) {
		t.Parallel()
		evt := SaleEvent{
			OrgID: org, SaleID: sale, Number: "VTA-3", PaymentStatus: "paid", PaymentMethod: "cash",
			Subtotal: 100, TaxTotal: 21, Total: 121,
			Lines: []SaleEventLine{{TaxRate: 21, Subtotal: 100}},
		}
		// falta vat_payable_21
		_, err := buildSaleEntry(evt, links("cash", "revenue"))
		if !errors.Is(err, ErrAccountLinkMissing) {
			t.Fatalf("expected ErrAccountLinkMissing, got %v", err)
		}
	})

	t.Run("payment cobro balances DR cash CR receivable", func(t *testing.T) {
		t.Parallel()
		party := uuid.New()
		evt := PaymentEvent{
			OrgID: org, PaymentID: uuid.New(), SaleID: sale, Method: "cash",
			Amount: 121, Currency: "ARS",
		}
		entry, err := buildPaymentEntry(evt, links("cash", "receivable"), &party)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entry.Lines) != 2 || entry.SourceType != "payment" {
			t.Fatalf("bad payment entry: %+v", entry)
		}
		var d, c float64
		for _, l := range entry.Lines {
			d += l.Debit
			c += l.Credit
		}
		if math.Abs(d-121) > 0.001 || math.Abs(c-121) > 0.001 {
			t.Fatalf("not balanced: debit=%.2f credit=%.2f", d, c)
		}
	})

	t.Run("purchase mixed product+service, two rates, balances", func(t *testing.T) {
		t.Parallel()
		supplier := uuid.New()
		d := purchaseData{
			OrgID: org, PurchaseID: uuid.New(), Currency: "ARS", PartyID: &supplier,
			Subtotal: 200, TaxTotal: 31.5, Total: 231.5,
			Items: []purchaseLine{
				{IsProduct: true, TaxRate: 21, Subtotal: 100},
				{IsProduct: false, TaxRate: 10.5, Subtotal: 100},
			},
		}
		entry, err := buildPurchaseEntry(d, links("inventory", "purchase_expense", "vat_credit_21", "vat_credit_105", "payable"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.SourceType != "purchase" || entry.SourceEvent != "purchase.received" {
			t.Fatalf("bad source: %+v", entry)
		}
		var d2, c float64
		for _, l := range entry.Lines {
			d2 += l.Debit
			c += l.Credit
		}
		if math.Abs(d2-231.5) > 0.001 || math.Abs(c-231.5) > 0.001 {
			t.Fatalf("not balanced: debit=%.2f credit=%.2f", d2, c)
		}
	})

	t.Run("return cash refund balances", func(t *testing.T) {
		t.Parallel()
		evt := ReturnEvent{
			OrgID: org, ReturnID: uuid.New(), SaleID: uuid.New(), Currency: "ARS", RefundMethod: "cash",
			Subtotal: 100, TaxTotal: 21, Total: 121,
			Lines: []SaleEventLine{{TaxRate: 21, Subtotal: 100}},
		}
		entry, err := buildReturnEntry(evt, links("revenue", "vat_payable_21", "cash"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.SourceType != "return" || len(entry.Lines) != 3 {
			t.Fatalf("bad entry: %+v", entry)
		}
		var d, c float64
		for _, l := range entry.Lines {
			d += l.Debit
			c += l.Credit
		}
		if math.Abs(d-121) > 0.001 || math.Abs(c-121) > 0.001 {
			t.Fatalf("not balanced: debit=%.2f credit=%.2f", d, c)
		}
	})

	t.Run("return credit_note refund uses credit_note_payable with party", func(t *testing.T) {
		t.Parallel()
		party := uuid.New()
		evt := ReturnEvent{
			OrgID: org, ReturnID: uuid.New(), SaleID: uuid.New(), Currency: "ARS", RefundMethod: "credit_note",
			PartyID: &party, Subtotal: 100, TaxTotal: 21, Total: 121,
			Lines: []SaleEventLine{{TaxRate: 21, Subtotal: 100}},
		}
		entry, err := buildReturnEntry(evt, links("revenue", "vat_payable_21", "credit_note_payable"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// la línea de crédito (contrapartida) debe llevar party_id
		last := entry.Lines[len(entry.Lines)-1]
		if last.PartyID == nil || *last.PartyID != party {
			t.Fatalf("expected credit_note line to carry party_id")
		}
	})

	t.Run("supplier payment balances DR payable CR cash", func(t *testing.T) {
		t.Parallel()
		supplier := uuid.New()
		evt := PurchasePaymentEvent{OrgID: org, PaymentID: uuid.New(), PurchaseID: uuid.New(), Method: "transfer", Amount: 121, Currency: "ARS"}
		entry, err := buildPurchasePaymentEntry(evt, links("payable", "bank"), &supplier)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.SourceType != "payment" || entry.SourceEvent != "supplier_payment.created" || len(entry.Lines) != 2 {
			t.Fatalf("bad entry: %+v", entry)
		}
		var d, c float64
		for _, l := range entry.Lines {
			d += l.Debit
			c += l.Credit
		}
		if math.Abs(d-121) > 0.001 || math.Abs(c-121) > 0.001 {
			t.Fatalf("not balanced: debit=%.2f credit=%.2f", d, c)
		}
	})

	t.Run("purchase missing payable link fails", func(t *testing.T) {
		t.Parallel()
		d := purchaseData{
			OrgID: org, PurchaseID: uuid.New(), Subtotal: 100, TaxTotal: 21, Total: 121,
			Items: []purchaseLine{{IsProduct: true, TaxRate: 21, Subtotal: 100}},
		}
		_, err := buildPurchaseEntry(d, links("inventory", "vat_credit_21"))
		if !errors.Is(err, ErrAccountLinkMissing) {
			t.Fatalf("expected ErrAccountLinkMissing, got %v", err)
		}
	})

	t.Run("payment missing receivable link fails", func(t *testing.T) {
		t.Parallel()
		evt := PaymentEvent{OrgID: org, PaymentID: uuid.New(), SaleID: sale, Method: "cash", Amount: 50}
		_, err := buildPaymentEntry(evt, links("cash"), nil)
		if !errors.Is(err, ErrAccountLinkMissing) {
			t.Fatalf("expected ErrAccountLinkMissing, got %v", err)
		}
	})

	t.Run("credito uses receivable with party", func(t *testing.T) {
		t.Parallel()
		party := uuid.New()
		evt := SaleEvent{
			OrgID: org, SaleID: sale, Number: "VTA-4", PaymentStatus: "pending", PaymentMethod: "cash",
			PartyID: &party, Subtotal: 100, TaxTotal: 21, Total: 121,
			Lines: []SaleEventLine{{TaxRate: 21, Subtotal: 100}},
		}
		entry, err := buildSaleEntry(evt, links("receivable", "revenue", "vat_payable_21"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// la primera línea (débito) debe llevar party_id
		if entry.Lines[0].PartyID == nil || *entry.Lines[0].PartyID != party {
			t.Fatalf("expected receivable line to carry party_id")
		}
	})
}
