package accounting

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	RoleCash               = "cash"
	RoleBank               = "bank"
	RoleCardClearing       = "card_clearing"
	RoleWalletClearing     = "wallet_clearing"
	RoleChecksClearing     = "checks_clearing"
	RoleReceivable         = "receivable"
	RolePayable            = "payable"
	RoleRevenue            = "revenue"
	RoleInventory          = "inventory"
	RoleCOGS               = "cogs"
	RolePurchaseExpense    = "purchase_expense"
	RoleCreditNotePayable  = "credit_note_payable"
	RoleFXGain             = "fx_gain"
	RoleFXLoss             = "fx_loss"
	RoleRoundingDifference = "rounding_difference"
	RoleRECPAM             = "recpam"
	RoleCurrentResult      = "current_result"
)

type CommercialLine struct {
	NetAmount Decimal `json:"net_amount"`
	TaxRate   Decimal `json:"tax_rate"`
	Cost      Decimal `json:"cost"`
}

type SaleEvent struct {
	ID                 uuid.UUID        `json:"id"`
	Number             string           `json:"number"`
	Date               time.Time        `json:"date"`
	Currency           Currency         `json:"currency"`
	FunctionalCurrency Currency         `json:"functional_currency"`
	ExchangeRate       Decimal          `json:"exchange_rate"`
	ExchangeRateDate   time.Time        `json:"exchange_rate_date"`
	ExchangeRateSource string           `json:"exchange_rate_source"`
	OnCredit           bool             `json:"on_credit"`
	PaymentMethod      string           `json:"payment_method"`
	PartyID            *uuid.UUID       `json:"party_id,omitempty"`
	Subtotal           Decimal          `json:"subtotal"`
	TaxTotal           Decimal          `json:"tax_total"`
	Total              Decimal          `json:"total"`
	Lines              []CommercialLine `json:"lines"`
	CostConfirmed      bool             `json:"cost_confirmed"`
	Actor              string           `json:"actor"`
	IdempotencyKey     string           `json:"-"`
}

type PurchaseLine struct {
	NetAmount   Decimal `json:"net_amount"`
	TaxRate     Decimal `json:"tax_rate"`
	IsInventory bool    `json:"is_inventory"`
}

type PurchaseEvent struct {
	ID                 uuid.UUID      `json:"id"`
	Number             string         `json:"number"`
	Date               time.Time      `json:"date"`
	DueDate            time.Time      `json:"due_date"`
	Currency           Currency       `json:"currency"`
	FunctionalCurrency Currency       `json:"functional_currency"`
	ExchangeRate       Decimal        `json:"exchange_rate"`
	ExchangeRateDate   time.Time      `json:"exchange_rate_date"`
	ExchangeRateSource string         `json:"exchange_rate_source"`
	PartyID            *uuid.UUID     `json:"party_id,omitempty"`
	Subtotal           Decimal        `json:"subtotal"`
	TaxTotal           Decimal        `json:"tax_total"`
	Total              Decimal        `json:"total"`
	Lines              []PurchaseLine `json:"lines"`
	Actor              string         `json:"actor"`
	IdempotencyKey     string         `json:"-"`
}

type ReceiptEvent struct {
	ID                   uuid.UUID  `json:"id"`
	OpenItemID           uuid.UUID  `json:"open_item_id"`
	PartyID              *uuid.UUID `json:"party_id,omitempty"`
	Date                 time.Time  `json:"date"`
	Currency             Currency   `json:"currency"`
	FunctionalCurrency   Currency   `json:"functional_currency"`
	ExchangeRate         Decimal    `json:"exchange_rate"`
	ExchangeRateDate     time.Time  `json:"exchange_rate_date"`
	ExchangeRateSource   string     `json:"exchange_rate_source"`
	PaymentMethod        string     `json:"payment_method"`
	Amount               Decimal    `json:"amount"`
	BookFunctionalAmount Decimal    `json:"book_functional_amount"`
	Actor                string     `json:"actor"`
	IdempotencyKey       string     `json:"-"`
}

