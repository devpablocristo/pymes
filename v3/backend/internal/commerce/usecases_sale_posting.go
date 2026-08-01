package commerce

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

const (
	postingMoneyScale = 2
	exchangeRateScale = 6
)

var unsignedDecimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)

type fiscalPostingSnapshot struct {
	IssueDate    string `json:"issue_date"`
	Currency     string `json:"currency"`
	ExchangeRate string `json:"exchange_rate,omitempty"`
	Totals       struct {
		Net    string `json:"net"`
		VAT    string `json:"vat"`
		Exempt string `json:"exempt"`
		Total  string `json:"total"`
	} `json:"totals"`
	AssociatedVoucher *struct {
		PointOfSale   int    `json:"point_of_sale"`
		DocumentType  string `json:"document_type"`
		VoucherNumber int    `json:"voucher_number"`
		IssueDate     string `json:"issue_date"`
	} `json:"associated_voucher,omitempty"`
}

// buildSalePostingCommand applies the accounting normalization policy at the
// orchestration boundary:
//   - the frozen fiscal values must balance exactly before any rounding;
//   - posting and functional amounts use two decimals, half away from zero;
//   - exchange rates use six decimals, half away from zero;
//   - any cent residual is assigned to revenue, never to VAT or receivables.
//
// Exempt sales are part of revenue (4100). VAT is always isolated in 2200 when
// non-zero so that the receivable remains equal to the fiscal total.
func buildSalePostingCommand(sale domain.Sale, original *domain.Sale) (domain.PostingCommand, error) {
	snapshot, err := decodeFiscalPostingSnapshot(sale.FiscalSnapshot)
	if err != nil {
		return domain.PostingCommand{}, err
	}
	sourceType, err := postingSourceType(sale.Voucher.DocumentType)
	if err != nil {
		return domain.PostingCommand{}, err
	}
	effectiveAt, err := time.Parse(time.DateOnly, snapshot.IssueDate)
	if err != nil || effectiveAt.Format(time.DateOnly) != snapshot.IssueDate {
		return domain.PostingCommand{}, postingValidationError("invalid fiscal issue_date")
	}
	if snapshot.Currency != sale.Total.Currency {
		return domain.PostingCommand{}, postingValidationError("snapshot currency differs from sale currency")
	}

	net, err := parsePostingDecimal("totals.net", snapshot.Totals.Net, true)
	if err != nil {
		return domain.PostingCommand{}, err
	}
	vat, err := parsePostingDecimal("totals.vat", snapshot.Totals.VAT, true)
	if err != nil {
		return domain.PostingCommand{}, err
	}
	exempt, err := parsePostingDecimal("totals.exempt", snapshot.Totals.Exempt, true)
	if err != nil {
		return domain.PostingCommand{}, err
	}
	total, err := parsePostingDecimal("totals.total", snapshot.Totals.Total, true)
	if err != nil {
		return domain.PostingCommand{}, err
	}
	saleTotal, err := parsePostingDecimal("sale.total", sale.Total.Amount, true)
	if err != nil {
		return domain.PostingCommand{}, err
	}
	components := new(big.Rat).Add(net, vat)
	components.Add(components, exempt)
	if components.Cmp(total) != 0 {
		return domain.PostingCommand{}, postingValidationError("fiscal total does not equal net + VAT + exempt")
	}
	if saleTotal.Cmp(total) != 0 {
		return domain.PostingCommand{}, postingValidationError("sale total differs from frozen fiscal total")
	}

	rate, normalizedRate, err := normalizedPostingExchangeRate(snapshot.Currency, snapshot.ExchangeRate)
	if err != nil {
		return domain.PostingCommand{}, err
	}

	totalMinor := roundToScale(total, postingMoneyScale)
	vatMinor := roundToScale(vat, postingMoneyScale)
	revenueMinor := new(big.Int).Sub(new(big.Int).Set(totalMinor), vatMinor)
	if revenueMinor.Sign() < 0 {
		return domain.PostingCommand{}, postingValidationError("rounded VAT exceeds rounded total")
	}

	functionalTotal := convertMinorAmount(totalMinor, rate)
	functionalVAT := convertMinorAmount(vatMinor, rate)
	functionalRevenue := new(big.Int).Sub(new(big.Int).Set(functionalTotal), functionalVAT)
	if functionalRevenue.Sign() < 0 {
		return domain.PostingCommand{}, postingValidationError("functional VAT exceeds functional total")
	}

	zero := domain.Money{Amount: fixedDecimal(big.NewInt(0), postingMoneyScale), Currency: snapshot.Currency}
	totalMoney := domain.Money{Amount: fixedDecimal(totalMinor, postingMoneyScale), Currency: snapshot.Currency}
	revenueMoney := domain.Money{Amount: fixedDecimal(revenueMinor, postingMoneyScale), Currency: snapshot.Currency}
	vatMoney := domain.Money{Amount: fixedDecimal(vatMinor, postingMoneyScale), Currency: snapshot.Currency}

	receivable := domain.PostingLine{
		AccountCode:      "1200",
		Debit:            totalMoney,
		Credit:           zero,
		FunctionalAmount: fixedDecimal(functionalTotal, postingMoneyScale),
		Memo:             "Total comprobante",
		OpenItem:         true,
		PartyRef:         sale.RecipientRef,
	}
	revenueLine := domain.PostingLine{
		AccountCode:      "4100",
		Debit:            zero,
		Credit:           revenueMoney,
		FunctionalAmount: fixedDecimal(functionalRevenue, postingMoneyScale),
		Memo:             "Neto gravado + exento",
	}
	vatLine := domain.PostingLine{
		AccountCode:      "2200",
		Debit:            zero,
		Credit:           vatMoney,
		FunctionalAmount: fixedDecimal(functionalVAT, postingMoneyScale),
		Memo:             "IVA débito fiscal",
	}
	lines := []domain.PostingLine{receivable, revenueLine}
	if vatMinor.Sign() != 0 {
		lines = append(lines, vatLine)
	}

	sourceVersion := persistedSourceVersion(sale.Origin)
	command := domain.PostingCommand{
		CommandID:      accountingCommandID("posting", sale.ID, sourceVersion),
		OrganizationID: sale.OrganizationID,
		IdempotencyKey: internalIdempotencyKey(sale.OrganizationID, "accounting.post", sale.ID, sourceVersion),
		SourceType:     sourceType,
		SourceID:       sale.ID,
		SourceVersion:  sourceVersion,
		SnapshotDigest: sale.SnapshotDigest,
		CorrelationID:  persistedCorrelationID(sale.Origin, sale.CorrelationID),
		EffectiveAt:    effectiveAt.UTC(),
		Description:    "Venta " + sale.ID,
		ExchangeRate:   normalizedRate,
		Lines:          lines,
	}

	if sourceType == "sales_credit_note" || sourceType == "sales_debit_note" {
		if err := addOriginalSaleReference(&command, sale, original, snapshot); err != nil {
			return domain.PostingCommand{}, err
		}
		command.Description = fmt.Sprintf(
			"%s; origen %s; asiento %s",
			command.Description, command.RelatedSource.ID, command.OriginalJournalEntryID,
		)
		if sourceType == "sales_credit_note" {
			for index := range command.Lines {
				command.Lines[index].Debit, command.Lines[index].Credit = command.Lines[index].Credit, command.Lines[index].Debit
			}
		}
	}
	return command, nil
}

