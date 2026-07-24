package accounting

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildSaleCharacterizesCashCreditVATAndCOGS(t *testing.T) {
	t.Parallel()

	engine, accounts := postingEngineFixture()
	partyID := uuid.New()
	base := saleEventFixture()
	base.CostConfirmed = true
	base.Lines[0].Cost = MustDecimal("60")

	cashPlan, err := engine.BuildSale(base)
	if err != nil {
		t.Fatal(err)
	}
	assertEntryBalanced(t, cashPlan.Entry)
	assertLine(t, cashPlan.Entry.Lines, accounts[RoleCash], "121", "0")
	assertLine(t, cashPlan.Entry.Lines, accounts[RoleRevenue], "0", "100")
	assertLine(t, cashPlan.Entry.Lines, accounts["vat_payable_21"], "0", "21")
	assertLine(t, cashPlan.Entry.Lines, accounts[RoleCOGS], "60", "0")
	assertLine(t, cashPlan.Entry.Lines, accounts[RoleInventory], "0", "60")
	if len(cashPlan.OpenItems) != 0 {
		t.Fatalf("cash sale created %d open items", len(cashPlan.OpenItems))
	}

	credit := base
	credit.ID = uuid.New()
	credit.OnCredit = true
	credit.PartyID = &partyID
	credit.IdempotencyKey = "sale-credit"
	credit.CostConfirmed = false
	creditPlan, err := engine.BuildSale(credit)
	if err != nil {
		t.Fatal(err)
	}
	assertLine(t, creditPlan.Entry.Lines, accounts[RoleReceivable], "121", "0")
	if len(creditPlan.OpenItems) != 1 || creditPlan.OpenItems[0].PartyID != partyID {
		t.Fatalf("credit sale open items = %+v", creditPlan.OpenItems)
	}
}

func TestBuildSaleUsesClearingAccountByPaymentMethod(t *testing.T) {
	t.Parallel()

	engine, accounts := postingEngineFixture()
	event := saleEventFixture()
	event.PaymentMethod = "card"
	plan, err := engine.BuildSale(event)
	if err != nil {
		t.Fatal(err)
	}
	assertLine(t, plan.Entry.Lines, accounts[RoleCardClearing], "121", "0")
}

func TestRateKeyPreservesIntegerTrailingZero(t *testing.T) {
	t.Parallel()

	if got := rateKey(MustDecimal("10")); got != "10" {
		t.Fatalf("10%% rate key = %q, want 10", got)
	}
	if got := rateKey(MustDecimal("10.5")); got != "105" {
		t.Fatalf("10.5%% rate key = %q, want 105", got)
	}
}

func TestBuildSaleRejectsLinesThatDoNotReconcileWithSubtotal(t *testing.T) {
	t.Parallel()

	engine, _ := postingEngineFixture()
	event := saleEventFixture()
	event.Lines[0].NetAmount = MustDecimal("99")
	if _, err := engine.BuildSale(event); !errors.Is(err, ErrUnbalancedEntry) {
		t.Fatalf("error = %v, want ErrUnbalancedEntry", err)
	}
}

func TestTaxDistributionAnchorsPersistedTotalAndLargestBaseGetsResidual(t *testing.T) {
	t.Parallel()

	shares, err := distributeTax([]CommercialLine{
		{NetAmount: MustDecimal("33.33"), TaxRate: MustDecimal("21")},
		{NetAmount: MustDecimal("66.67"), TaxRate: MustDecimal("10.5")},
	}, MustDecimal("14.01"), MustCurrency("ARS"))
	if err != nil {
		t.Fatal(err)
	}
	if len(shares) != 2 {
		t.Fatalf("shares = %+v", shares)
	}
	var total Decimal
	for _, share := range shares {
		total = total.Add(share.amount)
	}
	if total.String() != "14.01" {
		t.Fatalf("distributed tax = %s, want persisted 14.01", total)
	}
	for _, share := range shares {
		if share.rate.Equal(MustDecimal("10.5")) && share.amount.String() != "7.01" {
			t.Fatalf("largest basis share = %s, want 7.01", share.amount)
		}
	}
}

