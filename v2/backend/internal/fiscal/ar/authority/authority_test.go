package authority

import (
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsaa"
	"github.com/google/uuid"
)

func TestRequestForBuildsExactWSFERequestFromImmutableSnapshot(t *testing.T) {
	snapshot, err := fiscal.NewSnapshot(fiscal.FiscalSnapshot{
		Version: 1, CountryCode: "AR", IssueDate: "2026-07-24",
		Issuer: fiscal.PartySnapshot{
			Name: "Emisor", TaxID: "30000000007", TaxCondition: "responsable_inscripto",
		},
		Receiver: fiscal.PartySnapshot{
			Name: "Cliente", DocumentType: "80", DocumentNumber: "30710158211",
			TaxCondition: "responsable_inscripto",
		},
		Currency: fiscal.CurrencySnapshot{Code: ar.CurrencyPES, Rate: fiscal.MustDecimal("1")},
		Lines: []fiscal.FiscalLineSnapshot{{
			Position: 1, Description: "Venta", Quantity: fiscal.MustDecimal("1"),
			UnitPrice: fiscal.MustDecimal("100"), NetAmount: fiscal.MustDecimal("100"),
			TaxRate: fiscal.MustDecimal("21"), TaxAmount: fiscal.MustDecimal("21"),
			TotalAmount: fiscal.MustDecimal("121"),
		}},
		Totals: fiscal.FiscalTotalsSnapshot{
			NetTaxed: fiscal.MustDecimal("100"), VAT: fiscal.MustDecimal("21"),
			Total: fiscal.MustDecimal("121"), Functional: fiscal.MustDecimal("121"),
		},
		Metadata: map[string]string{"concept": "products"},
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := requestFor(fiscal.Voucher{
		ID: uuid.New(), Number: 7, PointOfSale: 2, AuthorityType: int(ar.InvoiceA),
		Snapshot: snapshot,
	})
	if err != nil {
		t.Fatalf("requestFor() error = %v", err)
	}
	if request.Number != 7 || request.PointOfSale != 2 ||
		request.Totals.Total.String() != "121" ||
		len(request.Totals.VATLines) != 1 ||
		request.Totals.VATLines[0].ID != ar.VATIDTwentyOne {
		t.Fatalf("request = %+v", request)
	}
}

func TestMemoryTicketsAreTenantAndCertificateScoped(t *testing.T) {
	repository := NewMemoryTickets()
	key := wsaaKey(uuid.New(), "fingerprint-a")
	ticket := wsaaTicket()
	if err := repository.SaveTicket(nil, key, ticket); err != nil {
		t.Fatal(err)
	}
	got, err := repository.GetTicket(nil, key)
	if err != nil || got.Token != ticket.Token {
		t.Fatalf("GetTicket() = %+v, %v", got, err)
	}
	other := key
	other.OrganizationID = uuid.New()
	if _, err := repository.GetTicket(nil, other); err == nil {
		t.Fatal("ticket crossed tenant boundary")
	}
}

func wsaaKey(organizationID uuid.UUID, fingerprint string) wsaa.TicketKey {
	return wsaa.TicketKey{
		OrganizationID: organizationID, Environment: ar.Homologation,
		Service: wsaa.ServiceWSFE, CertificateFingerprint: fingerprint,
	}
}

func wsaaTicket() wsaa.AccessTicket {
	return wsaa.AccessTicket{
		Token: "secret-token", Sign: "secret-sign", ExpiresAt: time.Now().Add(time.Hour),
	}
}
