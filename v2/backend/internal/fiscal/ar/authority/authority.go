package authority

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsaa"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsfev1"
)

type CredentialsProvider interface {
	Credentials(context.Context, fiscal.Voucher) (wsaa.Credentials, error)
}

type Adapter struct {
	credentials   CredentialsProvider
	authenticator *wsaa.Authenticator
	transport     ar.SOAPTransport
}

func New(
	credentials CredentialsProvider,
	authenticator *wsaa.Authenticator,
	transport ar.SOAPTransport,
) (*Adapter, error) {
	if credentials == nil || authenticator == nil || transport == nil {
		return nil, errors.New("Argentina fiscal authority dependencies are required")
	}
	return &Adapter{
		credentials: credentials, authenticator: authenticator, transport: transport,
	}, nil
}

func (adapter *Adapter) LastAuthorized(
	ctx context.Context,
	voucher fiscal.Voucher,
) (int64, error) {
	auth, client, err := adapter.client(ctx, voucher)
	if err != nil {
		return 0, err
	}
	return client.LastAuthorized(
		ctx, auth, voucher.PointOfSale, ar.VoucherType(voucher.AuthorityType),
	)
}

func (adapter *Adapter) Authorize(
	ctx context.Context,
	voucher fiscal.Voucher,
) (fiscal.AuthorityResult, error) {
	auth, client, err := adapter.client(ctx, voucher)
	if err != nil {
		return fiscal.AuthorityResult{}, err
	}
	request, err := requestFor(voucher)
	if err != nil {
		return fiscal.AuthorityResult{}, err
	}
	result, err := client.Authorize(ctx, auth, request)
	if err != nil {
		return fiscal.AuthorityResult{}, err
	}
	return result.AuthorityResult(), nil
}

func (adapter *Adapter) Consult(
	ctx context.Context,
	voucher fiscal.Voucher,
) (fiscal.AuthorityLookup, error) {
	auth, client, err := adapter.client(ctx, voucher)
	if err != nil {
		return fiscal.AuthorityLookup{}, err
	}
	result, err := client.Consult(
		ctx,
		auth,
		voucher.PointOfSale,
		ar.VoucherType(voucher.AuthorityType),
		voucher.Number,
	)
	if err != nil {
		return fiscal.AuthorityLookup{}, err
	}
	if !result.Found {
		return fiscal.AuthorityLookup{Found: false}, nil
	}
	if err := validateConsultedVoucher(voucher, result); err != nil {
		return fiscal.AuthorityLookup{}, err
	}
	return result.AuthorityLookup(), nil
}

func (adapter *Adapter) client(
	ctx context.Context,
	voucher fiscal.Voucher,
) (wsfev1.Auth, *wsfev1.Client, error) {
	credentials, err := adapter.credentials.Credentials(ctx, voucher)
	if err != nil {
		return wsfev1.Auth{}, nil, err
	}
	if credentials.OrganizationID != voucher.OrganizationID ||
		string(credentials.Environment) != voucher.Environment {
		return wsfev1.Auth{}, nil, errors.New("fiscal authority credentials do not match voucher scope")
	}
	ticket, err := adapter.authenticator.AccessTicket(ctx, credentials)
	if err != nil {
		return wsfev1.Auth{}, nil, err
	}
	client, err := wsfev1.NewClient(adapter.transport, credentials.Environment)
	if err != nil {
		return wsfev1.Auth{}, nil, err
	}
	return wsfev1.Auth{Ticket: ticket, CUIT: credentials.CUIT}, client, nil
}

