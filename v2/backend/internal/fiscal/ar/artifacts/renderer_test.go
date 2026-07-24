package artifacts

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

func TestRendererProducesDeterministicPDFFromSnapshotAndFiscalQR(t *testing.T) {
	snapshot, err := fiscal.NewSnapshot(fiscal.FiscalSnapshot{
		Version:     fiscal.SnapshotVersion,
		CountryCode: "AR",
		IssueDate:   "2026-07-24",
		Issuer: fiscal.PartySnapshot{
			Name: "Pyme Argentina SRL", TaxID: "30000000007",
			TaxCondition: "responsable_inscripto", Address: "CABA",
			ActivityStartDay: "2020-01-02",
		},
		Receiver: fiscal.PartySnapshot{
			Name: "Cliente SA", TaxCondition: "responsable_inscripto",
			DocumentType: "80", DocumentNumber: "30710158211",
		},
		Currency: fiscal.CurrencySnapshot{
			Code: "PES", Rate: fiscal.MustDecimal("1"),
		},
		Lines: []fiscal.FiscalLineSnapshot{{
			Position: 1, Description: "Servicio mensual",
			Quantity: fiscal.MustDecimal("1"), UnitPrice: fiscal.MustDecimal("100"),
			NetAmount: fiscal.MustDecimal("100"), TaxRate: fiscal.MustDecimal("21"),
			TaxAmount: fiscal.MustDecimal("21"), TotalAmount: fiscal.MustDecimal("121"),
		}},
		Totals: fiscal.FiscalTotalsSnapshot{
			NetTaxed: fiscal.MustDecimal("100"), VAT: fiscal.MustDecimal("21"),
			Total: fiscal.MustDecimal("121"), Functional: fiscal.MustDecimal("121"),
		},
		Metadata:    map[string]string{"voucher_type": "1", "point_of_sale": "3", "concept": "services"},
		ServiceFrom: "2026-07-01", ServiceTo: "2026-07-31", PaymentDue: "2026-08-10",
	})
	if err != nil {
		t.Fatal(err)
	}
	authorization := fiscal.Authorization{
		Decision: fiscal.DecisionAuthorized, Code: "74123456789012",
		ExpiresOn: "2026-08-03", Number: 42,
		ProcessedAt: time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
	}
	renderer := NewRenderer()
	first, err := renderer.Render(context.Background(), snapshot, authorization)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	second, err := renderer.Render(context.Background(), snapshot, authorization)
	if err != nil {
		t.Fatalf("second Render() error = %v", err)
	}
	if len(first) != 2 || first[0].Kind != "qr" || first[1].Kind != "pdf" {
		t.Fatalf("artifacts = %+v", first)
	}
	if !bytes.HasPrefix(first[0].Body, []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatal("QR artifact is not PNG")
	}
	if !bytes.HasPrefix(first[1].Body, []byte("%PDF-")) {
		t.Fatal("PDF artifact is not PDF")
	}
	if !bytes.Equal(first[0].Body, second[0].Body) ||
		!bytes.Equal(first[1].Body, second[1].Body) {
		t.Fatal("snapshot renderer is not deterministic")
	}
}

func TestRendererRejectsSnapshotWithoutImmutableFiscalIdentity(t *testing.T) {
	snapshot, err := fiscal.NewSnapshot(fiscal.FiscalSnapshot{
		Version: 1, CountryCode: "AR", IssueDate: "2026-07-24",
		Issuer:   fiscal.PartySnapshot{Name: "Issuer", TaxID: "30000000007"},
		Receiver: fiscal.PartySnapshot{Name: "Receiver"},
		Currency: fiscal.CurrencySnapshot{Code: "PES", Rate: fiscal.MustDecimal("1")},
		Lines: []fiscal.FiscalLineSnapshot{{
			Position: 1, Description: "Item", Quantity: fiscal.MustDecimal("1"),
			UnitPrice: fiscal.MustDecimal("1"), NetAmount: fiscal.MustDecimal("1"),
			TotalAmount: fiscal.MustDecimal("1"),
		}},
		Totals: fiscal.FiscalTotalsSnapshot{NetTaxed: fiscal.MustDecimal("1"), Total: fiscal.MustDecimal("1")},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewRenderer().Render(context.Background(), snapshot, fiscal.Authorization{
		Decision: fiscal.DecisionAuthorized, Code: "74123456789012",
		ExpiresOn: "2026-08-03", Number: 1, ProcessedAt: time.Now(),
	})
	if err == nil {
		t.Fatal("snapshot without voucher type/POS was rendered")
	}
}
