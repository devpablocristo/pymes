package ar

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

type IVARecordDirection string

const (
	IVASale     IVARecordDirection = "sale"
	IVAPurchase IVARecordDirection = "purchase"
)

type IVARecord struct {
	Direction              IVARecordDirection
	Authorized             bool
	IssueDate              string
	VoucherType            VoucherType
	PointOfSale            int
	Number                 int64
	NumberTo               int64
	CounterpartyDocument   ReceiverDocument
	CounterpartyName       string
	Currency               string
	ExchangeRate           fiscal.Decimal
	Total                  fiscal.Decimal
	Untaxed                fiscal.Decimal
	Exempt                 fiscal.Decimal
	VAT                    fiscal.Decimal
	VATPerceptions         fiscal.Decimal
	NationalPerceptions    fiscal.Decimal
	GrossIncomePerceptions fiscal.Decimal
	MunicipalPerceptions   fiscal.Decimal
	InternalTaxes          fiscal.Decimal
	OtherTaxes             fiscal.Decimal
	ComputableVATCredit    fiscal.Decimal
	VATLines               []VATBreakdown
	OperationCode          string
	PaymentDue             string
}

type IVASimpleFiles struct {
	SalesVouchers    []byte
	SalesVAT         []byte
	PurchaseVouchers []byte
	PurchaseVAT      []byte
}

// ExportIVASimple creates the four fixed-width interchange files used by the
// current ARCA electronic VAT registry design. Presentation remains a user
// action in ARCA.
func ExportIVASimple(period string, records []IVARecord) (IVASimpleFiles, error) {
	if _, err := time.Parse("200601", period); err != nil {
		return IVASimpleFiles{}, errors.New("IVA Simple period must use YYYYMM")
	}
	ordered := append([]IVARecord(nil), records...)
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].IssueDate != ordered[right].IssueDate {
			return ordered[left].IssueDate < ordered[right].IssueDate
		}
		if ordered[left].PointOfSale != ordered[right].PointOfSale {
			return ordered[left].PointOfSale < ordered[right].PointOfSale
		}
		return ordered[left].Number < ordered[right].Number
	})

	var sales, salesVAT, purchases, purchasesVAT bytes.Buffer
	purchaseKeys := make(map[string]struct{})
	for index, record := range ordered {
		if err := validateIVARecord(period, record); err != nil {
			return IVASimpleFiles{}, fmt.Errorf("IVA Simple record %d: %w", index, err)
		}
		switch record.Direction {
		case IVASale:
			header, err := salesHeader(record)
			if err != nil {
				return IVASimpleFiles{}, err
			}
			appendRegistryLine(&sales, header)
			for _, line := range record.VATLines {
				detail, err := salesVATLine(record, line)
				if err != nil {
					return IVASimpleFiles{}, err
				}
				appendRegistryLine(&salesVAT, detail)
			}
		case IVAPurchase:
			key := strings.Join([]string{
				record.CounterpartyDocument.Number,
				strconv.Itoa(int(record.VoucherType)),
				strconv.Itoa(record.PointOfSale),
				strconv.FormatInt(record.Number, 10),
			}, ":")
			if _, duplicate := purchaseKeys[key]; duplicate {
				return IVASimpleFiles{}, errors.New("duplicate supplier/type/point/number purchase")
			}
			purchaseKeys[key] = struct{}{}
			header, err := purchaseHeader(record)
			if err != nil {
				return IVASimpleFiles{}, err
			}
			appendRegistryLine(&purchases, header)
			if purchaseDiscriminatesVAT(record.VoucherType) {
				for _, line := range record.VATLines {
					detail, err := purchaseVATLine(record, line)
					if err != nil {
						return IVASimpleFiles{}, err
					}
					appendRegistryLine(&purchasesVAT, detail)
				}
			}
		}
	}
	return IVASimpleFiles{
		SalesVouchers: sales.Bytes(), SalesVAT: salesVAT.Bytes(),
		PurchaseVouchers: purchases.Bytes(), PurchaseVAT: purchasesVAT.Bytes(),
	}, nil
}

