package homologation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsaa"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsfev1"
)

type matrixCase struct {
	Name        string
	VoucherType ar.VoucherType
	Concept     ar.Concept
	Currency    string
	Rate        fiscal.Decimal
}

type localMatrixEvidence struct {
	CaseName        string            `json:"case_name"`
	VoucherType     int               `json:"voucher_type"`
	Concept         int               `json:"concept"`
	Currency        string            `json:"currency"`
	HasAssociation  bool              `json:"has_association"`
	SnapshotSHA256  string            `json:"snapshot_sha256"`
	EnvelopeSHA256  string            `json:"local_envelope_sha256"`
	ArtifactSHA256  map[string]string `json:"artifact_sha256"`
	Deterministic   bool              `json:"deterministic"`
	NetworkEmission bool              `json:"network_emission"`
}

func localCases() []matrixCase {
	return []matrixCase{
		{
			Name: "invoice_a_products_ars", VoucherType: ar.InvoiceA,
			Concept: ar.ConceptProducts, Currency: ar.CurrencyPES,
			Rate: fiscal.MustDecimal("1"),
		},
		{
			Name: "debit_note_a_services_usd", VoucherType: ar.DebitNoteA,
			Concept: ar.ConceptServices, Currency: ar.CurrencyDOL,
			Rate: fiscal.MustDecimal("1325.125"),
		},
		{
			Name: "credit_note_a_mixed_ars", VoucherType: ar.CreditNoteA,
			Concept: ar.ConceptMixed, Currency: ar.CurrencyPES,
			Rate: fiscal.MustDecimal("1"),
		},
		{
			Name: "invoice_b_services_usd", VoucherType: ar.InvoiceB,
			Concept: ar.ConceptServices, Currency: ar.CurrencyDOL,
			Rate: fiscal.MustDecimal("1325.125"),
		},
		{
			Name: "debit_note_b_mixed_ars", VoucherType: ar.DebitNoteB,
			Concept: ar.ConceptMixed, Currency: ar.CurrencyPES,
			Rate: fiscal.MustDecimal("1"),
		},
		{
			Name: "credit_note_b_products_ars", VoucherType: ar.CreditNoteB,
			Concept: ar.ConceptProducts, Currency: ar.CurrencyPES,
			Rate: fiscal.MustDecimal("1"),
		},
		{
			Name: "invoice_c_mixed_usd", VoucherType: ar.InvoiceC,
			Concept: ar.ConceptMixed, Currency: ar.CurrencyDOL,
			Rate: fiscal.MustDecimal("1325.125"),
		},
		{
			Name: "debit_note_c_products_ars", VoucherType: ar.DebitNoteC,
			Concept: ar.ConceptProducts, Currency: ar.CurrencyPES,
			Rate: fiscal.MustDecimal("1"),
		},
		{
			Name: "credit_note_c_services_ars", VoucherType: ar.CreditNoteC,
			Concept: ar.ConceptServices, Currency: ar.CurrencyPES,
			Rate: fiscal.MustDecimal("1"),
		},
	}
}

