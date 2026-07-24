package fiscalaccounting

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

const roleTaxesPayable = "taxes_payable"

type workItem struct {
	OrganizationID               uuid.UUID
	IntentID                     uuid.UUID
	VoucherID                    uuid.UUID
	SourceType                   string
	SourceID                     uuid.UUID
	Operation                    fiscal.Operation
	SnapshotHash                 string
	AuthorityCode                string
	AttemptCount                 int
	VoucherType                  int
	PointOfSale                  int
	VoucherNumber                int64
	FunctionalCode               string
	CanonicalSnapshot            []byte
	AssociatedOpenItemID         *uuid.UUID
	AssociatedOpenCurrency       *string
	AssociatedOpenCurrencyAmount *string
	AssociatedOpenFunctional     *string
}

type postingSide uint8

const (
	postingDebit postingSide = iota
	postingCredit
)

type vatBucket struct {
	rate   accounting.Decimal
	amount accounting.Decimal
}

type postingOptions struct {
	onCredit              bool
	partyID               *uuid.UUID
	paymentMethod         string
	refundMethod          string
	originalPaymentMethod string
}

func buildPostingPlan(
	item workItem,
	document fiscal.FiscalSnapshot,
	mappings map[string]accounting.AccountMapping,
	actor string,
) (accounting.PostingPlan, error) {
	if item.SourceID == uuid.Nil || !item.Operation.Valid() {
		return accounting.PostingPlan{}, fmt.Errorf("invalid fiscal accounting source")
	}
	issueDate, err := time.Parse("2006-01-02", document.IssueDate)
	if err != nil {
		return accounting.PostingPlan{}, fmt.Errorf("parse fiscal accounting issue date: %w", err)
	}
	rateDate := issueDate
	if strings.TrimSpace(document.Currency.RateDate) != "" {
		rateDate, err = time.Parse("2006-01-02", document.Currency.RateDate)
		if err != nil {
			return accounting.PostingPlan{}, fmt.Errorf("parse fiscal accounting exchange-rate date: %w", err)
		}
	}
	currency, err := accounting.NewCurrency(document.Currency.Code)
	if err != nil {
		return accounting.PostingPlan{}, err
	}
	functionalCurrency, err := accounting.NewCurrency(item.FunctionalCode)
	if err != nil {
		return accounting.PostingPlan{}, err
	}
	exchangeRate, err := accounting.ParseExchangeRate(document.Currency.Rate.String())
	if err != nil {
		return accounting.PostingPlan{}, err
	}

	commercialLines, subtotal, vat, otherTaxes, total, buckets, err :=
		snapshotAmounts(document, currency, functionalCurrency)
	if err != nil {
		return accounting.PostingPlan{}, err
	}
	costConfirmed := false
	for _, line := range document.Lines {
		costConfirmed = costConfirmed || line.CostConfirmed
	}
	options, err := parsePostingOptions(document.Metadata)
	if err != nil {
		return accounting.PostingPlan{}, err
	}
	if options.onCredit && options.partyID == nil {
		return accounting.PostingPlan{}, fmt.Errorf(
			"credit fiscal document requires accounting.party_id metadata",
		)
	}

	number := fmt.Sprintf("%05d-%08d", item.PointOfSale, item.VoucherNumber)
	idempotencyKey := "fiscal-accounting:" + item.IntentID.String()
	engine := accounting.NewPostingEngine(mappings)
	baseTotal := subtotal.Add(vat)
	var plan accounting.PostingPlan
	switch item.Operation {
	case fiscal.OperationInvoice, fiscal.OperationDebitNote:
		event := accounting.SaleEvent{
			ID:                 item.SourceID,
			Number:             number,
			Date:               issueDate,
			Currency:           currency,
			FunctionalCurrency: functionalCurrency,
			ExchangeRate:       exchangeRate,
			ExchangeRateDate:   rateDate,
			ExchangeRateSource: document.Currency.RateSource,
			OnCredit:           options.onCredit,
			PaymentMethod:      options.paymentMethod,
			PartyID:            options.partyID,
			Subtotal:           subtotal,
			TaxTotal:           vat,
			Total:              baseTotal,
			Lines:              commercialLines,
			CostConfirmed:      costConfirmed,
			Actor:              actor,
			IdempotencyKey:     idempotencyKey,
		}
		if item.Operation == fiscal.OperationDebitNote {
			plan, err = engine.BuildCustomerDebitNote(event)
		} else {
			plan, err = engine.BuildSale(event)
		}
	case fiscal.OperationCreditNote:
		plan, err = engine.BuildReturn(accounting.ReturnEvent{
			ID:                    item.SourceID,
			Number:                number,
			Date:                  issueDate,
			Currency:              currency,
			FunctionalCurrency:    functionalCurrency,
			ExchangeRate:          exchangeRate,
			ExchangeRateDate:      rateDate,
			ExchangeRateSource:    document.Currency.RateSource,
			RefundMethod:          options.refundMethod,
			OriginalPaymentMethod: options.originalPaymentMethod,
			PartyID:               options.partyID,
			Subtotal:              subtotal,
			TaxTotal:              vat,
			Total:                 baseTotal,
			Lines:                 commercialLines,
			CostConfirmed:         costConfirmed,
			Actor:                 actor,
			IdempotencyKey:        idempotencyKey,
		})
	default:
		return accounting.PostingPlan{}, fmt.Errorf(
			"unsupported fiscal accounting operation %q",
			item.Operation,
		)
	}
	if err != nil {
		return accounting.PostingPlan{}, err
	}

	plan.Entry.Source.Type = strings.TrimSpace(item.SourceType)
	plan.Entry.Source.Event = "fiscal." + string(item.Operation) + ".authorized"
	plan.Entry.Source.IdempotencyKey = idempotencyKey
	plan.Entry.Description = fiscalDescription(item.Operation, number)
	costLines := fiscalCostLines(plan.Entry.Lines, mappings)
	plan.Entry.Lines, err = exactFiscalLines(
		plan.Entry,
		mappings,
		options,
		subtotal,
		vat,
		otherTaxes,
		total,
		buckets,
	)
	if err != nil {
		return accounting.PostingPlan{}, err
	}
	plan.Entry.Lines = append(plan.Entry.Lines, costLines...)
	for index := range plan.Entry.Lines {
		plan.Entry.Lines[index].LineNo = index + 1
	}
	dueDate := issueDate
	dueDateRaw := metadataValue(
		document.Metadata,
		"accounting.due_date",
	)
	if dueDateRaw == "" {
		dueDateRaw = strings.TrimSpace(document.PaymentDue)
	}
	if dueDateRaw != "" {
		dueDate, err = time.Parse("2006-01-02", dueDateRaw)
		if err != nil || dueDate.Before(issueDate) {
			return accounting.PostingPlan{}, fmt.Errorf(
				"invalid fiscal accounting payment due date",
			)
		}
	}
	for index := range plan.OpenItems {
		plan.OpenItems[index].SourceType = plan.Entry.Source.Type
		plan.OpenItems[index].DueDate = dueDate
		plan.OpenItems[index].OriginalAmount = total
		plan.OpenItems[index].OpenAmount = total
		functional := convert(total, exchangeRate, functionalCurrency)
		plan.OpenItems[index].FunctionalAmount = functional
		plan.OpenItems[index].OpenFunctional = functional
	}
	if item.Operation == fiscal.OperationCreditNote &&
		strings.EqualFold(strings.TrimSpace(options.refundMethod), "credit_note") {
		if err := applyCustomerCreditNote(
			&plan,
			item,
			total,
			functionalCurrency,
			mappings,
			issueDate,
			actor,
		); err != nil {
			return accounting.PostingPlan{}, err
		}
	}
	if err := plan.Entry.ValidateForPosting(); err != nil {
		return accounting.PostingPlan{}, err
	}
	return plan, nil
}

