package wsfev1

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar/wsaa"
)

const Namespace = "http://ar.gov.afip.dif.FEV1/"

type Auth struct {
	Ticket wsaa.AccessTicket
	CUIT   ar.CUIT
}

func (auth Auth) validate() error {
	if strings.TrimSpace(auth.Ticket.Token) == "" || strings.TrimSpace(auth.Ticket.Sign) == "" {
		return errors.New("WSFE access ticket is required")
	}
	if _, err := ar.ParseCUIT(auth.CUIT.String()); err != nil {
		return fmt.Errorf("WSFE issuer CUIT: %w", err)
	}
	return nil
}

type AssociatedVoucher struct {
	Type        ar.VoucherType
	PointOfSale int
	Number      int64
	IssuerCUIT  ar.CUIT
	IssueDate   string
}

type Request struct {
	PointOfSale          int
	VoucherType          ar.VoucherType
	Concept              ar.Concept
	Receiver             ar.ReceiverDocument
	ReceiverVATCondition ar.VATCondition
	Number               int64
	IssueDate            string
	Totals               ar.Totals
	Tributes             []ar.Tribute
	Currency             string
	ExchangeRate         fiscal.Decimal
	ServiceFrom          string
	ServiceTo            string
	PaymentDue           string
	Associated           *AssociatedVoucher
	ActivityIDs          []int64
}

func (request Request) Validate() error {
	if request.PointOfSale <= 0 || request.PointOfSale > 99999 {
		return errors.New("WSFE point of sale must be between 1 and 99999")
	}
	if !request.VoucherType.ValidMVP() {
		return fmt.Errorf("WSFE voucher type %d is outside the MVP", request.VoucherType)
	}
	if !request.Concept.Valid() {
		return errors.New("WSFE concept must be products, services, or mixed")
	}
	normalizedReceiver, err := ar.NewReceiverDocument(request.Receiver.Type, request.Receiver.Number)
	if err != nil {
		return fmt.Errorf("WSFE receiver document: %w", err)
	}
	request.Receiver = normalizedReceiver
	if err := ar.ValidateReceiver(request.ReceiverVATCondition, request.Receiver); err != nil {
		return fmt.Errorf("WSFE receiver: %w", err)
	}
	if request.Number <= 0 || request.Number > 99999999 {
		return errors.New("WSFE voucher number must be between 1 and 99999999")
	}
	issueDate, err := parseARCADate(request.IssueDate)
	if err != nil {
		return fmt.Errorf("WSFE issue date: %w", err)
	}
	if err := validateTotals(request.VoucherType, request.Totals, request.Tributes); err != nil {
		return err
	}
	currency, err := ar.CurrencyCode(request.Currency)
	if err != nil {
		return err
	}
	if request.ExchangeRate.Cmp(fiscal.Decimal{}) <= 0 {
		return errors.New("WSFE exchange rate must be positive")
	}
	if currency == ar.CurrencyPES && !request.ExchangeRate.Equal(fiscal.NewDecimalFromInt(1)) {
		return errors.New("WSFE PES exchange rate must equal 1")
	}

	if request.Concept.NeedsServiceDates() {
		from, err := parseARCADate(request.ServiceFrom)
		if err != nil {
			return fmt.Errorf("WSFE service_from: %w", err)
		}
		to, err := parseARCADate(request.ServiceTo)
		if err != nil {
			return fmt.Errorf("WSFE service_to: %w", err)
		}
		due, err := parseARCADate(request.PaymentDue)
		if err != nil {
			return fmt.Errorf("WSFE payment_due: %w", err)
		}
		if to.Before(from) {
			return errors.New("WSFE service_to cannot precede service_from")
		}
		if due.Before(issueDate) {
			return errors.New("WSFE payment_due cannot precede issue_date")
		}
	} else if request.ServiceFrom != "" || request.ServiceTo != "" || request.PaymentDue != "" {
		return errors.New("WSFE service dates are only valid for services or mixed concepts")
	}

	operation, _ := request.VoucherType.Operation()
	if operation == fiscal.OperationInvoice {
		if request.Associated != nil {
			return errors.New("WSFE invoice cannot reference an associated voucher")
		}
	} else {
		if request.Associated == nil {
			return errors.New("WSFE credit/debit notes require the original voucher")
		}
		expectedType, err := ar.NoteTypeFor(request.Associated.Type, operation)
		if err != nil {
			return err
		}
		if expectedType != request.VoucherType {
			return fmt.Errorf(
				"WSFE note type %d does not match original voucher type %d",
				request.VoucherType, request.Associated.Type,
			)
		}
		if request.Associated.PointOfSale <= 0 || request.Associated.Number <= 0 {
			return errors.New("WSFE associated voucher point and number are required")
		}
		if request.Associated.IssueDate != "" {
			if _, err := parseARCADate(request.Associated.IssueDate); err != nil {
				return fmt.Errorf("WSFE associated issue date: %w", err)
			}
		}
		if request.Associated.IssuerCUIT != "" {
			if _, err := ar.ParseCUIT(request.Associated.IssuerCUIT.String()); err != nil {
				return fmt.Errorf("WSFE associated issuer CUIT: %w", err)
			}
		}
	}
	for _, activityID := range request.ActivityIDs {
		if activityID <= 0 {
			return errors.New("WSFE activity IDs must be positive")
		}
	}
	return nil
}

