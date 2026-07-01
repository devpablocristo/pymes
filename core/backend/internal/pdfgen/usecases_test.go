package pdfgen

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	admindomain "github.com/devpablocristo/pymes/core/backend/internal/admin/usecases/domain"
	fiscaldomain "github.com/devpablocristo/pymes/core/backend/internal/fiscal/usecases/domain"
	quotedomain "github.com/devpablocristo/pymes/core/backend/internal/quotes/usecases/domain"
	saledomain "github.com/devpablocristo/pymes/core/backend/internal/sales/usecases/domain"
)

type fakeFiscal struct {
	voucher fiscaldomain.FiscalVoucher
	err     error
}

func (f *fakeFiscal) GetVoucher(_ context.Context, _, _ uuid.UUID) (fiscaldomain.FiscalVoucher, error) {
	return f.voucher, f.err
}

type fakeQuotes struct {
	quote quotedomain.Quote
	err   error
}

func (f *fakeQuotes) GetByID(_ context.Context, _, _ uuid.UUID) (quotedomain.Quote, error) {
	return f.quote, f.err
}

type fakeSales struct {
	sale saledomain.Sale
	err  error
}

func (f *fakeSales) GetByID(_ context.Context, _, _ uuid.UUID) (saledomain.Sale, error) {
	return f.sale, f.err
}

type fakeAdmin struct {
	settings admindomain.TenantSettings
	err      error
}

func (f *fakeAdmin) GetTenantSettings(_ context.Context, _ string) (admindomain.TenantSettings, error) {
	return f.settings, f.err
}

func TestRenderQuotePDFHappyPath(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	quoteID := uuid.New()
	validUntil := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)

	uc := NewUsecases(
		&fakeQuotes{quote: quotedomain.Quote{
			ID:           quoteID,
			OrgID:     orgID,
			Number:       "P-001",
			CustomerName: "Test Customer",
			Status:       "draft",
			Items: []quotedomain.QuoteItem{
				{Description: "Widget A", Quantity: 2, UnitPrice: 100, Subtotal: 200},
				{Description: "Widget B", Quantity: 1, UnitPrice: 50, Subtotal: 50},
			},
			Subtotal:   250,
			TaxTotal:   52.50,
			Total:      302.50,
			Notes:      "Sample notes",
			ValidUntil: &validUntil,
			CreatedAt:  time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		}},
		&fakeSales{},
		&fakeAdmin{settings: admindomain.TenantSettings{
			BusinessName:  "Demo Shop",
			BusinessPhone: "+54-11-1234-5678",
			Currency:      "ARS",
		}},
	)

	pdf, filename, err := uc.RenderQuotePDF(context.Background(), orgID, quoteID)
	if err != nil {
		t.Fatalf("RenderQuotePDF() error = %v", err)
	}
	if filename != "P-001.pdf" {
		t.Fatalf("expected filename P-001.pdf, got %s", filename)
	}
	if len(pdf) < 100 {
		t.Fatalf("expected valid PDF bytes, got %d bytes", len(pdf))
	}
	if string(pdf[:5]) != "%PDF-" {
		t.Fatal("output does not start with PDF header")
	}
}

func TestRenderSaleReceiptHappyPath(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	saleID := uuid.New()

	uc := NewUsecases(
		&fakeQuotes{},
		&fakeSales{sale: saledomain.Sale{
			ID:            saleID,
			OrgID:      orgID,
			Number:        "V-042",
			CustomerName:  "Buyer",
			PaymentMethod: "transfer",
			Items: []saledomain.SaleItem{
				{Description: "Service X", Quantity: 1, UnitPrice: 500, Subtotal: 500},
			},
			Subtotal:  500,
			TaxTotal:  105,
			Total:     605,
			CreatedAt: time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC),
		}},
		&fakeAdmin{settings: admindomain.TenantSettings{
			BusinessName: "Receipt Corp",
			Currency:     "BRL",
		}},
	)

	pdf, filename, err := uc.RenderSaleReceipt(context.Background(), orgID, saleID)
	if err != nil {
		t.Fatalf("RenderSaleReceipt() error = %v", err)
	}
	if filename != "V-042.pdf" {
		t.Fatalf("expected filename V-042.pdf, got %s", filename)
	}
	if len(pdf) < 100 {
		t.Fatalf("expected valid PDF bytes, got %d bytes", len(pdf))
	}
}

