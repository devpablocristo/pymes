package ar

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

func TestCUITCheckDigit(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"30-00000000-7", "30710158211"} {
		if _, err := ParseCUIT(raw); err != nil {
			t.Fatalf("ParseCUIT(%q): %v", raw, err)
		}
	}
	if _, err := ParseCUIT("30-00000000-8"); err == nil {
		t.Fatal("expected invalid check digit")
	}
}

func TestVoucherTypeForABCAndNotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		operation fiscal.Operation
		issuer    VATCondition
		receiver  VATCondition
		want      VoucherType
	}{
		{fiscal.OperationInvoice, VATRegistered, VATRegistered, InvoiceA},
		{fiscal.OperationInvoice, VATRegistered, VATConsumerFinal, InvoiceB},
		{fiscal.OperationInvoice, VATMonotax, VATRegistered, InvoiceC},
		{fiscal.OperationCreditNote, VATRegistered, VATRegistered, CreditNoteA},
		{fiscal.OperationDebitNote, VATRegistered, VATConsumerFinal, DebitNoteB},
		{fiscal.OperationDebitNote, VATMonotax, VATConsumerFinal, DebitNoteC},
	}
	for _, test := range tests {
		got, err := VoucherTypeFor(test.operation, test.issuer, test.receiver)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("VoucherTypeFor(%s,%d,%d) = %d, want %d", test.operation, test.issuer, test.receiver, got, test.want)
		}
	}
}

func TestCalculateTotalsExactByVATRate(t *testing.T) {
	t.Parallel()

	totals, err := CalculateTotals(InvoiceA, []TaxableAmount{
		{Category: Taxable, Amount: fiscal.MustDecimal("100.01"), Rate: fiscal.MustDecimal("21")},
		{Category: Taxable, Amount: fiscal.MustDecimal("50"), Rate: fiscal.MustDecimal("10.5")},
		{Category: Exempt, Amount: fiscal.MustDecimal("5")},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := totals.VAT.String(), "26.25"; got != want {
		t.Fatalf("VAT = %s, want %s", got, want)
	}
	if got, want := totals.Total.String(), "181.26"; got != want {
		t.Fatalf("total = %s, want %s", got, want)
	}
	if len(totals.VATLines) != 2 {
		t.Fatalf("VAT lines = %d, want 2", len(totals.VATLines))
	}
}

func TestTypeCDoesNotDiscriminateVAT(t *testing.T) {
	t.Parallel()

	totals, err := CalculateTotals(InvoiceC, []TaxableAmount{{
		Category: Taxable, Amount: fiscal.MustDecimal("121"), Rate: fiscal.MustDecimal("21"),
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !totals.VAT.IsZero() || len(totals.VATLines) != 0 || totals.Total.String() != "121" {
		t.Fatalf("unexpected type C totals: %#v", totals)
	}
}

func TestBuildQRUsesNumericExactJSONAndARCADomain(t *testing.T) {
	t.Parallel()

	cuit, err := ParseCUIT("30-00000000-7")
	if err != nil {
		t.Fatal(err)
	}
	document, err := NewReceiverDocument(DocumentCUIT, "30710158211")
	if err != nil {
		t.Fatal(err)
	}
	qrURL, err := BuildQRURL(QRInput{
		IssueDate: "2026-07-24", IssuerCUIT: cuit, PointOfSale: 3,
		VoucherType: InvoiceA, VoucherNumber: 42, Total: fiscal.MustDecimal("121.01"),
		Currency: "ARS", ExchangeRate: fiscal.MustDecimal("1"),
		ReceiverDocument: &document, AuthorizationCode: "74212345678901",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(qrURL, QRBaseURL) {
		t.Fatalf("QR URL = %q", qrURL)
	}
	encoded := strings.TrimPrefix(qrURL, QRBaseURL)
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatal(err)
	}
	if got, want := string(fields["importe"]), "121.01"; got != want {
		t.Fatalf("importe JSON = %s, want numeric %s", got, want)
	}
	if got, want := string(fields["cuit"]), "30000000007"; got != want {
		t.Fatalf("cuit JSON = %s, want numeric %s", got, want)
	}
}
