package httpserver

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func TestBuildFiscalLinesKeepsExactVATAndTotal(t *testing.T) {
	t.Parallel()

	lines, taxes, totals, err := buildFiscalLines(
		[]api.FiscalVoucherLineInput{{
			Description: "Servicio",
			Quantity:    "1",
			UnitPrice:   "100",
			Subtotal:    "100",
			Taxes: []api.FiscalTaxComponent{{
				Kind: api.FiscalTaxComponentKindVat, TaxableBase: "100",
				Rate: "21", Amount: "21",
			}},
		}},
		ar.InvoiceA,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || len(taxes) != 0 {
		t.Fatalf("lines/taxes = %d/%d", len(lines), len(taxes))
	}
	if got, want := lines[0].TaxAmount.String(), "21"; got != want {
		t.Fatalf("line VAT = %s, want %s", got, want)
	}
	if got, want := totals.Total.String(), "121"; got != want {
		t.Fatalf("total = %s, want %s", got, want)
	}
}

func TestBuildFiscalLinesRejectsInexactOrTypeCVAT(t *testing.T) {
	t.Parallel()

	base := api.FiscalVoucherLineInput{
		Description: "Servicio", Quantity: "1", UnitPrice: "100", Subtotal: "100",
		Taxes: []api.FiscalTaxComponent{{
			Kind: api.FiscalTaxComponentKindVat, TaxableBase: "100",
			Rate: "21", Amount: "20",
		}},
	}
	if _, _, _, err := buildFiscalLines([]api.FiscalVoucherLineInput{base}, ar.InvoiceA); err == nil {
		t.Fatal("expected non-reconciling VAT to fail")
	}
	base.Taxes[0].Amount = "21"
	if _, _, _, err := buildFiscalLines([]api.FiscalVoucherLineInput{base}, ar.InvoiceC); err == nil {
		t.Fatal("expected type C discriminated VAT to fail")
	}
	base.Taxes = nil
	lines, _, totals, err := buildFiscalLines(
		[]api.FiscalVoucherLineInput{base},
		ar.InvoiceC,
	)
	if err != nil {
		t.Fatalf("type C final price failed: %v", err)
	}
	if !lines[0].TaxAmount.IsZero() || totals.Total.String() != "100" {
		t.Fatalf("type C line/totals = %#v / %#v", lines[0], totals)
	}
}

func TestBuildFiscalSnapshotValidatesServiceDateOrder(t *testing.T) {
	t.Parallel()

	handler := &IAMAPI{now: func() time.Time {
		return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	}}
	date := func(value string) *openapi_types.Date {
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			t.Fatal(err)
		}
		return &openapi_types.Date{Time: parsed}
	}
	input := api.FiscalVoucherInput{
		Environment:            api.FiscalEnvironmentHomologation,
		SourceType:             api.FiscalVoucherInputSourceType("sale"),
		SourceId:               uuid.New(),
		PointOfSaleId:          uuid.New(),
		Concept:                api.Services,
		ReceiverDocumentType:   api.FiscalVoucherInputReceiverDocumentTypeCUIT,
		ReceiverDocumentNumber: "30710158211",
		ReceiverTaxCondition:   "responsable_inscripto",
		Currency:               "ARS",
		ExchangeRate:           "1",
		ServiceFrom:            date("2026-07-25"),
		ServiceTo:              date("2026-07-24"),
		PaymentDueDate:         date("2026-07-26"),
		Lines: []api.FiscalVoucherLineInput{{
			Description: "Servicio", Quantity: "1", UnitPrice: "100", Subtotal: "100",
			Taxes: []api.FiscalTaxComponent{{
				Kind: api.FiscalTaxComponentKindVat, TaxableBase: "100",
				Rate: "21", Amount: "21",
			}},
		}},
	}
	receiverName := "Cliente"
	input.ReceiverName = &receiverName
	issuer := fiscalIssuer{
		environment: "homologation", pointOfSale: 3,
		legalName: "Emisor", taxAddress: "Córdoba 123",
		activityStartDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		cuit:              "30710158211", taxCondition: "responsable_inscripto",
	}
	if _, err := handler.buildFiscalSnapshot(input, issuer, ar.InvoiceA, nil); err == nil ||
		!strings.Contains(err.Error(), "start date") {
		t.Fatalf("service date order error = %v", err)
	}

	input.ServiceFrom = date("2026-07-24")
	input.ServiceTo = date("2026-07-25")
	input.PaymentDueDate = date("2026-07-23")
	if _, err := handler.buildFiscalSnapshot(input, issuer, ar.InvoiceA, nil); err == nil ||
		!strings.Contains(err.Error(), "due date") {
		t.Fatalf("payment due date error = %v", err)
	}

	input.PaymentDueDate = date("2026-07-26")
	document, err := handler.buildFiscalSnapshot(input, issuer, ar.InvoiceA, nil)
	if err != nil {
		t.Fatal(err)
	}
	if document.ServiceFrom != "2026-07-24" ||
		document.ServiceTo != "2026-07-25" ||
		document.PaymentDue != "2026-07-26" {
		t.Fatalf("service dates = %#v", document)
	}
}