func TestRenderFiscalVoucherHappyPath(t *testing.T) {
	t.Parallel()
	orgID := uuid.New()
	voucherID := uuid.New()
	emitted := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	uc := NewUsecases(
		&fakeQuotes{}, &fakeSales{}, // sin venta asociada: se renderiza solo con datos del voucher
		&fakeAdmin{settings: admindomain.TenantSettings{
			BusinessName: "Estudio Fiscal SA", BusinessTaxID: "20111111112", Currency: "ARS",
		}},
		WithFiscal(&fakeFiscal{voucher: fiscaldomain.FiscalVoucher{
			ID: voucherID, OrgID: orgID, VoucherType: 6, PointOfSale: 3, CbteNro: 11,
			DocTipo: 99, DocNro: "0", CondicionIVAReceptor: 5, Currency: "PES", ExchangeRate: 1,
			ImpNeto: 100, ImpIVA: 21, ImpTotal: 121, Status: "authorized",
			CAE: "75000000000001", CAEVto: "20260801", EmittedAt: &emitted,
			QRURL: "https://www.afip.gob.ar/fe/qr/?p=eyJ2ZXIiOjF9",
		}}),
	)

	pdf, filename, err := uc.RenderFiscalVoucher(context.Background(), orgID, voucherID)
	if err != nil {
		t.Fatalf("RenderFiscalVoucher() error = %v", err)
	}
	if filename != "comprobante-B-0003-00000011.pdf" {
		t.Fatalf("unexpected filename: %s", filename)
	}
	if len(pdf) < 100 || string(pdf[:5]) != "%PDF-" {
		t.Fatalf("expected valid PDF, got %d bytes", len(pdf))
	}
}

func TestRenderFiscalVoucherNoFiscalPort(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(&fakeQuotes{}, &fakeSales{}, &fakeAdmin{})
	if _, _, err := uc.RenderFiscalVoucher(context.Background(), uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected error when fiscal port not configured")
	}
}

func TestFiscalPresentationHelpers(t *testing.T) {
	t.Parallel()
	if voucherLetter(1) != "A" || voucherLetter(6) != "B" || voucherLetter(11) != "C" || voucherLetter(19) != "E" {
		t.Fatalf("voucherLetter wrong")
	}
	if voucherTitle(8) != "NOTA DE CREDITO" || voucherTitle(6) != "FACTURA" || voucherTitle(7) != "NOTA DE DEBITO" {
		t.Fatalf("voucherTitle wrong")
	}
	if fiscalNumber(fiscaldomain.FiscalVoucher{PointOfSale: 3, CbteNro: 11}) != "0003-00000011" {
		t.Fatalf("fiscalNumber wrong")
	}
	if condIVALabel(1) != "Responsable Inscripto" || condIVALabel(5) != "Consumidor Final" || condIVALabel(6) != "Monotributo" {
		t.Fatalf("condIVALabel wrong")
	}
	if fiscalCAEVto(fiscaldomain.FiscalVoucher{CAEVto: "20260801"}) != "01/08/2026" {
		t.Fatalf("fiscalCAEVto wrong")
	}
}

func TestRenderQuotePDFNilPortsReturnsError(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(nil, nil, nil)
	_, _, err := uc.RenderQuotePDF(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error for nil ports")
	}
}

func TestRenderSaleReceiptNilPortsReturnsError(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(nil, nil, nil)
	_, _, err := uc.RenderSaleReceipt(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error for nil ports")
	}
}

func TestCurrencySymbol(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"ARS", "$"},
		{"USD", "$"},
		{"BRL", "R$"},
		{"PEN", "S/"},
		{"", "$"},
	}
	for _, tc := range cases {
		if got := currencySymbol(tc.in); got != tc.want {
			t.Errorf("currencySymbol(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatQty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   float64
		want string
	}{
		{1, "1"},
		{2.5, "2.5"},
		{3.10, "3.1"},
		{4.00, "4"},
	}
	for _, tc := range cases {
		if got := formatQty(tc.in); got != tc.want {
			t.Errorf("formatQty(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