func decodeFiscalPostingSnapshot(raw json.RawMessage) (fiscalPostingSnapshot, error) {
	var snapshot fiscalPostingSnapshot
	if len(raw) == 0 || json.Unmarshal(raw, &snapshot) != nil {
		return fiscalPostingSnapshot{}, postingValidationError("frozen fiscal snapshot is required")
	}
	if snapshot.IssueDate == "" || snapshot.Currency == "" ||
		snapshot.Totals.Net == "" || snapshot.Totals.VAT == "" ||
		snapshot.Totals.Exempt == "" || snapshot.Totals.Total == "" {
		return fiscalPostingSnapshot{}, postingValidationError("frozen fiscal totals are incomplete")
	}
	return snapshot, nil
}

func addOriginalSaleReference(command *domain.PostingCommand, sale domain.Sale, original *domain.Sale, snapshot fiscalPostingSnapshot) error {
	if sale.SourceDocumentID == "" || original == nil || original.ID != sale.SourceDocumentID {
		return postingValidationError("credit/debit note source document is unavailable")
	}
	if original.OrganizationID != sale.OrganizationID || original.RecipientRef != sale.RecipientRef ||
		original.Total.Currency != sale.Total.Currency {
		return postingValidationError("credit/debit note source document does not match")
	}
	if original.JournalEntryID == "" {
		return postingValidationError("credit/debit note source journal entry is unavailable")
	}
	originalType, err := postingSourceType(original.Voucher.DocumentType)
	if err != nil || originalType != "sales_invoice" {
		return postingValidationError("associated source must be a posted invoice")
	}
	if snapshot.AssociatedVoucher == nil {
		return postingValidationError("associated voucher is required for credit/debit notes")
	}
	var originalSnapshot fiscalPostingSnapshot
	if len(original.FiscalSnapshot) == 0 || json.Unmarshal(original.FiscalSnapshot, &originalSnapshot) != nil ||
		originalSnapshot.IssueDate == "" {
		return postingValidationError("associated source fiscal snapshot is unavailable")
	}
	associated := snapshot.AssociatedVoucher
	if associated.PointOfSale != original.Voucher.PointOfSale ||
		associated.DocumentType != original.Voucher.DocumentType ||
		associated.VoucherNumber != original.Voucher.VoucherNumber ||
		associated.IssueDate != originalSnapshot.IssueDate {
		return postingValidationError("associated voucher does not match source invoice")
	}
	command.RelatedSource = &domain.PostingSourceReference{
		Type: originalType, ID: original.ID,
		Version: persistedSourceVersion(original.Origin), Digest: original.SnapshotDigest,
	}
	command.OriginalJournalEntryID = original.JournalEntryID
	return nil
}