type SupplierPaymentEvent ReceiptEvent

type ReturnEvent struct {
	ID                    uuid.UUID        `json:"id"`
	SaleID                uuid.UUID        `json:"sale_id"`
	Number                string           `json:"number"`
	Date                  time.Time        `json:"date"`
	Currency              Currency         `json:"currency"`
	FunctionalCurrency    Currency         `json:"functional_currency"`
	ExchangeRate          Decimal          `json:"exchange_rate"`
	ExchangeRateDate      time.Time        `json:"exchange_rate_date"`
	ExchangeRateSource    string           `json:"exchange_rate_source"`
	RefundMethod          string           `json:"refund_method"`
	OriginalPaymentMethod string           `json:"original_payment_method"`
	PartyID               *uuid.UUID       `json:"party_id,omitempty"`
	Subtotal              Decimal          `json:"subtotal"`
	TaxTotal              Decimal          `json:"tax_total"`
	Total                 Decimal          `json:"total"`
	Lines                 []CommercialLine `json:"lines"`
	CostConfirmed         bool             `json:"cost_confirmed"`
	Actor                 string           `json:"actor"`
	IdempotencyKey        string           `json:"-"`
}

type PostingPlan struct {
	Entry        JournalEntry          `json:"entry"`
	OpenItems    []OpenItem            `json:"open_items,omitempty"`
	Applications []OpenItemApplication `json:"applications,omitempty"`
}

type PostingEngine struct {
	mappings map[string]AccountMapping
}

func NewPostingEngine(mappings map[string]AccountMapping) *PostingEngine {
	copyMappings := make(map[string]AccountMapping, len(mappings))
	for role, mapping := range mappings {
		copyMappings[strings.TrimSpace(role)] = mapping
	}
	return &PostingEngine{mappings: copyMappings}
}

func (e *PostingEngine) BuildSale(event SaleEvent) (PostingPlan, error) {
	if err := validateDocument(
		event.ID,
		event.Number,
		event.Date,
		event.Currency,
		event.FunctionalCurrency,
		event.ExchangeRate,
		event.Subtotal,
		event.TaxTotal,
		event.Total,
		event.Actor,
		event.IdempotencyKey,
	); err != nil {
		return PostingPlan{}, err
	}
	if event.OnCredit && (event.PartyID == nil || *event.PartyID == uuid.Nil) {
		return PostingPlan{}, fmt.Errorf("%w: credit sale requires a party", ErrInvalidArgument)
	}
	if err := validateCommercialSubtotal(event.Lines, event.Subtotal, event.Currency); err != nil {
		return PostingPlan{}, err
	}

	debitRole := paymentMethodRole(event.PaymentMethod)
	var partyID *uuid.UUID
	if event.OnCredit {
		debitRole = RoleReceivable
		partyID = event.PartyID
	}
	debitAccount, err := e.account(debitRole)
	if err != nil {
		return PostingPlan{}, err
	}
	revenueAccount, err := e.account(RoleRevenue)
	if err != nil {
		return PostingPlan{}, err
	}

	entry := baseEntry(
		event.ID,
		"sale",
		"sale.completed",
		"Venta "+event.Number,
		event.Date,
		event.Currency,
		event.FunctionalCurrency,
		event.ExchangeRate,
		event.ExchangeRateDate,
		event.ExchangeRateSource,
		event.Actor,
		event.IdempotencyKey,
	)
	entry.Lines = append(entry.Lines,
		e.transactionLine(debitAccount, debitSide, event.Total, partyID, "Venta "+event.Number, entry),
		e.transactionLine(revenueAccount, creditSide, event.Subtotal, nil, "Ventas "+event.Number, entry),
	)
	taxLines, err := e.taxLines(event.Lines, event.TaxTotal, creditSide, "vat_payable_", "IVA venta "+event.Number, entry)
	if err != nil {
		return PostingPlan{}, err
	}
	entry.Lines = append(entry.Lines, taxLines...)

	if event.CostConfirmed {
		var cost Decimal
		for _, line := range event.Lines {
			if line.Cost.Sign() < 0 {
				return PostingPlan{}, fmt.Errorf("%w: sale cost cannot be negative", ErrInvalidArgument)
			}
			cost = cost.Add(line.Cost)
		}
		cost = event.FunctionalCurrency.Round(cost)
		if !cost.IsZero() {
			cogsAccount, accountErr := e.account(RoleCOGS)
			if accountErr != nil {
				return PostingPlan{}, accountErr
			}
			inventoryAccount, accountErr := e.account(RoleInventory)
			if accountErr != nil {
				return PostingPlan{}, accountErr
			}
			entry.Lines = append(entry.Lines,
				functionalLine(cogsAccount, debitSide, cost, event.FunctionalCurrency, nil, "CMV venta "+event.Number),
				functionalLine(inventoryAccount, creditSide, cost, event.FunctionalCurrency, nil, "Salida inventario "+event.Number),
			)
		}
	}
	if err := e.finishEntry(&entry); err != nil {
		return PostingPlan{}, err
	}

	plan := PostingPlan{Entry: entry}
	if event.OnCredit {
		dueDate := event.Date
		plan.OpenItems = []OpenItem{{
			ID:               uuid.New(),
			Kind:             Receivable,
			PartyID:          *event.PartyID,
			AccountID:        debitAccount,
			SourceType:       entry.Source.Type,
			SourceID:         event.ID,
			IssueDate:        event.Date,
			DueDate:          dueDate,
			Currency:         event.Currency,
			OriginalAmount:   event.Total,
			FunctionalAmount: convert(event.Total, event.ExchangeRate, event.FunctionalCurrency),
			OpenAmount:       event.Total,
			OpenFunctional:   convert(event.Total, event.ExchangeRate, event.FunctionalCurrency),
		}}
	}
	return plan, nil
}