func TestFiscalVoucherDetailExposesEnvironmentAndSnapshotLines(t *testing.T) {
	t.Parallel()

	snapshot, err := fiscal.NewSnapshot(fiscal.FiscalSnapshot{
		Version:     fiscal.SnapshotVersion,
		CountryCode: "AR",
		IssueDate:   "2026-07-24",
		Issuer: fiscal.PartySnapshot{
			Name: "Emisor",
		},
		Receiver: fiscal.PartySnapshot{
			Name: "Cliente", DocumentType: "80",
			DocumentNumber: "30712345670", TaxCondition: "responsable_inscripto",
		},
		Currency: fiscal.CurrencySnapshot{
			Code: "PES", Rate: fiscal.NewDecimalFromInt(1), RateSource: "ARCA",
		},
		Lines: []fiscal.FiscalLineSnapshot{{
			Position: 1, Description: "Producto",
			Quantity: fiscal.NewDecimalFromInt(1), UnitPrice: fiscal.NewDecimalFromInt(100),
			NetAmount: fiscal.NewDecimalFromInt(100), TaxRate: fiscal.NewDecimalFromInt(21),
			TaxAmount: fiscal.NewDecimalFromInt(21), TotalAmount: fiscal.NewDecimalFromInt(121),
		}},
		Totals: fiscal.FiscalTotalsSnapshot{
			NetTaxed:   fiscal.NewDecimalFromInt(100),
			VAT:        fiscal.NewDecimalFromInt(21),
			Total:      fiscal.NewDecimalFromInt(121),
			Functional: fiscal.NewDecimalFromInt(121),
		},
		Metadata: map[string]string{"concept": "products"},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	detail, err := fiscalVoucherDetailFromDomain(fiscal.Voucher{
		ID: uuid.New(), Source: fiscal.SourceReference{Kind: "sale", ID: uuid.New()},
		Operation: fiscal.OperationInvoice, Environment: "production",
		PointOfSale: 3, AuthorityType: int(ar.InvoiceA),
		Status: fiscal.StatusQueued, Snapshot: snapshot,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Environment != api.FiscalEnvironmentProduction ||
		detail.ReceiverDocumentType != api.FiscalVoucherDetailReceiverDocumentTypeCUIT ||
		len(detail.Lines) != 1 ||
		detail.Lines[0].VatAmount != "21" {
		t.Fatalf("detail = %#v", detail)
	}
}

func TestFiscalRegistryBundleContainsBothOfficialFiles(t *testing.T) {
	t.Parallel()

	raw, err := fiscalRegistryBundle(
		"comprobantes.txt", []byte("header\n"),
		"alicuotas.txt", []byte("vat\n"),
	)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		names = append(names, file.Name)
		entry, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.ReadAll(entry); err != nil {
			t.Fatal(err)
		}
		_ = entry.Close()
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "alicuotas.txt,comprobantes.txt" {
		t.Fatalf("bundle files = %v", names)
	}
}

func TestFiscalEndpointsRejectBeforePersistenceWhenInputIsInvalid(t *testing.T) {
	t.Parallel()

	handler := NewIAMAPI(config.ClerkConfig{})
	t.Run("IVA period", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.getIVASimple(
			response,
			httptest.NewRequest(http.MethodGet, "/api/v1/fiscal/iva-simple/2026-13", nil),
			"2026-13",
			api.GetIVASimpleParams{},
		)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", response.Code)
		}
		var payload api.ErrorResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Error.Code != "REQUEST_INVALID" {
			t.Fatalf("error code = %s", payload.Error.Code)
		}
	})
	t.Run("PDF object store", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.getFiscalVoucherPDF(
			response,
			httptest.NewRequest(http.MethodGet, "/api/v1/fiscal/vouchers/id/pdf", nil),
			uuid.New(),
		)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", response.Code)
		}
		if !strings.Contains(response.Body.String(), "FISCAL_OBJECT_STORE_UNAVAILABLE") {
			t.Fatalf("body = %s", response.Body.String())
		}
	})
}

func TestMapFiscalErrorUsesStableHTTPCategories(t *testing.T) {
	t.Parallel()

	if !errors.Is(mapFiscalError(fiscal.ErrNotFound), errBusinessNotFound) {
		t.Fatal("fiscal not found did not map to business not found")
	}
	if !errors.Is(mapFiscalError(fiscal.ErrIdempotencyConflict), errBusinessIdempotency) {
		t.Fatal("fiscal idempotency conflict did not map to stable conflict")
	}
}