func postingSourceType(documentType string) (string, error) {
	switch documentType {
	case "FA", "FB", "FC":
		return "sales_invoice", nil
	case "NCA", "NCB", "NCC":
		return "sales_credit_note", nil
	case "NDA", "NDB", "NDC":
		return "sales_debit_note", nil
	default:
		return "", postingValidationError("unsupported fiscal document type")
	}
}

func normalizedPostingExchangeRate(currency, raw string) (*big.Rat, string, error) {
	switch currency {
	case "ARS":
		if raw != "" {
			value, err := parsePostingDecimal("exchange_rate", raw, false)
			if err != nil {
				return nil, "", err
			}
			if value.Cmp(big.NewRat(1, 1)) != 0 {
				return nil, "", postingValidationError("ARS exchange_rate must be one")
			}
		}
		return big.NewRat(1, 1), fixedDecimal(big.NewInt(1_000_000), exchangeRateScale), nil
	case "USD", "EUR":
		if raw == "" {
			return nil, "", postingValidationError(currency + " exchange_rate is required")
		}
		value, err := parsePostingDecimal("exchange_rate", raw, false)
		if err != nil {
			return nil, "", err
		}
		normalizedMinor := roundToScale(value, exchangeRateScale)
		if normalizedMinor.Sign() <= 0 {
			return nil, "", postingValidationError("exchange_rate rounds to zero")
		}
		normalized := fixedDecimal(normalizedMinor, exchangeRateScale)
		rate, _ := new(big.Rat).SetString(normalized)
		return rate, normalized, nil
	default:
		return nil, "", postingValidationError("unsupported currency")
	}
}

func parsePostingDecimal(field, raw string, zeroAllowed bool) (*big.Rat, error) {
	if !unsignedDecimalPattern.MatchString(raw) {
		return nil, postingValidationError(field + " must be an unsigned base-ten decimal")
	}
	value, ok := new(big.Rat).SetString(raw)
	if !ok || value.Sign() < 0 || (!zeroAllowed && value.Sign() == 0) {
		return nil, postingValidationError(field + " must be positive")
	}
	return value, nil
}

func roundToScale(value *big.Rat, scale int) *big.Int {
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	numerator := new(big.Int).Mul(value.Num(), multiplier)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, value.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(value.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient
}

func convertMinorAmount(amount *big.Int, rate *big.Rat) *big.Int {
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(postingMoneyScale), nil)
	value := new(big.Rat).SetFrac(new(big.Int).Set(amount), divisor)
	value.Mul(value, rate)
	return roundToScale(value, postingMoneyScale)
}

func fixedDecimal(value *big.Int, scale int) string {
	digits := value.String()
	if scale == 0 {
		return digits
	}
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale-len(digits)+1) + digits
	}
	return digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]
}

func postingValidationError(detail string) error {
	return fmt.Errorf("VALIDATION_ERROR: %s", detail)
}