func (e *PostingEngine) BuildPurchase(event PurchaseEvent) (PostingPlan, error) {
	if err := validateDocument(
		event.ID,
		event.Number,
		event.Date,
		event.Currency,
		event.FunctionalCurrency,
		event.ExchangeRate,
		event.Subtotal,
		event.TaxTotal,
		event.Total,
		event.Actor,
		event.IdempotencyKey,
	); err != nil {
		return PostingPlan{}, err
	}
	if event.PartyID == nil || *event.PartyID == uuid.Nil {
		return PostingPlan{}, fmt.Errorf("%w: purchase requires a supplier party", ErrInvalidArgument)
	}
	var inventoryNet, expenseNet Decimal
	taxBasis := make([]CommercialLine, 0, len(event.Lines))
	for _, line := range event.Lines {
		if line.NetAmount.Sign() < 0 || line.TaxRate.Sign() < 0 {
			return PostingPlan{}, fmt.Errorf("%w: purchase lines cannot be negative", ErrInvalidArgument)
		}
		if line.IsInventory {
			inventoryNet = inventoryNet.Add(line.NetAmount)
		} else {
			expenseNet = expenseNet.Add(line.NetAmount)
		}
		taxBasis = append(taxBasis, CommercialLine{NetAmount: line.NetAmount, TaxRate: line.TaxRate})
	}
	if !event.Currency.Round(inventoryNet.Add(expenseNet)).Equal(event.Currency.Round(event.Subtotal)) {
		return PostingPlan{}, fmt.Errorf("%w: purchase lines do not reconcile with subtotal", ErrUnbalancedEntry)
	}

	entry := baseEntry(
		event.ID,
		"purchase",
		"purchase.received",
		"Compra "+event.Number,
		event.Date,
		event.Currency,
		event.FunctionalCurrency,
		event.ExchangeRate,
		event.ExchangeRateDate,
		event.ExchangeRateSource,
		event.Actor,
		event.IdempotencyKey,
	)
	if !inventoryNet.IsZero() {
		account, err := e.account(RoleInventory)
		if err != nil {
			return PostingPlan{}, err
		}
		entry.Lines = append(entry.Lines, e.transactionLine(account, debitSide, inventoryNet, nil, "Mercadería compra "+event.Number, entry))
	}
	if !expenseNet.IsZero() {
		account, err := e.account(RolePurchaseExpense)
		if err != nil {
			return PostingPlan{}, err
		}
		entry.Lines = append(entry.Lines, e.transactionLine(account, debitSide, expenseNet, nil, "Servicios compra "+event.Number, entry))
	}
	taxLines, err := e.taxLines(taxBasis, event.TaxTotal, debitSide, "vat_credit_", "IVA compra "+event.Number, entry)
	if err != nil {
		return PostingPlan{}, err
	}
	entry.Lines = append(entry.Lines, taxLines...)
	payableAccount, err := e.account(RolePayable)
	if err != nil {
		return PostingPlan{}, err
	}
	entry.Lines = append(entry.Lines, e.transactionLine(payableAccount, creditSide, event.Total, event.PartyID, "Proveedor "+event.Number, entry))
	if err := e.finishEntry(&entry); err != nil {
		return PostingPlan{}, err
	}
	dueDate := event.DueDate
	if dueDate.IsZero() {
		dueDate = event.Date
	}
	openItem := OpenItem{
		ID:               uuid.New(),
		Kind:             Payable,
		PartyID:          *event.PartyID,
		AccountID:        payableAccount,
		SourceType:       entry.Source.Type,
		SourceID:         event.ID,
		IssueDate:        event.Date,
		DueDate:          dueDate,
		Currency:         event.Currency,
		OriginalAmount:   event.Total,
		FunctionalAmount: convert(event.Total, event.ExchangeRate, event.FunctionalCurrency),
		OpenAmount:       event.Total,
		OpenFunctional:   convert(event.Total, event.ExchangeRate, event.FunctionalCurrency),
	}
	return PostingPlan{Entry: entry, OpenItems: []OpenItem{openItem}}, nil
}