func validateIVARecord(period string, record IVARecord) error {
	if record.Direction != IVASale && record.Direction != IVAPurchase {
		return errors.New("direction must be sale or purchase")
	}
	if record.Direction == IVASale && !record.Authorized {
		return errors.New("sales must come from authorized vouchers")
	}
	date, err := parseRegistryDate(record.IssueDate)
	if err != nil || date.Format("200601") != period {
		return errors.New("issue date must belong to the requested period")
	}
	if !record.VoucherType.ValidMVP() || record.PointOfSale <= 0 || record.PointOfSale > 99999 ||
		record.Number <= 0 {
		return errors.New("invalid voucher type, point, or number")
	}
	if record.NumberTo == 0 {
		record.NumberTo = record.Number
	}
	if _, err := NewReceiverDocument(
		record.CounterpartyDocument.Type, record.CounterpartyDocument.Number,
	); err != nil {
		return err
	}
	if strings.TrimSpace(record.CounterpartyName) == "" {
		return errors.New("counterparty name is required")
	}
	currency, err := CurrencyCode(record.Currency)
	if err != nil {
		return err
	}
	if record.ExchangeRate.Cmp(fiscal.Decimal{}) <= 0 ||
		(currency == CurrencyPES && !record.ExchangeRate.Equal(fiscal.NewDecimalFromInt(1))) {
		return errors.New("invalid registry exchange rate")
	}
	for label, amount := range map[string]fiscal.Decimal{
		"total": record.Total, "untaxed": record.Untaxed, "exempt": record.Exempt,
		"vat": record.VAT, "vat_perceptions": record.VATPerceptions,
		"national_perceptions":     record.NationalPerceptions,
		"gross_income_perceptions": record.GrossIncomePerceptions,
		"municipal_perceptions":    record.MunicipalPerceptions,
		"internal_taxes":           record.InternalTaxes, "other_taxes": record.OtherTaxes,
		"computable_vat_credit": record.ComputableVATCredit,
	} {
		if amount.IsNegative() {
			return fmt.Errorf("%s cannot be negative", label)
		}
	}
	vatSum := fiscal.Decimal{}
	for _, line := range record.VATLines {
		if _, valid := VATRateForID(line.ID); !valid ||
			line.BaseAmount.IsNegative() || line.Amount.IsNegative() {
			return errors.New("invalid IVA Simple VAT line")
		}
		vatSum = vatSum.Add(line.Amount)
	}
	if purchaseDiscriminatesVAT(record.VoucherType) || record.Direction == IVASale {
		if !vatSum.Equal(record.VAT) {
			return errors.New("VAT lines do not reconcile with voucher VAT")
		}
	}
	return nil
}

func salesHeader(record IVARecord) (string, error) {
	numberTo := record.NumberTo
	if numberTo == 0 {
		numberTo = record.Number
	}
	currency, _ := CurrencyCode(record.Currency)
	exchange, err := registryAmount(record.ExchangeRate, 10, 6)
	if err != nil {
		return "", err
	}
	operation := registryOperation(record.OperationCode)
	fields := []string{
		registryDate(record.IssueDate),
		leftZero(strconv.Itoa(int(record.VoucherType)), 3),
		leftZero(strconv.Itoa(record.PointOfSale), 5),
		leftZero(strconv.FormatInt(record.Number, 10), 20),
		leftZero(strconv.FormatInt(numberTo, 10), 20),
		leftZero(strconv.Itoa(int(record.CounterpartyDocument.Type)), 2),
		leftZero(record.CounterpartyDocument.Number, 20),
		rightANSI(record.CounterpartyName, 30),
		mustRegistryAmount(record.Total, 15, 2),
		mustRegistryAmount(record.Untaxed, 15, 2),
		zeroAmount15(),
		mustRegistryAmount(record.Exempt, 15, 2),
		mustRegistryAmount(record.NationalPerceptions, 15, 2),
		mustRegistryAmount(record.GrossIncomePerceptions, 15, 2),
		mustRegistryAmount(record.MunicipalPerceptions, 15, 2),
		mustRegistryAmount(record.InternalTaxes, 15, 2),
		rightANSI(currency, 3),
		exchange,
		strconv.Itoa(len(record.VATLines)),
		operation,
		mustRegistryAmount(record.OtherTaxes, 15, 2),
		registryOptionalDate(record.PaymentDue),
	}
	line := strings.Join(fields, "")
	if len([]byte(line)) != 266 {
		return "", fmt.Errorf("sales registry record has %d bytes, want 266", len([]byte(line)))
	}
	return line, nil
}