func validateLocalMatrixCase(
	ctx context.Context,
	renderer fiscal.ArtifactRenderer,
	configuration Configuration,
	pointOfSale int,
	at time.Time,
	testCase matrixCase,
) (localMatrixEvidence, error) {
	if renderer == nil {
		return localMatrixEvidence{}, fmt.Errorf("local matrix renderer is required")
	}
	receiver, receiverCondition, receiverSnapshot, err := matrixReceiver(testCase.VoucherType)
	if err != nil {
		return localMatrixEvidence{}, err
	}
	totals, err := ar.CalculateTotals(
		testCase.VoucherType,
		[]ar.TaxableAmount{{
			Category: ar.Taxable,
			Amount:   fiscal.MustDecimal("100"),
			Rate:     fiscal.MustDecimal("21"),
		}},
		nil,
	)
	if err != nil {
		return localMatrixEvidence{}, err
	}
	issueDate := at.UTC().Format("2006-01-02")
	request := wsfev1.Request{
		PointOfSale: pointOfSale, VoucherType: testCase.VoucherType,
		Concept: testCase.Concept, Receiver: receiver,
		ReceiverVATCondition: receiverCondition,
		Number:               1, IssueDate: issueDate, Totals: totals,
		Currency: testCase.Currency, ExchangeRate: testCase.Rate,
	}
	if testCase.Concept.NeedsServiceDates() {
		request.ServiceFrom = issueDate
		request.ServiceTo = issueDate
		request.PaymentDue = at.UTC().AddDate(0, 0, 10).Format("2006-01-02")
	}
	operation, err := testCase.VoucherType.Operation()
	if err != nil {
		return localMatrixEvidence{}, err
	}
	var associatedSnapshot *fiscal.AssociatedDocumentSnapshot
	if operation != fiscal.OperationInvoice {
		originalType, err := invoiceTypeFor(testCase.VoucherType)
		if err != nil {
			return localMatrixEvidence{}, err
		}
		request.Associated = &wsfev1.AssociatedVoucher{
			Type: originalType, PointOfSale: pointOfSale, Number: 1,
			IssueDate: issueDate,
		}
		associatedSnapshot = &fiscal.AssociatedDocumentSnapshot{
			VoucherID: "00000000-0000-0000-0000-000000000001",
			Type:      int(originalType), PointOfSale: pointOfSale,
			Number: 1, IssueDate: issueDate,
		}
	}
	if err := request.Validate(); err != nil {
		return localMatrixEvidence{}, fmt.Errorf("validate local WSFE matrix request: %w", err)
	}
	// Building an envelope locally exercises the exact WSFE serializer. It is
	// deliberately never passed to SOAPTransport by this package.
	envelope, err := wsfev1.BuildAuthorizeEnvelope(wsfev1.Auth{
		Ticket: wsaa.AccessTicket{
			Token: "local-validation-token",
			Sign:  "local-validation-sign",
		},
		CUIT: ar.CUIT(configuration.CUIT),
	}, request)
	if err != nil {
		return localMatrixEvidence{}, fmt.Errorf("build local WSFE matrix envelope: %w", err)
	}
	envelopeHash := sha256.Sum256(envelope)

	taxAmount := totals.VAT
	totalAmount := totals.Total
	functionalTotal := totalAmount.Mul(testCase.Rate)
	taxCode := "IVA_21"
	taxRate := fiscal.MustDecimal("21")
	if testCase.VoucherType.IsTypeC() {
		taxCode = ""
		taxRate = fiscal.Decimal{}
	}
	document := fiscal.FiscalSnapshot{
		Version:     fiscal.SnapshotVersion,
		CountryCode: "AR",
		IssueDate:   issueDate,
		Issuer: fiscal.PartySnapshot{
			Name: configuration.LegalName, TaxID: configuration.CUIT,
			TaxCondition:     configuration.IssuerVATCondition,
			Address:          configuration.LegalAddress,
			ActivityStartDay: configuration.ActivityStartDate.UTC().Format("2006-01-02"),
		},
		Receiver: receiverSnapshot,
		Currency: fiscal.CurrencySnapshot{
			Code: testCase.Currency, Rate: testCase.Rate,
			RateDate: issueDate, RateSource: "homologation-local-fixture",
		},
		Lines: []fiscal.FiscalLineSnapshot{{
			Position: 1, Description: "Caso técnico local " + testCase.Name,
			Quantity: fiscal.MustDecimal("1"), UnitPrice: fiscal.MustDecimal("100"),
			NetAmount: fiscal.MustDecimal("100"), TaxCode: taxCode,
			TaxRate: taxRate, TaxAmount: taxAmount,
			TotalAmount: totalAmount,
		}},
		Totals: fiscal.FiscalTotalsSnapshot{
			NetTaxed: totals.NetTaxed, NetUntaxed: totals.NetUntaxed,
			Exempt: totals.Exempt, VAT: totals.VAT, OtherTaxes: totals.Tributes,
			Total: totals.Total, Functional: functionalTotal,
		},
		AssociatedDocument: associatedSnapshot,
		Metadata: map[string]string{
			"voucher_type":  strconv.Itoa(int(testCase.VoucherType)),
			"point_of_sale": strconv.Itoa(pointOfSale),
			"concept":       conceptName(testCase.Concept),
			"evidence_only": "true",
		},
	}
	if testCase.Concept.NeedsServiceDates() {
		document.ServiceFrom = request.ServiceFrom
		document.ServiceTo = request.ServiceTo
		document.PaymentDue = request.PaymentDue
	}
	snapshot, err := fiscal.NewSnapshot(document)
	if err != nil {
		return localMatrixEvidence{}, fmt.Errorf("build local fiscal snapshot: %w", err)
	}
	authorization := fiscal.Authorization{
		Decision:  fiscal.DecisionAuthorized,
		Code:      "70000000000000",
		ExpiresOn: at.UTC().AddDate(0, 0, 10).Format("2006-01-02"),
		Number:    1, ProcessedAt: at.UTC(),
	}
	first, err := renderer.Render(ctx, snapshot, authorization)
	if err != nil {
		return localMatrixEvidence{}, fmt.Errorf("render local fiscal evidence: %w", err)
	}
	second, err := renderer.Render(ctx, snapshot, authorization)
	if err != nil {
		return localMatrixEvidence{}, fmt.Errorf("repeat local fiscal evidence render: %w", err)
	}
	if len(first) == 0 || len(first) != len(second) {
		return localMatrixEvidence{}, fmt.Errorf("local fiscal renderer returned inconsistent artifacts")
	}
	artifactHashes := make(map[string]string, len(first))
	for index := range first {
		if first[index].Kind != second[index].Kind ||
			first[index].ContentType != second[index].ContentType ||
			!bytes.Equal(first[index].Body, second[index].Body) {
			return localMatrixEvidence{}, fmt.Errorf(
				"local fiscal artifact %d is not deterministic", index,
			)
		}
		sum := sha256.Sum256(first[index].Body)
		artifactHashes[first[index].Kind] = hex.EncodeToString(sum[:])
	}
	return localMatrixEvidence{
		CaseName: testCase.Name, VoucherType: int(testCase.VoucherType),
		Concept: int(testCase.Concept), Currency: testCase.Currency,
		HasAssociation: associatedSnapshot != nil,
		SnapshotSHA256: snapshot.Hash(),
		EnvelopeSHA256: hex.EncodeToString(envelopeHash[:]),
		ArtifactSHA256: artifactHashes,
		Deterministic:  true, NetworkEmission: false,
	}, nil
}