func TestBuildPurchaseSplitsInventoryExpenseVATAndPayable(t *testing.T) {
	t.Parallel()

	engine, accounts := postingEngineFixture()
	partyID := uuid.New()
	event := PurchaseEvent{
		ID:                 uuid.New(),
		Number:             "FC-1",
		Date:               dateFixture(),
		Currency:           MustCurrency("ARS"),
		FunctionalCurrency: MustCurrency("ARS"),
		ExchangeRate:       One,
		PartyID:            &partyID,
		Subtotal:           MustDecimal("150"),
		TaxTotal:           MustDecimal("31.5"),
		Total:              MustDecimal("181.5"),
		Lines: []PurchaseLine{
			{NetAmount: MustDecimal("100"), TaxRate: MustDecimal("21"), IsInventory: true},
			{NetAmount: MustDecimal("50"), TaxRate: MustDecimal("21")},
		},
		Actor:          "user_1",
		IdempotencyKey: "purchase-1",
	}
	plan, err := engine.BuildPurchase(event)
	if err != nil {
		t.Fatal(err)
	}
	assertEntryBalanced(t, plan.Entry)
	assertLine(t, plan.Entry.Lines, accounts[RoleInventory], "100", "0")
	assertLine(t, plan.Entry.Lines, accounts[RolePurchaseExpense], "50", "0")
	assertLine(t, plan.Entry.Lines, accounts["vat_credit_21"], "31.5", "0")
	assertLine(t, plan.Entry.Lines, accounts[RolePayable], "0", "181.5")
	if len(plan.OpenItems) != 1 || plan.OpenItems[0].Kind != Payable {
		t.Fatalf("purchase open items = %+v", plan.OpenItems)
	}
}

func TestReceiptRecognizesExchangeGainWithoutChangingOpenItemBookValue(t *testing.T) {
	t.Parallel()

	engine, accounts := postingEngineFixture()
	partyID := uuid.New()
	event := ReceiptEvent{
		ID:                   uuid.New(),
		OpenItemID:           uuid.New(),
		PartyID:              &partyID,
		Date:                 dateFixture(),
		Currency:             MustCurrency("USD"),
		FunctionalCurrency:   MustCurrency("ARS"),
		ExchangeRate:         MustDecimal("1000"),
		ExchangeRateDate:     dateFixture(),
		ExchangeRateSource:   "BNA",
		PaymentMethod:        "transfer",
		Amount:               MustDecimal("100"),
		BookFunctionalAmount: MustDecimal("90000"),
		Actor:                "user_1",
		IdempotencyKey:       "receipt-1",
	}
	plan, err := engine.BuildReceipt(event)
	if err != nil {
		t.Fatal(err)
	}
	assertEntryBalanced(t, plan.Entry)
	assertLine(t, plan.Entry.Lines, accounts[RoleBank], "100000", "0")
	assertLine(t, plan.Entry.Lines, accounts[RoleReceivable], "0", "90000")
	assertLine(t, plan.Entry.Lines, accounts[RoleFXGain], "0", "10000")
	if len(plan.Applications) != 1 || plan.Applications[0].FunctionalAmount.String() != "90000" {
		t.Fatalf("applications = %+v", plan.Applications)
	}
	if plan.Applications[0].ExchangeDifference.String() != "10000" {
		t.Fatalf("exchange difference = %s", plan.Applications[0].ExchangeDifference)
	}
}