func fiscalCostLines(
	lines []accounting.JournalLine,
	mappings map[string]accounting.AccountMapping,
) []accounting.JournalLine {
	accountIDs := make(map[uuid.UUID]struct{}, 2)
	for _, role := range []string{accounting.RoleInventory, accounting.RoleCOGS} {
		if mapping, ok := mappings[role]; ok && mapping.AccountID != uuid.Nil {
			accountIDs[mapping.AccountID] = struct{}{}
		}
	}
	result := make([]accounting.JournalLine, 0, 2)
	for _, line := range lines {
		if _, ok := accountIDs[line.AccountID]; ok {
			result = append(result, line)
		}
	}
	return result
}

func applyCustomerCreditNote(
	plan *accounting.PostingPlan,
	item workItem,
	amount accounting.Decimal,
	functionalCurrency accounting.Currency,
	mappings map[string]accounting.AccountMapping,
	appliedAt time.Time,
	actor string,
) error {
	if plan == nil ||
		item.AssociatedOpenItemID == nil ||
		*item.AssociatedOpenItemID == uuid.Nil ||
		item.AssociatedOpenCurrency == nil ||
		item.AssociatedOpenCurrencyAmount == nil ||
		item.AssociatedOpenFunctional == nil ||
		!strings.EqualFold(
			strings.TrimSpace(*item.AssociatedOpenCurrency),
			plan.Entry.Currency.Code(),
		) {
		return fmt.Errorf(
			"credit note cannot find a matching associated receivable balance",
		)
	}
	remainingCurrency, err := accounting.ParseAmount(
		*item.AssociatedOpenCurrencyAmount,
	)
	if err != nil {
		return fmt.Errorf("parse associated receivable currency balance: %w", err)
	}
	remainingFunctional, err := accounting.ParseAmount(
		*item.AssociatedOpenFunctional,
	)
	if err != nil {
		return fmt.Errorf("parse associated receivable functional balance: %w", err)
	}
	if amount.Sign() <= 0 ||
		remainingCurrency.Sign() <= 0 ||
		remainingFunctional.Sign() <= 0 ||
		amount.Cmp(remainingCurrency) > 0 {
		return fmt.Errorf("credit note exceeds the associated receivable balance")
	}
	bookFunctional := remainingFunctional
	if !amount.Equal(remainingCurrency) {
		proportional, err := remainingFunctional.
			Mul(amount).
			Quo(remainingCurrency, 12)
		if err != nil {
			return err
		}
		bookFunctional = functionalCurrency.Round(proportional)
	}
	receivableAccount, err := mappedAccount(mappings, accounting.RoleReceivable)
	if err != nil {
		return err
	}
	controlIndex := -1
	currentFunctional := accounting.Zero
	for index := range plan.Entry.Lines {
		line := &plan.Entry.Lines[index]
		if line.AccountID == receivableAccount && line.PartyID != nil {
			controlIndex = index
			currentFunctional = line.Credit
			break
		}
	}
	if controlIndex < 0 || currentFunctional.Sign() <= 0 {
		return fmt.Errorf("credit note receivable control line is missing")
	}
	control := &plan.Entry.Lines[controlIndex]
	control.Credit = bookFunctional
	control.TransactionCredit = bookFunctional
	control.Currency = functionalCurrency
	control.ExchangeRate = accounting.One
	control.ExchangeRateDate = appliedAt
	control.ExchangeRateSource = "functional_currency"
	difference := currentFunctional.Sub(bookFunctional)
	if !difference.IsZero() {
		role := accounting.RoleFXLoss
		debit, credit := difference.Abs(), accounting.Zero
		if difference.Sign() > 0 {
			role = accounting.RoleFXGain
			debit, credit = accounting.Zero, difference
		}
		accountID, err := mappedAccount(mappings, role)
		if err != nil {
			return err
		}
		plan.Entry.Lines = append(plan.Entry.Lines, accounting.JournalLine{
			ID:                 uuid.New(),
			AccountID:          accountID,
			Debit:              debit,
			Credit:             credit,
			TransactionDebit:   debit,
			TransactionCredit:  credit,
			Currency:           functionalCurrency,
			ExchangeRate:       accounting.One,
			ExchangeRateDate:   appliedAt,
			ExchangeRateSource: "functional_currency",
			Memo:               "Diferencia de cambio por nota de crédito de cliente",
		})
	}
	for index := range plan.Entry.Lines {
		plan.Entry.Lines[index].LineNo = index + 1
	}
	plan.Applications = []accounting.OpenItemApplication{{
		ID:                 uuid.New(),
		OpenItemID:         *item.AssociatedOpenItemID,
		AppliedAt:          appliedAt,
		Amount:             amount,
		FunctionalAmount:   bookFunctional,
		ExchangeDifference: difference,
		CreatedBy:          actor,
	}}
	return plan.Entry.ValidateForPosting()
}

