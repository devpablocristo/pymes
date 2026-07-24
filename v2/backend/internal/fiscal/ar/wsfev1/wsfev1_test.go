package wsfev1

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsaa"
)

func authFixture(t *testing.T) Auth {
	t.Helper()
	cuit, err := ar.ParseCUIT("30-00000000-7")
	if err != nil {
		t.Fatal(err)
	}
	return Auth{
		Ticket: wsaa.AccessTicket{Token: "token<&", Sign: "sign>&"},
		CUIT:   cuit,
	}
}

func requestFixture(t *testing.T) Request {
	t.Helper()
	receiver, err := ar.NewReceiverDocument(ar.DocumentCUIT, "30710158211")
	if err != nil {
		t.Fatal(err)
	}
	totals, err := ar.CalculateTotals(ar.InvoiceA, []ar.TaxableAmount{{
		Category: ar.Taxable, Amount: fiscal.MustDecimal("100"), Rate: fiscal.MustDecimal("21"),
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return Request{
		PointOfSale: 3, VoucherType: ar.InvoiceA, Concept: ar.ConceptServices,
		Receiver: receiver, ReceiverVATCondition: ar.VATRegistered,
		Number: 42, IssueDate: "2026-07-24", Totals: totals,
		Currency: "ARS", ExchangeRate: fiscal.MustDecimal("1"),
		ServiceFrom: "2026-07-01", ServiceTo: "2026-07-24", PaymentDue: "2026-08-01",
	}
}

func TestBuildAuthorizeEnvelopeExactAmountsServicesAndEscaping(t *testing.T) {
	t.Parallel()

	envelope, err := BuildAuthorizeEnvelope(authFixture(t), requestFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	xml := string(envelope)
	for _, expected := range []string{
		"<ar:Token>token&lt;&amp;</ar:Token>",
		"<ar:Sign>sign&gt;&amp;</ar:Sign>",
		"<ar:ImpNeto>100.00</ar:ImpNeto>",
		"<ar:ImpIVA>21.00</ar:ImpIVA>",
		"<ar:ImpTotal>121.00</ar:ImpTotal>",
		"<ar:FchServDesde>20260701</ar:FchServDesde>",
		"<ar:MonCotiz>1.000000</ar:MonCotiz>",
		"<ar:CondicionIVAReceptorId>1</ar:CondicionIVAReceptorId>",
	} {
		if !strings.Contains(xml, expected) {
			t.Fatalf("envelope does not contain %q:\n%s", expected, xml)
		}
	}
}

func TestRequestRequiresAssociationForCreditAndDebitNotes(t *testing.T) {
	t.Parallel()

	request := requestFixture(t)
	request.VoucherType = ar.CreditNoteA
	if err := request.Validate(); err == nil {
		t.Fatal("expected associated voucher validation error")
	}
	request.Associated = &AssociatedVoucher{
		Type: ar.InvoiceA, PointOfSale: 3, Number: 10, IssueDate: "2026-07-01",
	}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestParseAuthorizeFixtures(t *testing.T) {
	t.Parallel()

	approvedRaw, err := os.ReadFile("testdata/authorize_approved.xml")
	if err != nil {
		t.Fatal(err)
	}
	approved, err := ParseAuthorizeResponse(approvedRaw)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Decision != fiscal.DecisionAuthorized || approved.CAE != "74212345678901" ||
		approved.Number != 42 || len(approved.Observations) != 1 {
		t.Fatalf("unexpected approved result: %#v", approved)
	}

	rejectedRaw, err := os.ReadFile("testdata/authorize_rejected.xml")
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := ParseAuthorizeResponse(rejectedRaw)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Decision != fiscal.DecisionRejected || len(rejected.Errors) != 1 {
		t.Fatalf("unexpected rejected result: %#v", rejected)
	}
}

func TestParseConsultSupportsRecoveryAndDefinitiveNotFound(t *testing.T) {
	t.Parallel()

	approvedRaw, err := os.ReadFile("testdata/consult_approved.xml")
	if err != nil {
		t.Fatal(err)
	}
	approved, err := ParseConsultResponse(approvedRaw)
	if err != nil {
		t.Fatal(err)
	}
	if !approved.Found || approved.Code != "74212345678901" || approved.Number != 42 {
		t.Fatalf("unexpected consult result: %#v", approved)
	}
	if got, want := approved.Totals.Total.String(), "121"; got != want {
		t.Fatalf("consult total = %s, want %s", got, want)
	}

	missingRaw, err := os.ReadFile("testdata/consult_not_found.xml")
	if err != nil {
		t.Fatal(err)
	}
	missing, err := ParseConsultResponse(missingRaw)
	if err != nil {
		t.Fatal(err)
	}
	if missing.Found {
		t.Fatal("expected code 602 to be a definitive not-found")
	}
}

type errorTransport struct{}

func (errorTransport) Call(context.Context, ar.SOAPRequest) ([]byte, error) {
	return nil, errors.New("lost connection")
}

func TestAuthorizeClassifiesLostResponseAsUncertain(t *testing.T) {
	t.Parallel()

	client, err := NewClient(errorTransport{}, ar.Homologation)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Authorize(context.Background(), authFixture(t), requestFixture(t))
	if !errors.Is(err, fiscal.ErrUncertainResponse) {
		t.Fatalf("error = %v, want ErrUncertainResponse", err)
	}
}

func TestForeignCurrencyAndMixedConcept(t *testing.T) {
	t.Parallel()

	request := requestFixture(t)
	request.Concept = ar.ConceptMixed
	request.Currency = "USD"
	request.ExchangeRate = fiscal.MustDecimal("1325.125")
	request.ServiceFrom = "20260701"
	request.ServiceTo = "20260724"
	request.PaymentDue = "20260801"
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	envelope, err := BuildAuthorizeEnvelope(authFixture(t), request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(envelope), "<ar:MonId>DOL</ar:MonId>") ||
		!strings.Contains(string(envelope), "<ar:MonCotiz>1325.125000</ar:MonCotiz>") {
		t.Fatalf("foreign currency missing from request: %s", envelope)
	}
}

func TestAuthorizeResultMapsProcessedDayWithoutClockGuessing(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/authorize_approved.xml")
	if err != nil {
		t.Fatal(err)
	}
	result, err := ParseAuthorizeResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.ProcessedAt.Format("2006-01-02"), "2026-07-24"; got != want {
		t.Fatalf("processed date = %s, want %s", got, want)
	}
	if result.ProcessedAt.Location() != time.UTC {
		t.Fatalf("processed date location = %s", result.ProcessedAt.Location())
	}
}