func requestFor(voucher fiscal.Voucher) (wsfev1.Request, error) {
	if voucher.Number <= 0 {
		return wsfev1.Request{}, errors.New("fiscal voucher must have a reserved number")
	}
	document, err := voucher.Snapshot.Document()
	if err != nil {
		return wsfev1.Request{}, err
	}
	voucherType := ar.VoucherType(voucher.AuthorityType)
	if !voucherType.ValidMVP() {
		return wsfev1.Request{}, errors.New("unsupported Argentina voucher type")
	}
	concept, err := conceptFromSnapshot(document)
	if err != nil {
		return wsfev1.Request{}, err
	}
	documentType, err := strconv.Atoi(strings.TrimSpace(document.Receiver.DocumentType))
	if err != nil {
		return wsfev1.Request{}, errors.New("invalid receiver document type in snapshot")
	}
	receiver, err := ar.NewReceiverDocument(
		ar.DocumentType(documentType),
		nonEmpty(document.Receiver.DocumentNumber, document.Receiver.TaxID),
	)
	if err != nil {
		return wsfev1.Request{}, err
	}
	receiverCondition, err := ar.ParseVATCondition(document.Receiver.TaxCondition)
	if err != nil {
		return wsfev1.Request{}, err
	}
	vatLines, err := vatLines(document.Lines)
	if err != nil {
		return wsfev1.Request{}, err
	}
	tributes := tributeLines(document.Taxes)
	request := wsfev1.Request{
		PointOfSale: voucher.PointOfSale, VoucherType: voucherType,
		Concept: concept, Receiver: receiver, ReceiverVATCondition: receiverCondition,
		Number: voucher.Number, IssueDate: document.IssueDate,
		Totals: ar.Totals{
			NetTaxed: document.Totals.NetTaxed, NetUntaxed: document.Totals.NetUntaxed,
			Exempt: document.Totals.Exempt, Tributes: document.Totals.OtherTaxes,
			VAT: document.Totals.VAT, Total: document.Totals.Total, VATLines: vatLines,
		},
		Tributes: tributes, Currency: document.Currency.Code,
		ExchangeRate: document.Currency.Rate,
		ServiceFrom:  document.ServiceFrom, ServiceTo: document.ServiceTo,
		PaymentDue: document.PaymentDue,
	}
	if document.AssociatedDocument != nil {
		issuerCUIT := ar.CUIT("")
		if document.AssociatedDocument.IssuerTaxID != "" {
			issuerCUIT, err = ar.ParseCUIT(document.AssociatedDocument.IssuerTaxID)
			if err != nil {
				return wsfev1.Request{}, err
			}
		}
		request.Associated = &wsfev1.AssociatedVoucher{
			Type:        ar.VoucherType(document.AssociatedDocument.Type),
			PointOfSale: document.AssociatedDocument.PointOfSale,
			Number:      document.AssociatedDocument.Number,
			IssuerCUIT:  issuerCUIT,
			IssueDate:   document.AssociatedDocument.IssueDate,
		}
	}
	if err := request.Validate(); err != nil {
		return wsfev1.Request{}, err
	}
	return request, nil
}

func conceptFromSnapshot(document fiscal.FiscalSnapshot) (ar.Concept, error) {
	switch strings.ToLower(strings.TrimSpace(document.Metadata["concept"])) {
	case "products":
		return ar.ConceptProducts, nil
	case "services":
		return ar.ConceptServices, nil
	case "mixed":
		return ar.ConceptMixed, nil
	default:
		return 0, errors.New("invalid fiscal concept in snapshot")
	}
}

func vatLines(lines []fiscal.FiscalLineSnapshot) ([]ar.VATBreakdown, error) {
	type accumulated struct {
		id     int
		rate   fiscal.Decimal
		base   fiscal.Decimal
		amount fiscal.Decimal
	}
	orderedKeys := make([]string, 0)
	byRate := make(map[string]accumulated)
	for _, line := range lines {
		if line.TaxRate.IsZero() && line.TaxAmount.IsZero() {
			continue
		}
		id, found := ar.VATIDForRate(line.TaxRate)
		if !found {
			return nil, fmt.Errorf("unsupported VAT rate %s", line.TaxRate)
		}
		key := line.TaxRate.String()
		value, exists := byRate[key]
		if !exists {
			value.id, value.rate = id, line.TaxRate
			orderedKeys = append(orderedKeys, key)
		}
		value.base = value.base.Add(line.NetAmount)
		value.amount = value.amount.Add(line.TaxAmount)
		byRate[key] = value
	}
	result := make([]ar.VATBreakdown, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		value := byRate[key]
		result = append(result, ar.VATBreakdown{
			ID: value.id, Rate: value.rate, BaseAmount: value.base, Amount: value.amount,
		})
	}
	return result, nil
}

func tributeLines(taxes []fiscal.TaxSnapshot) []ar.Tribute {
	result := make([]ar.Tribute, 0, len(taxes))
	for _, tax := range taxes {
		result = append(result, ar.Tribute{
			ID: 99, Description: nonEmpty(tax.Description, tax.Code),
			BaseAmount: tax.BaseAmount, Rate: tax.Rate, Amount: tax.Amount,
		})
	}
	return result
}

func validateConsultedVoucher(voucher fiscal.Voucher, result wsfev1.ConsultResult) error {
	document, err := voucher.Snapshot.Document()
	if err != nil {
		return err
	}
	expected, err := requestFor(voucher)
	if err != nil {
		return err
	}
	if result.PointOfSale != expected.PointOfSale ||
		result.VoucherType != expected.VoucherType ||
		result.Number != expected.Number ||
		result.Concept != expected.Concept ||
		result.Receiver != expected.Receiver ||
		result.Currency != document.Currency.Code ||
		!result.ExchangeRate.Equal(document.Currency.Rate) ||
		!result.Totals.Total.Equal(document.Totals.Total) {
		return fmt.Errorf("%w: consulted ARCA voucher does not match immutable snapshot", fiscal.ErrSequenceConflict)
	}
	return nil
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