func snapshotAmounts(
	document fiscal.FiscalSnapshot,
	currency accounting.Currency,
	functionalCurrency accounting.Currency,
) (
	[]accounting.CommercialLine,
	accounting.Decimal,
	accounting.Decimal,
	accounting.Decimal,
	accounting.Decimal,
	[]vatBucket,
	error,
) {
	netTaxed, err := parseSnapshotAmount("net_taxed", document.Totals.NetTaxed)
	if err != nil {
		return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil, err
	}
	netUntaxed, err := parseSnapshotAmount("net_untaxed", document.Totals.NetUntaxed)
	if err != nil {
		return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil, err
	}
	exempt, err := parseSnapshotAmount("exempt", document.Totals.Exempt)
	if err != nil {
		return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil, err
	}
	vat, err := parseSnapshotAmount("vat", document.Totals.VAT)
	if err != nil {
		return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil, err
	}
	otherTaxes, err := parseSnapshotAmount("other_taxes", document.Totals.OtherTaxes)
	if err != nil {
		return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil, err
	}
	total, err := parseSnapshotAmount("total", document.Totals.Total)
	if err != nil {
		return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil, err
	}
	subtotal := netTaxed.Add(netUntaxed).Add(exempt)
	for label, amount := range map[string]accounting.Decimal{
		"subtotal": subtotal,
		"vat":      vat,
		"taxes":    otherTaxes,
		"total":    total,
	} {
		if amount.Sign() < 0 || !amount.Equal(currency.Round(amount)) {
			return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil,
				fmt.Errorf("fiscal accounting %s does not use %s minor units", label, currency.Code())
		}
	}
	if total.Sign() <= 0 ||
		!currency.Round(subtotal.Add(vat).Add(otherTaxes)).Equal(total) {
		return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil,
			fmt.Errorf("fiscal accounting snapshot totals do not reconcile")
	}

	lines := make([]accounting.CommercialLine, 0, len(document.Lines))
	bucketsByRate := make(map[string]vatBucket)
	var lineSubtotal, lineVAT accounting.Decimal
	for index, line := range document.Lines {
		basis, parseErr := snapshotLineBasis(line)
		if parseErr != nil {
			return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil,
				fmt.Errorf("fiscal accounting line %d: %w", index+1, parseErr)
		}
		taxRate, parseErr := accounting.ParseAmount(line.TaxRate.String())
		if parseErr != nil || taxRate.Sign() < 0 {
			return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil,
				fmt.Errorf("fiscal accounting line %d has an invalid tax rate", index+1)
		}
		taxAmount, parseErr := parseSnapshotAmount("line tax", line.TaxAmount)
		if parseErr != nil || taxAmount.Sign() < 0 ||
			!taxAmount.Equal(currency.Round(taxAmount)) {
			return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil,
				fmt.Errorf("fiscal accounting line %d has an invalid tax amount", index+1)
		}
		if basis.Sign() < 0 || !basis.Equal(currency.Round(basis)) {
			return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil,
				fmt.Errorf("fiscal accounting line %d has an invalid basis", index+1)
		}
		if taxAmount.Sign() > 0 && taxRate.IsZero() {
			return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil,
				fmt.Errorf("fiscal accounting line %d has tax without a rate", index+1)
		}
		cost, parseErr := parseSnapshotAmount("line cost", line.CostAmount)
		if parseErr != nil || cost.Sign() < 0 ||
			!cost.Equal(functionalCurrency.Round(cost)) {
			return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil,
				fmt.Errorf("fiscal accounting line %d has an invalid confirmed cost", index+1)
		}
		if !line.CostConfirmed && !cost.IsZero() {
			return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil,
				fmt.Errorf("fiscal accounting line %d has an unconfirmed cost", index+1)
		}
		lines = append(lines, accounting.CommercialLine{
			NetAmount: basis,
			TaxRate:   taxRate,
			Cost:      cost,
		})
		lineSubtotal = lineSubtotal.Add(basis)
		lineVAT = lineVAT.Add(taxAmount)
		if taxAmount.Sign() > 0 {
			key := taxRate.String()
			bucket := bucketsByRate[key]
			bucket.rate = taxRate
			bucket.amount = bucket.amount.Add(taxAmount)
			bucketsByRate[key] = bucket
		}
	}
	if !currency.Round(lineSubtotal).Equal(subtotal) ||
		!currency.Round(lineVAT).Equal(vat) {
		return nil, accounting.Zero, accounting.Zero, accounting.Zero, accounting.Zero, nil,
			fmt.Errorf("fiscal accounting lines do not reconcile with snapshot totals")
	}
	buckets := make([]vatBucket, 0, len(bucketsByRate))
	for _, bucket := range bucketsByRate {
		buckets = append(buckets, bucket)
	}
	sort.Slice(buckets, func(left, right int) bool {
		return buckets[left].rate.Cmp(buckets[right].rate) < 0
	})
	return lines, subtotal, vat, otherTaxes, total, buckets, nil
}