func matrixReceiver(
	voucherType ar.VoucherType,
) (ar.ReceiverDocument, ar.VATCondition, fiscal.PartySnapshot, error) {
	letter, err := voucherType.Letter()
	if err != nil {
		return ar.ReceiverDocument{}, 0, fiscal.PartySnapshot{}, err
	}
	if letter == "A" {
		document, err := ar.NewReceiverDocument(ar.DocumentCUIT, "30710158211")
		return document, ar.VATRegistered, fiscal.PartySnapshot{
			Name: "Receptor técnico RI", TaxCondition: "responsable_inscripto",
			DocumentType:   strconv.Itoa(int(ar.DocumentCUIT)),
			DocumentNumber: document.Number,
		}, err
	}
	document, err := ar.NewReceiverDocument(ar.DocumentConsumerFinal, "0")
	return document, ar.VATConsumerFinal, fiscal.PartySnapshot{
		Name: "Consumidor final técnico", TaxCondition: "consumidor_final",
		DocumentType:   strconv.Itoa(int(ar.DocumentConsumerFinal)),
		DocumentNumber: document.Number,
	}, err
}

func invoiceTypeFor(voucherType ar.VoucherType) (ar.VoucherType, error) {
	letter, err := voucherType.Letter()
	if err != nil {
		return 0, err
	}
	switch letter {
	case "A":
		return ar.InvoiceA, nil
	case "B":
		return ar.InvoiceB, nil
	case "C":
		return ar.InvoiceC, nil
	default:
		return 0, fmt.Errorf("unsupported voucher letter %q", letter)
	}
}

func conceptName(concept ar.Concept) string {
	switch concept {
	case ar.ConceptProducts:
		return "products"
	case ar.ConceptServices:
		return "services"
	case ar.ConceptMixed:
		return "mixed"
	default:
		return ""
	}
}