func TestBuildReturnReversesRevenueVATAndCOGS(t *testing.T) {
	t.Parallel()

	engine, accounts := postingEngineFixture()
	event := ReturnEvent{
		ID:                 uuid.New(),
		SaleID:             uuid.New(),
		Number:             "NC-1",
		Date:               dateFixture(),
		Currency:           MustCurrency("ARS"),
		FunctionalCurrency: MustCurrency("ARS"),
		ExchangeRate:       One,
		RefundMethod:       "cash",
		Subtotal:           MustDecimal("100"),
		TaxTotal:           MustDecimal("21"),
		Total:              MustDecimal("121"),
		Lines: []CommercialLine{{
			NetAmount: MustDecimal("100"),
			TaxRate:   MustDecimal("21"),
			Cost:      MustDecimal("60"),
		}},
		CostConfirmed:  true,
		Actor:          "user_1",
		IdempotencyKey: "return-1",
	}
	plan, err := engine.BuildReturn(event)
	if err != nil {
		t.Fatal(err)
	}
	assertEntryBalanced(t, plan.Entry)
	assertLine(t, plan.Entry.Lines, accounts[RoleRevenue], "100", "0")
	assertLine(t, plan.Entry.Lines, accounts["vat_payable_21"], "21", "0")
	assertLine(t, plan.Entry.Lines, accounts[RoleCash], "0", "121")
	assertLine(t, plan.Entry.Lines, accounts[RoleInventory], "60", "0")
	assertLine(t, plan.Entry.Lines, accounts[RoleCOGS], "0", "60")
}

func TestPostingFailsClosedWhenMappingIsMissing(t *testing.T) {
	t.Parallel()

	engine := NewPostingEngine(map[string]AccountMapping{})
	_, err := engine.BuildSale(saleEventFixture())
	if !errors.Is(err, ErrMappingMissing) {
		t.Fatalf("error = %v, want ErrMappingMissing", err)
	}
}

func postingEngineFixture() (*PostingEngine, map[string]uuid.UUID) {
	roles := []string{
		RoleCash,
		RoleBank,
		RoleCardClearing,
		RoleWalletClearing,
		RoleChecksClearing,
		RoleReceivable,
		RolePayable,
		RoleRevenue,
		RoleInventory,
		RoleCOGS,
		RolePurchaseExpense,
		RoleCreditNotePayable,
		RoleFXGain,
		RoleFXLoss,
		RoleRoundingDifference,
		"vat_payable_21",
		"vat_payable_105",
		"vat_credit_21",
		"vat_credit_105",
	}
	ids := make(map[string]uuid.UUID, len(roles))
	mappings := make(map[string]AccountMapping, len(roles))
	for _, role := range roles {
		id := uuid.New()
		ids[role] = id
		mappings[role] = AccountMapping{Role: role, AccountID: id}
	}
	return NewPostingEngine(mappings), ids
}

func saleEventFixture() SaleEvent {
	return SaleEvent{
		ID:                 uuid.New(),
		Number:             "V-1",
		Date:               dateFixture(),
		Currency:           MustCurrency("ARS"),
		FunctionalCurrency: MustCurrency("ARS"),
		ExchangeRate:       One,
		PaymentMethod:      "cash",
		Subtotal:           MustDecimal("100"),
		TaxTotal:           MustDecimal("21"),
		Total:              MustDecimal("121"),
		Lines: []CommercialLine{{
			NetAmount: MustDecimal("100"),
			TaxRate:   MustDecimal("21"),
		}},
		Actor:          "user_1",
		IdempotencyKey: "sale-1",
	}
}

func dateFixture() time.Time {
	return time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
}

func assertEntryBalanced(t *testing.T, entry JournalEntry) {
	t.Helper()
	if err := entry.ValidateForPosting(); err != nil {
		t.Fatalf("entry is invalid: %v\n%+v", err, entry)
	}
	debit, credit := entry.Totals()
	if !debit.Equal(credit) {
		t.Fatalf("debit %s != credit %s", debit, credit)
	}
}

func assertLine(t *testing.T, lines []JournalLine, accountID uuid.UUID, debit, credit string) {
	t.Helper()
	for _, line := range lines {
		if line.AccountID != accountID {
			continue
		}
		if line.Debit.String() != debit || line.Credit.String() != credit {
			t.Fatalf("account %s line = debit %s credit %s, want %s/%s", accountID, line.Debit, line.Credit, debit, credit)
		}
		return
	}
	t.Fatalf("account %s line not found in %+v", accountID, lines)
}