func salesVATLine(record IVARecord, line VATBreakdown) (string, error) {
	rate, err := VATRegisterRateCode(line.Rate)
	if err != nil {
		return "", err
	}
	value := leftZero(strconv.Itoa(int(record.VoucherType)), 3) +
		leftZero(strconv.Itoa(record.PointOfSale), 5) +
		leftZero(strconv.FormatInt(record.Number, 10), 20) +
		mustRegistryAmount(line.BaseAmount, 15, 2) +
		rate +
		mustRegistryAmount(line.Amount, 15, 2)
	if len(value) != 62 {
		return "", errors.New("sales VAT registry record must contain 62 bytes")
	}
	return value, nil
}

func purchaseHeader(record IVARecord) (string, error) {
	currency, _ := CurrencyCode(record.Currency)
	exchange, err := registryAmount(record.ExchangeRate, 10, 6)
	if err != nil {
		return "", err
	}
	vatCount := len(record.VATLines)
	if !purchaseDiscriminatesVAT(record.VoucherType) {
		vatCount = 0
	}
	fields := []string{
		registryDate(record.IssueDate),
		leftZero(strconv.Itoa(int(record.VoucherType)), 3),
		leftZero(strconv.Itoa(record.PointOfSale), 5),
		leftZero(strconv.FormatInt(record.Number, 10), 20),
		strings.Repeat(" ", 16),
		leftZero(strconv.Itoa(int(record.CounterpartyDocument.Type)), 2),
		leftZero(record.CounterpartyDocument.Number, 20),
		rightANSI(record.CounterpartyName, 30),
		mustRegistryAmount(record.Total, 15, 2),
		mustRegistryAmount(record.Untaxed, 15, 2),
		mustRegistryAmount(record.Exempt, 15, 2),
		mustRegistryAmount(record.VATPerceptions, 15, 2),
		mustRegistryAmount(record.NationalPerceptions, 15, 2),
		mustRegistryAmount(record.GrossIncomePerceptions, 15, 2),
		mustRegistryAmount(record.MunicipalPerceptions, 15, 2),
		mustRegistryAmount(record.InternalTaxes, 15, 2),
		rightANSI(currency, 3),
		exchange,
		strconv.Itoa(vatCount),
		registryOperation(record.OperationCode),
		mustRegistryAmount(record.ComputableVATCredit, 15, 2),
		mustRegistryAmount(record.OtherTaxes, 15, 2),
		strings.Repeat("0", 11),
		strings.Repeat(" ", 30),
		zeroAmount15(),
	}
	line := strings.Join(fields, "")
	if len([]byte(line)) != 325 {
		return "", fmt.Errorf("purchase registry record has %d bytes, want 325", len([]byte(line)))
	}
	return line, nil
}

func purchaseVATLine(record IVARecord, line VATBreakdown) (string, error) {
	rate, err := VATRegisterRateCode(line.Rate)
	if err != nil {
		return "", err
	}
	value := leftZero(strconv.Itoa(int(record.VoucherType)), 3) +
		leftZero(strconv.Itoa(record.PointOfSale), 5) +
		leftZero(strconv.FormatInt(record.Number, 10), 20) +
		leftZero(strconv.Itoa(int(record.CounterpartyDocument.Type)), 2) +
		leftZero(record.CounterpartyDocument.Number, 20) +
		mustRegistryAmount(line.BaseAmount, 15, 2) +
		rate +
		mustRegistryAmount(line.Amount, 15, 2)
	if len(value) != 84 {
		return "", errors.New("purchase VAT registry record must contain 84 bytes")
	}
	return value, nil
}