func validateTotals(voucherType ar.VoucherType, totals ar.Totals, tributes []ar.Tribute) error {
	for label, amount := range map[string]fiscal.Decimal{
		"net_taxed":   totals.NetTaxed,
		"net_untaxed": totals.NetUntaxed,
		"exempt":      totals.Exempt,
		"tributes":    totals.Tributes,
		"vat":         totals.VAT,
		"total":       totals.Total,
	} {
		if amount.IsNegative() {
			return fmt.Errorf("WSFE %s cannot be negative", label)
		}
	}
	tributeSum := fiscal.Decimal{}
	for index, tribute := range tributes {
		if tribute.ID <= 0 || tribute.Amount.IsNegative() {
			return fmt.Errorf("WSFE tribute %d is invalid", index)
		}
		tributeSum = tributeSum.Add(tribute.Amount)
	}
	if !tributeSum.Equal(totals.Tributes) {
		return fmt.Errorf(
			"WSFE tribute detail %s does not equal ImpTrib %s", tributeSum, totals.Tributes,
		)
	}
	if voucherType.IsTypeC() {
		if !totals.VAT.IsZero() || len(totals.VATLines) != 0 ||
			!totals.NetUntaxed.IsZero() || !totals.Exempt.IsZero() {
			return errors.New("WSFE type C cannot discriminate VAT, exempt, or untaxed amounts")
		}
		expected := totals.NetTaxed.Add(totals.Tributes)
		if !expected.Equal(totals.Total) {
			return errors.New("WSFE type C total must equal net plus tributes")
		}
		return nil
	}

	vatSum := fiscal.Decimal{}
	ids := make(map[int]struct{}, len(totals.VATLines))
	for index, line := range totals.VATLines {
		if _, duplicate := ids[line.ID]; duplicate {
			return fmt.Errorf("WSFE duplicate VAT ID %d", line.ID)
		}
		ids[line.ID] = struct{}{}
		rate, valid := ar.VATRateForID(line.ID)
		if !valid || !rate.Equal(line.Rate) {
			return fmt.Errorf("WSFE VAT line %d has inconsistent ID/rate", index)
		}
		if line.BaseAmount.IsNegative() || line.Amount.IsNegative() {
			return fmt.Errorf("WSFE VAT line %d cannot be negative", index)
		}
		vatSum = vatSum.Add(line.Amount)
	}
	if !vatSum.Equal(totals.VAT) {
		return fmt.Errorf("WSFE VAT detail %s does not equal ImpIVA %s", vatSum, totals.VAT)
	}
	expected := totals.NetTaxed.Add(totals.NetUntaxed).
		Add(totals.Exempt).Add(totals.Tributes).Add(totals.VAT)
	if !expected.Equal(totals.Total) {
		return fmt.Errorf("WSFE components %s do not equal ImpTotal %s", expected, totals.Total)
	}
	return nil
}

func parseARCADate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layout := "20060102"
	if len(value) == len("2006-01-02") {
		layout = "2006-01-02"
	}
	date, err := time.Parse(layout, value)
	if err != nil {
		return time.Time{}, errors.New("date must use YYYYMMDD or YYYY-MM-DD")
	}
	return date, nil
}

func formatARCADate(value string) (string, error) {
	date, err := parseARCADate(value)
	if err != nil {
		return "", err
	}
	return date.Format("20060102"), nil
}

type Note struct {
	Code    int
	Message string
}

type AuthorizationResult struct {
	Decision     fiscal.AuthorityDecision
	CAE          string
	CAEExpiresOn string
	Number       int64
	ProcessedAt  time.Time
	Observations []Note
	Errors       []Note
	RawResponse  []byte
}

func (result AuthorizationResult) AuthorityResult() fiscal.AuthorityResult {
	observations := make([]fiscal.AuthorityNote, 0, len(result.Observations))
	for _, note := range result.Observations {
		observations = append(observations, fiscal.AuthorityNote{
			Code: strconv.Itoa(note.Code), Message: note.Message,
		})
	}
	errs := make([]fiscal.AuthorityNote, 0, len(result.Errors))
	for _, note := range result.Errors {
		errs = append(errs, fiscal.AuthorityNote{
			Code: strconv.Itoa(note.Code), Message: note.Message,
		})
	}
	return fiscal.AuthorityResult{
		Decision: result.Decision, Code: result.CAE, ExpiresOn: result.CAEExpiresOn,
		Number: result.Number, ProcessedAt: result.ProcessedAt,
		Observations: observations, Errors: errs,
		RawResponse: append([]byte(nil), result.RawResponse...),
	}
}

type ConsultResult struct {
	Found        bool
	VoucherType  ar.VoucherType
	PointOfSale  int
	Number       int64
	Concept      ar.Concept
	Receiver     ar.ReceiverDocument
	IssueDate    string
	Totals       ar.Totals
	Currency     string
	ExchangeRate fiscal.Decimal
	Decision     fiscal.AuthorityDecision
	Code         string
	EmissionType string
	ExpiresOn    string
	ProcessedAt  time.Time
	Observations []Note
	Errors       []Note
	RawResponse  []byte
}

func (result ConsultResult) AuthorityLookup() fiscal.AuthorityLookup {
	if !result.Found {
		return fiscal.AuthorityLookup{Found: false}
	}
	authorization := AuthorizationResult{
		Decision:     result.Decision,
		CAE:          result.Code,
		CAEExpiresOn: result.ExpiresOn,
		Number:       result.Number,
		ProcessedAt:  result.ProcessedAt,
		Observations: result.Observations,
		Errors:       result.Errors,
		RawResponse:  result.RawResponse,
	}
	return fiscal.AuthorityLookup{Found: true, Result: authorization.AuthorityResult()}
}