func (e *PostingEngine) BuildReceipt(event ReceiptEvent) (PostingPlan, error) {
	return e.buildSettlement(event, false)
}

func (e *PostingEngine) BuildSupplierPayment(event SupplierPaymentEvent) (PostingPlan, error) {
	return e.buildSettlement(ReceiptEvent(event), true)
}

func (e *PostingEngine) buildSettlement(event ReceiptEvent, supplier bool) (PostingPlan, error) {
	if event.ID == uuid.Nil || event.OpenItemID == uuid.Nil || event.Date.IsZero() ||
		event.Amount.Sign() <= 0 || event.ExchangeRate.Sign() <= 0 ||
		strings.TrimSpace(event.Actor) == "" || strings.TrimSpace(event.IdempotencyKey) == "" {
		return PostingPlan{}, fmt.Errorf("%w: incomplete settlement event", ErrInvalidArgument)
	}
	cashAccount, err := e.account(paymentMethodRole(event.PaymentMethod))
	if err != nil {
		return PostingPlan{}, err
	}
	controlRole := RoleReceivable
	sourceType := "receipt"
	sourceEvent := "receipt.created"
	description := "Cobro de cliente"
	if supplier {
		controlRole = RolePayable
		sourceType = "supplier_payment"
		sourceEvent = "supplier_payment.created"
		description = "Pago a proveedor"
	}
	controlAccount, err := e.account(controlRole)
	if err != nil {
		return PostingPlan{}, err
	}
	bookAmount := event.BookFunctionalAmount
	currentAmount := convert(event.Amount, event.ExchangeRate, event.FunctionalCurrency)
	if bookAmount.IsZero() {
		bookAmount = currentAmount
	}
	if bookAmount.Sign() <= 0 {
		return PostingPlan{}, fmt.Errorf("%w: settlement book amount must be positive", ErrInvalidArgument)
	}
	entry := baseEntry(
		event.ID,
		sourceType,
		sourceEvent,
		description,
		event.Date,
		event.Currency,
		event.FunctionalCurrency,
		event.ExchangeRate,
		event.ExchangeRateDate,
		event.ExchangeRateSource,
		event.Actor,
		event.IdempotencyKey,
	)
	if supplier {
		entry.Lines = append(entry.Lines,
			functionalLine(controlAccount, debitSide, bookAmount, event.FunctionalCurrency, event.PartyID, description),
			lineWithAmounts(cashAccount, creditSide, currentAmount, event.Amount, event.Currency, event.ExchangeRate, event.ExchangeRateDate, event.ExchangeRateSource, nil, description),
		)
	} else {
		entry.Lines = append(entry.Lines,
			lineWithAmounts(cashAccount, debitSide, currentAmount, event.Amount, event.Currency, event.ExchangeRate, event.ExchangeRateDate, event.ExchangeRateSource, nil, description),
			functionalLine(controlAccount, creditSide, bookAmount, event.FunctionalCurrency, event.PartyID, description),
		)
	}
	if difference := currentAmount.Sub(bookAmount); !difference.IsZero() {
		var role string
		var side entrySide
		if supplier {
			if difference.Sign() > 0 {
				role, side = RoleFXLoss, debitSide
			} else {
				role, side = RoleFXGain, creditSide
			}
		} else if difference.Sign() > 0 {
			role, side = RoleFXGain, creditSide
		} else {
			role, side = RoleFXLoss, debitSide
		}
		account, accountErr := e.account(role)
		if accountErr != nil {
			return PostingPlan{}, accountErr
		}
		entry.Lines = append(entry.Lines, functionalLine(account, side, difference.Abs(), event.FunctionalCurrency, nil, "Diferencia de cambio"))
	}
	if err := e.finishEntry(&entry); err != nil {
		return PostingPlan{}, err
	}
	application := OpenItemApplication{
		ID:                 uuid.New(),
		OpenItemID:         event.OpenItemID,
		AppliedAt:          event.Date,
		Amount:             event.Amount,
		FunctionalAmount:   bookAmount,
		ExchangeDifference: currentAmount.Sub(bookAmount),
		CreatedBy:          event.Actor,
	}
	return PostingPlan{Entry: entry, Applications: []OpenItemApplication{application}}, nil
}

