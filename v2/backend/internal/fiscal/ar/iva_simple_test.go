package ar

import (
	"bytes"
	"testing"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

func ivaRecordFixture(t *testing.T, direction IVARecordDirection) IVARecord {
	t.Helper()
	document, err := NewReceiverDocument(DocumentCUIT, "30710158211")
	if err != nil {
		t.Fatal(err)
	}
	return IVARecord{
		Direction: direction, Authorized: true, IssueDate: "2026-07-24",
		VoucherType: InvoiceA, PointOfSale: 3, Number: 42,
		CounterpartyDocument: document, CounterpartyName: "Compañía Ñandú SA",
		Currency: CurrencyPES, ExchangeRate: fiscal.MustDecimal("1"),
		Total: fiscal.MustDecimal("121"), VAT: fiscal.MustDecimal("21"),
		ComputableVATCredit: fiscal.MustDecimal("21"),
		VATLines: []VATBreakdown{{
			ID: VATIDTwentyOne, Rate: fiscal.MustDecimal("21"),
			BaseAmount: fiscal.MustDecimal("100"), Amount: fiscal.MustDecimal("21"),
		}},
	}
}

func TestExportIVASimpleFixedWidthAndOrdering(t *testing.T) {
	t.Parallel()

	files, err := ExportIVASimple("202607", []IVARecord{
		ivaRecordFixture(t, IVAPurchase),
		ivaRecordFixture(t, IVASale),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(bytes.TrimSuffix(files.SalesVouchers, []byte("\r\n"))), 266; got != want {
		t.Fatalf("sales header length = %d, want %d", got, want)
	}
	if got, want := len(bytes.TrimSuffix(files.SalesVAT, []byte("\r\n"))), 62; got != want {
		t.Fatalf("sales VAT length = %d, want %d", got, want)
	}
	if got, want := len(bytes.TrimSuffix(files.PurchaseVouchers, []byte("\r\n"))), 325; got != want {
		t.Fatalf("purchase header length = %d, want %d", got, want)
	}
	if got, want := len(bytes.TrimSuffix(files.PurchaseVAT, []byte("\r\n"))), 84; got != want {
		t.Fatalf("purchase VAT length = %d, want %d", got, want)
	}
	if !bytes.Contains(files.SalesVouchers, []byte{0xf1}) {
		t.Fatal("expected ANSI/Latin-1 ñ in export")
	}
}

func TestExportIVASimpleRejectsDuplicatePurchase(t *testing.T) {
	t.Parallel()

	record := ivaRecordFixture(t, IVAPurchase)
	if _, err := ExportIVASimple("202607", []IVARecord{record, record}); err == nil {
		t.Fatal("expected duplicate supplier/type/point/number to be rejected")
	}
}

func TestCalculateIVAPositionAccountsForCreditNotes(t *testing.T) {
	t.Parallel()

	sale := ivaRecordFixture(t, IVASale)
	credit := sale
	credit.VoucherType = CreditNoteA
	credit.Number = 43
	credit.VAT = fiscal.MustDecimal("5")
	position, err := CalculateIVAPosition(
		[]IVARecord{sale, credit},
		fiscal.MustDecimal("1"), fiscal.MustDecimal("2"), fiscal.MustDecimal("3"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := position.Payable.String(), "10"; got != want {
		t.Fatalf("IVA payable = %s, want %s", got, want)
	}
}