func purchaseDiscriminatesVAT(voucherType VoucherType) bool {
	switch voucherType {
	case InvoiceA, DebitNoteA, CreditNoteA:
		return true
	default:
		return false
	}
}

func registryAmount(value fiscal.Decimal, width int, scale int32) (string, error) {
	scaled, err := value.ScaledInteger(scale, fiscal.RoundHalfAwayFromZero)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(scaled, "-") {
		return "", errors.New("registry amounts cannot be negative")
	}
	if len(scaled) > width {
		return "", fmt.Errorf("registry amount %s exceeds %d digits", value, width)
	}
	return strings.Repeat("0", width-len(scaled)) + scaled, nil
}

func mustRegistryAmount(value fiscal.Decimal, width int, scale int32) string {
	formatted, _ := registryAmount(value, width, scale)
	return formatted
}

func zeroAmount15() string { return strings.Repeat("0", 15) }

func leftZero(value string, width int) string {
	value = strings.TrimSpace(value)
	if len(value) > width {
		value = value[len(value)-width:]
	}
	return strings.Repeat("0", width-len(value)) + value
}

func rightANSI(value string, width int) string {
	encoded := latin1(value)
	if len(encoded) > width {
		encoded = encoded[:width]
	}
	return string(encoded) + strings.Repeat(" ", width-len(encoded))
}

func latin1(value string) []byte {
	out := make([]byte, 0, len(value))
	for _, character := range value {
		if character >= 0 && character <= 255 {
			out = append(out, byte(character))
		} else {
			out = append(out, '?')
		}
	}
	return out
}

func registryOperation(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 1 {
		return "0"
	}
	return value
}

func parseRegistryDate(value string) (time.Time, error) {
	if len(strings.TrimSpace(value)) == 8 {
		return time.Parse("20060102", value)
	}
	return time.Parse("2006-01-02", value)
}

func registryDate(value string) string {
	date, _ := parseRegistryDate(value)
	return date.Format("20060102")
}

func registryOptionalDate(value string) string {
	if strings.TrimSpace(value) == "" {
		return strings.Repeat("0", 8)
	}
	return registryDate(value)
}

func appendRegistryLine(buffer *bytes.Buffer, value string) {
	buffer.WriteString(value)
	buffer.WriteString("\r\n")
}

type IVAPosition struct {
	DebitVAT     fiscal.Decimal
	CreditVAT    fiscal.Decimal
	Withholdings fiscal.Decimal
	Perceptions  fiscal.Decimal
	CarryForward fiscal.Decimal
	Payable      fiscal.Decimal
}

func CalculateIVAPosition(
	records []IVARecord,
	withholdings, perceptions, carryForward fiscal.Decimal,
) (IVAPosition, error) {
	if withholdings.IsNegative() || perceptions.IsNegative() || carryForward.IsNegative() {
		return IVAPosition{}, errors.New("IVA position offsets cannot be negative")
	}
	position := IVAPosition{
		Withholdings: withholdings, Perceptions: perceptions, CarryForward: carryForward,
	}
	for _, record := range records {
		if record.Direction == IVASale && !record.Authorized {
			continue
		}
		sign := voucherPolarity(record.VoucherType)
		switch record.Direction {
		case IVASale:
			if sign > 0 {
				position.DebitVAT = position.DebitVAT.Add(record.VAT)
			} else {
				position.DebitVAT = position.DebitVAT.Sub(record.VAT)
			}
		case IVAPurchase:
			credit := record.ComputableVATCredit
			if sign > 0 {
				position.CreditVAT = position.CreditVAT.Add(credit)
			} else {
				position.CreditVAT = position.CreditVAT.Sub(credit)
			}
		default:
			return IVAPosition{}, errors.New("invalid IVA position direction")
		}
	}
	position.Payable = position.DebitVAT.Sub(position.CreditVAT).
		Sub(position.Withholdings).Sub(position.Perceptions).Sub(position.CarryForward)
	return position, nil
}

func voucherPolarity(voucherType VoucherType) int {
	switch voucherType {
	case CreditNoteA, CreditNoteB, CreditNoteC:
		return -1
	default:
		return 1
	}
}