func (e *PostingEngine) BuildReturn(event ReturnEvent) (PostingPlan, error) {
	if err := validateDocument(
		event.ID,
		event.Number,
		event.Date,
		event.Currency,
		event.FunctionalCurrency,
		event.ExchangeRate,
		event.Subtotal,
		event.TaxTotal,
		event.Total,
		event.Actor,
		event.IdempotencyKey,
	); err != nil {
		return PostingPlan{}, err
	}
	if err := validateCommercialSubtotal(event.Lines, event.Subtotal, event.Currency); err != nil {
		return PostingPlan{}, err
	}
	revenueAccount, err := e.account(RoleRevenue)
	if err != nil {
		return PostingPlan{}, err
	}
	entry := baseEntry(
		event.ID,
		"customer_credit_note",
		"customer_credit_note.created",
		"Devolución "+event.Number,
		event.Date,
		event.Currency,
		event.FunctionalCurrency,
		event.ExchangeRate,
		event.ExchangeRateDate,
		event.ExchangeRateSource,
		event.Actor,
		event.IdempotencyKey,
	)
	entry.Lines = append(entry.Lines, e.transactionLine(revenueAccount, debitSide, event.Subtotal, nil, "Devolución "+event.Number, entry))
	taxLines, err := e.taxLines(event.Lines, event.TaxTotal, debitSide, "vat_payable_", "IVA devolución "+event.Number, entry)
	if err != nil {
		return PostingPlan{}, err
	}
	entry.Lines = append(entry.Lines, taxLines...)

	refundRole := paymentMethodRole(event.OriginalPaymentMethod)
	var partyID *uuid.UUID
	if strings.EqualFold(strings.TrimSpace(event.RefundMethod), "credit_note") {
		refundRole = RoleCreditNotePayable
		partyID = event.PartyID
	}
	refundAccount, err := e.account(refundRole)
	if err != nil {
		return PostingPlan{}, err
	}
	entry.Lines = append(entry.Lines, e.transactionLine(refundAccount, creditSide, event.Total, partyID, "Devolución "+event.Number, entry))

	if event.CostConfirmed {
		var returnedCost Decimal
		for _, line := range event.Lines {
			if line.Cost.Sign() < 0 {
				return PostingPlan{}, fmt.Errorf("%w: returned cost cannot be negative", ErrInvalidArgument)
			}
			returnedCost = returnedCost.Add(line.Cost)
		}
		returnedCost = event.FunctionalCurrency.Round(returnedCost)
		if !returnedCost.IsZero() {
			inventoryAccount, accountErr := e.account(RoleInventory)
			if accountErr != nil {
				return PostingPlan{}, accountErr
			}
			cogsAccount, accountErr := e.account(RoleCOGS)
			if accountErr != nil {
				return PostingPlan{}, accountErr
			}
			entry.Lines = append(entry.Lines,
				functionalLine(inventoryAccount, debitSide, returnedCost, event.FunctionalCurrency, nil, "Reingreso inventario "+event.Number),
				functionalLine(cogsAccount, creditSide, returnedCost, event.FunctionalCurrency, nil, "Reversa CMV "+event.Number),
			)
		}
	}
	if err := e.finishEntry(&entry); err != nil {
		return PostingPlan{}, err
	}
	return PostingPlan{Entry: entry}, nil
}

