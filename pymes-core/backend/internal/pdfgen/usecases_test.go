package pdfgen

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	admindomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/admin/usecases/domain"
	quotedomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/quotes/usecases/domain"
	saledomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/sales/usecases/domain"
)

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
			OrgID:        orgID,
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
			OrgID:         orgID,
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