func snapshotLineBasis(line fiscal.FiscalLineSnapshot) (accounting.Decimal, error) {
	basis := line.NetAmount
	if basis.IsZero() {
		switch {
		case !line.ExemptAmount.IsZero():
			basis = line.ExemptAmount
		case !line.UntaxedAmount.IsZero():
			basis = line.UntaxedAmount
		}
	}
	return parseSnapshotAmount("line basis", basis)
}

func parseSnapshotAmount(label string, value fiscal.Decimal) (accounting.Decimal, error) {
	parsed, err := accounting.ParseAmount(value.String())
	if err != nil {
		return accounting.Zero, fmt.Errorf("parse fiscal accounting %s: %w", label, err)
	}
	return parsed, nil
}

func parsePostingOptions(metadata map[string]string) (postingOptions, error) {
	options := postingOptions{
		paymentMethod:         metadataValue(metadata, "accounting.payment_method", "payment_method"),
		originalPaymentMethod: metadataValue(metadata, "accounting.original_payment_method", "original_payment_method"),
		refundMethod:          metadataValue(metadata, "accounting.refund_method", "refund_method"),
	}
	if options.paymentMethod == "" {
		options.paymentMethod = "cash"
	}
	if options.originalPaymentMethod == "" {
		options.originalPaymentMethod = options.paymentMethod
	}
	if raw := metadataValue(metadata, "accounting.party_id", "party_id"); raw != "" {
		partyID, err := uuid.Parse(raw)
		if err != nil || partyID == uuid.Nil {
			return postingOptions{}, fmt.Errorf("accounting.party_id metadata must be a UUID")
		}
		options.partyID = &partyID
	}
	options.onCredit = options.partyID != nil
	if raw := metadataValue(metadata, "accounting.on_credit", "on_credit"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return postingOptions{}, fmt.Errorf("accounting.on_credit metadata must be boolean")
		}
		options.onCredit = value
	}
	if options.refundMethod == "" {
		if options.partyID != nil {
			options.refundMethod = "credit_note"
		} else {
			options.refundMethod = options.originalPaymentMethod
		}
	}
	return options, nil
}