// BuildCustomerDebitNote uses the sale posting rule but preserves a distinct,
// immutable source identity.
func (e *PostingEngine) BuildCustomerDebitNote(event SaleEvent) (PostingPlan, error) {
	plan, err := e.BuildSale(event)
	if err != nil {
		return PostingPlan{}, err
	}
	plan.Entry.Source.Type = "customer_debit_note"
	plan.Entry.Source.Event = "customer_debit_note.created"
	plan.Entry.Description = "Nota de débito " + event.Number
	for index := range plan.OpenItems {
		plan.OpenItems[index].SourceType = plan.Entry.Source.Type
	}
	return plan, nil
}

type taxShare struct {
	rate   Decimal
	base   Decimal
	amount Decimal
}

func distributeTax(lines []CommercialLine, total Decimal, currency Currency) ([]taxShare, error) {
	type bucket struct {
		rate Decimal
		base Decimal
	}
	bucketsByRate := make(map[string]bucket)
	for _, line := range lines {
		if line.NetAmount.Sign() < 0 || line.TaxRate.Sign() < 0 {
			return nil, fmt.Errorf("%w: tax basis and rate cannot be negative", ErrInvalidArgument)
		}
		if line.TaxRate.IsZero() {
			continue
		}
		key := line.TaxRate.String()
		current := bucketsByRate[key]
		current.rate = line.TaxRate
		current.base = current.base.Add(line.NetAmount)
		bucketsByRate[key] = current
	}
	if len(bucketsByRate) == 0 {
		if total.IsZero() {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: tax total has no taxable basis", ErrUnbalancedEntry)
	}
	buckets := make([]bucket, 0, len(bucketsByRate))
	for _, current := range bucketsByRate {
		buckets = append(buckets, current)
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].rate.Cmp(buckets[j].rate) < 0
	})
	shares := make([]taxShare, 0, len(buckets))
	var calculated Decimal
	largest := 0
	for index, current := range buckets {
		percentage, err := current.base.Mul(current.rate).Quo(MustDecimal("100"), currency.MinorUnits())
		if err != nil {
			return nil, err
		}
		amount := currency.Round(percentage)
		shares = append(shares, taxShare{rate: current.rate, base: current.base, amount: amount})
		calculated = calculated.Add(amount)
		if current.base.Cmp(buckets[largest].base) > 0 {
			largest = index
		}
	}
	residual := currency.Round(total.Sub(calculated))
	if !residual.IsZero() {
		shares[largest].amount = currency.Round(shares[largest].amount.Add(residual))
	}
	var reconciled Decimal
	for _, share := range shares {
		if share.amount.Sign() < 0 {
			return nil, fmt.Errorf("%w: tax residual produced a negative tax line", ErrUnbalancedEntry)
		}
		reconciled = reconciled.Add(share.amount)
	}
	if !currency.Round(reconciled).Equal(currency.Round(total)) {
		return nil, ErrUnbalancedEntry
	}
	return shares, nil
}