func metadataValue(metadata map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return value
		}
	}
	return ""
}

func exactFiscalLines(
	entry accounting.JournalEntry,
	mappings map[string]accounting.AccountMapping,
	options postingOptions,
	subtotal accounting.Decimal,
	_ accounting.Decimal,
	otherTaxes accounting.Decimal,
	total accounting.Decimal,
	buckets []vatBucket,
) ([]accounting.JournalLine, error) {
	revenueAccount, err := mappedAccount(mappings, accounting.RoleRevenue)
	if err != nil {
		return nil, err
	}
	controlRole := paymentRole(options.paymentMethod)
	controlParty := (*uuid.UUID)(nil)
	revenueSide, taxSide, controlSide := postingCredit, postingCredit, postingDebit
	if entry.Kind == accounting.EntryRefund {
		revenueSide, taxSide, controlSide = postingDebit, postingDebit, postingCredit
		if strings.EqualFold(strings.TrimSpace(options.refundMethod), "credit_note") {
			controlRole = accounting.RoleReceivable
			controlParty = options.partyID
		} else {
			controlRole = paymentRole(options.originalPaymentMethod)
		}
	} else if options.onCredit {
		controlRole = accounting.RoleReceivable
		controlParty = options.partyID
	}
	controlAccount, err := mappedAccount(mappings, controlRole)
	if err != nil {
		return nil, err
	}

	lines := make([]accounting.JournalLine, 0, 4+len(buckets))
	control := transactionLine(
		entry,
		controlAccount,
		controlSide,
		total,
		controlParty,
		entry.Description,
	)
	revenue := transactionLine(
		entry,
		revenueAccount,
		revenueSide,
		subtotal,
		nil,
		entry.Description,
	)
	if controlSide == postingDebit {
		lines = append(lines, control)
		if subtotal.Sign() > 0 {
			lines = append(lines, revenue)
		}
	} else if subtotal.Sign() > 0 {
		lines = append(lines, revenue)
	}
	for _, bucket := range buckets {
		accountID, mappingErr := mappedAccount(
			mappings,
			"vat_payable_"+rateMappingKey(bucket.rate),
		)
		if mappingErr != nil {
			return nil, mappingErr
		}
		lines = append(lines, transactionLine(
			entry,
			accountID,
			taxSide,
			bucket.amount,
			nil,
			"IVA "+bucket.rate.String()+"%",
		))
	}
	if otherTaxes.Sign() > 0 {
		accountID, mappingErr := mappedAccount(mappings, roleTaxesPayable)
		if mappingErr != nil {
			return nil, mappingErr
		}
		lines = append(lines, transactionLine(
			entry,
			accountID,
			taxSide,
			otherTaxes,
			nil,
			"Otros tributos",
		))
	}
	if controlSide == postingCredit {
		lines = append(lines, control)
	}
	lines, err = balanceFunctionalRounding(entry, mappings, lines)
	if err != nil {
		return nil, err
	}
	for index := range lines {
		lines[index].LineNo = index + 1
	}
	return lines, nil
}