type entrySide bool

const (
	debitSide  entrySide = false
	creditSide entrySide = true
)

func (e *PostingEngine) taxLines(
	basis []CommercialLine,
	total Decimal,
	side entrySide,
	rolePrefix string,
	memo string,
	entry JournalEntry,
) ([]JournalLine, error) {
	shares, err := distributeTax(basis, total, entry.Currency)
	if err != nil {
		return nil, err
	}
	lines := make([]JournalLine, 0, len(shares))
	for _, share := range shares {
		if share.amount.IsZero() {
			continue
		}
		account, accountErr := e.account(rolePrefix + rateKey(share.rate))
		if accountErr != nil {
			return nil, accountErr
		}
		lines = append(lines, e.transactionLine(
			account,
			side,
			share.amount,
			nil,
			fmt.Sprintf("%s %s%%", memo, share.rate.String()),
			entry,
		))
	}
	return lines, nil
}

func (e *PostingEngine) finishEntry(entry *JournalEntry) error {
	var functionalDebit, functionalCredit Decimal
	for _, line := range entry.Lines {
		functionalDebit = functionalDebit.Add(line.Debit)
		functionalCredit = functionalCredit.Add(line.Credit)
	}
	difference := entry.FunctionalCurrency.Round(functionalDebit.Sub(functionalCredit))
	if !difference.IsZero() {
		account, err := e.account(RoleRoundingDifference)
		if err != nil {
			return err
		}
		side := creditSide
		if difference.Sign() < 0 {
			side = debitSide
		}
		entry.Lines = append(entry.Lines, functionalLine(account, side, difference.Abs(), entry.FunctionalCurrency, nil, "Diferencia de redondeo"))
	}
	for index := range entry.Lines {
		entry.Lines[index].LineNo = index + 1
	}
	return entry.ValidateForPosting()
}

func (e *PostingEngine) account(role string) (uuid.UUID, error) {
	mapping, ok := e.mappings[role]
	if !ok || mapping.AccountID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: %s", ErrMappingMissing, role)
	}
	return mapping.AccountID, nil
}

func (e *PostingEngine) transactionLine(
	accountID uuid.UUID,
	side entrySide,
	amount Decimal,
	partyID *uuid.UUID,
	memo string,
	entry JournalEntry,
) JournalLine {
	return lineWithAmounts(
		accountID,
		side,
		convert(amount, entry.ExchangeRate, entry.FunctionalCurrency),
		amount,
		entry.Currency,
		entry.ExchangeRate,
		entry.ExchangeRateDate,
		entry.ExchangeRateSource,
		partyID,
		memo,
	)
}

func lineWithAmounts(
	accountID uuid.UUID,
	side entrySide,
	functionalAmount Decimal,
	transactionAmount Decimal,
	currency Currency,
	exchangeRate Decimal,
	exchangeRateDate time.Time,
	exchangeRateSource string,
	partyID *uuid.UUID,
	memo string,
) JournalLine {
	line := JournalLine{
		AccountID:          accountID,
		Currency:           currency,
		ExchangeRate:       exchangeRate,
		ExchangeRateDate:   exchangeRateDate,
		ExchangeRateSource: exchangeRateSource,
		PartyID:            partyID,
		Memo:               memo,
	}
	if side == debitSide {
		line.Debit = functionalAmount
		line.TransactionDebit = transactionAmount
	} else {
		line.Credit = functionalAmount
		line.TransactionCredit = transactionAmount
	}
	return line
}

func functionalLine(
	accountID uuid.UUID,
	side entrySide,
	amount Decimal,
	functionalCurrency Currency,
	partyID *uuid.UUID,
	memo string,
) JournalLine {
	return lineWithAmounts(
		accountID,
		side,
		amount,
		amount,
		functionalCurrency,
		One,
		time.Time{},
		"",
		partyID,
		memo,
	)
}

func convert(amount, exchangeRate Decimal, functional Currency) Decimal {
	return functional.Round(amount.Mul(exchangeRate))
}

func baseEntry(
	id uuid.UUID,
	sourceType string,
	sourceEvent string,
	description string,
	date time.Time,
	currency Currency,
	functionalCurrency Currency,
	exchangeRate Decimal,
	exchangeRateDate time.Time,
	exchangeRateSource string,
	actor string,
	idempotencyKey string,
) JournalEntry {
	if exchangeRateDate.IsZero() {
		exchangeRateDate = date
	}
	return JournalEntry{
		Date:               date,
		Kind:               entryKindForSource(sourceType),
		PostingKind:        "primary",
		FunctionalCurrency: functionalCurrency,
		Currency:           currency,
		ExchangeRate:       exchangeRate,
		ExchangeRateDate:   exchangeRateDate,
		ExchangeRateSource: exchangeRateSource,
		Source: EntrySource{
			Type:           sourceType,
			ID:             id,
			Event:          sourceEvent,
			IdempotencyKey: idempotencyKey,
		},
		Description: description,
		CreatedBy:   actor,
	}
}

func entryKindForSource(sourceType string) EntryKind {
	switch sourceType {
	case "sale", "customer_debit_note":
		return EntrySale
	case "purchase":
		return EntryPurchase
	case "receipt":
		return EntryCollection
	case "supplier_payment":
		return EntryPayment
	case "customer_credit_note":
		return EntryRefund
	default:
		return EntryManual
	}
}

func validateDocument(
	id uuid.UUID,
	number string,
	date time.Time,
	currency Currency,
	functionalCurrency Currency,
	rate Decimal,
	subtotal Decimal,
	tax Decimal,
	total Decimal,
	actor string,
	idempotencyKey string,
) error {
	if id == uuid.Nil || strings.TrimSpace(number) == "" || date.IsZero() ||
		strings.TrimSpace(actor) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return fmt.Errorf("%w: incomplete commercial event", ErrInvalidArgument)
	}
	if subtotal.Sign() < 0 || tax.Sign() < 0 || total.Sign() <= 0 {
		return fmt.Errorf("%w: commercial amounts are invalid", ErrInvalidArgument)
	}
	if rate.Sign() <= 0 {
		return fmt.Errorf("%w: exchange rate must be positive", ErrInvalidArgument)
	}
	if currency.Code() == functionalCurrency.Code() && !rate.Equal(One) {
		return fmt.Errorf("%w: functional-currency document must use exchange rate 1", ErrInvalidArgument)
	}
	if !currency.Round(subtotal.Add(tax)).Equal(currency.Round(total)) {
		return fmt.Errorf("%w: subtotal plus tax must equal total", ErrUnbalancedEntry)
	}
	return nil
}

func paymentMethodRole(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "transfer", "bank", "bank_transfer":
		return RoleBank
	case "card", "credit_card", "debit_card":
		return RoleCardClearing
	case "wallet", "mercadopago", "mp":
		return RoleWalletClearing
	case "check", "cheque":
		return RoleChecksClearing
	default:
		return RoleCash
	}
}

func rateKey(rate Decimal) string {
	// Decimal.String already removes fractional trailing zeroes. Removing zeroes
	// from the whole string would turn a valid 10% rate into the 1% mapping.
	return strings.ReplaceAll(rate.String(), ".", "")
}

func validateCommercialSubtotal(
	lines []CommercialLine,
	subtotal Decimal,
	currency Currency,
) error {
	var lineSubtotal Decimal
	for _, line := range lines {
		if line.NetAmount.Sign() < 0 || line.TaxRate.Sign() < 0 {
			return fmt.Errorf("%w: commercial lines cannot contain negative amounts", ErrInvalidArgument)
		}
		lineSubtotal = lineSubtotal.Add(line.NetAmount)
	}
	if !currency.Round(lineSubtotal).Equal(currency.Round(subtotal)) {
		return fmt.Errorf("%w: commercial lines do not reconcile with subtotal", ErrUnbalancedEntry)
	}
	return nil
}