func transactionLine(
	entry accounting.JournalEntry,
	accountID uuid.UUID,
	side postingSide,
	amount accounting.Decimal,
	partyID *uuid.UUID,
	memo string,
) accounting.JournalLine {
	functional := convert(amount, entry.ExchangeRate, entry.FunctionalCurrency)
	line := accounting.JournalLine{
		AccountID:          accountID,
		Currency:           entry.Currency,
		ExchangeRate:       entry.ExchangeRate,
		ExchangeRateDate:   entry.ExchangeRateDate,
		ExchangeRateSource: entry.ExchangeRateSource,
		PartyID:            partyID,
		Memo:               memo,
	}
	if side == postingDebit {
		line.Debit = functional
		line.TransactionDebit = amount
	} else {
		line.Credit = functional
		line.TransactionCredit = amount
	}
	return line
}

func balanceFunctionalRounding(
	entry accounting.JournalEntry,
	mappings map[string]accounting.AccountMapping,
	lines []accounting.JournalLine,
) ([]accounting.JournalLine, error) {
	var debit, credit accounting.Decimal
	for _, line := range lines {
		debit = debit.Add(line.Debit)
		credit = credit.Add(line.Credit)
	}
	difference := entry.FunctionalCurrency.Round(debit.Sub(credit))
	if difference.IsZero() {
		return lines, nil
	}
	accountID, err := mappedAccount(mappings, accounting.RoleRoundingDifference)
	if err != nil {
		return nil, err
	}
	side := postingCredit
	if difference.Sign() < 0 {
		side = postingDebit
	}
	line := accounting.JournalLine{
		AccountID:    accountID,
		Currency:     entry.FunctionalCurrency,
		ExchangeRate: accounting.One,
		Memo:         "Diferencia de redondeo",
	}
	if side == postingDebit {
		line.Debit = difference.Abs()
		line.TransactionDebit = difference.Abs()
	} else {
		line.Credit = difference.Abs()
		line.TransactionCredit = difference.Abs()
	}
	return append(lines, line), nil
}

func convert(
	amount accounting.Decimal,
	exchangeRate accounting.Decimal,
	functionalCurrency accounting.Currency,
) accounting.Decimal {
	return functionalCurrency.Round(amount.Mul(exchangeRate))
}

func mappedAccount(
	mappings map[string]accounting.AccountMapping,
	role string,
) (uuid.UUID, error) {
	mapping, ok := mappings[role]
	if !ok || mapping.AccountID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: %s", accounting.ErrMappingMissing, role)
	}
	return mapping.AccountID, nil
}

func paymentRole(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "transfer", "bank", "bank_transfer":
		return accounting.RoleBank
	case "card", "credit_card", "debit_card":
		return accounting.RoleCardClearing
	case "wallet", "mercadopago", "mp":
		return accounting.RoleWalletClearing
	case "check", "cheque":
		return accounting.RoleChecksClearing
	default:
		return accounting.RoleCash
	}
}

func rateMappingKey(rate accounting.Decimal) string {
	return strings.ReplaceAll(rate.String(), ".", "")
}

func fiscalDescription(operation fiscal.Operation, number string) string {
	switch operation {
	case fiscal.OperationInvoice:
		return "Factura fiscal " + number
	case fiscal.OperationCreditNote:
		return "Nota de crédito fiscal " + number
	case fiscal.OperationDebitNote:
		return "Nota de débito fiscal " + number
	default:
		return "Comprobante fiscal " + number
	}
}
