package fiscalaccounting

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

func TestBuildPostingPlanUsesExactSnapshotAmounts(t *testing.T) {
	t.Parallel()

	partyID := uuid.New()
	mappings := postingMappings()
	document := fiscalDocument(map[string]string{
		"accounting.party_id":  partyID.String(),
		"accounting.on_credit": "true",
	})
	tests := []struct {
		name           string
		operation      fiscal.Operation
		controlRole    string
		controlDebit   bool
		expectedKind   accounting.EntryKind
		expectedSource string
	}{
		{
			name:           "invoice",
			operation:      fiscal.OperationInvoice,
			controlRole:    accounting.RoleReceivable,
			controlDebit:   true,
			expectedKind:   accounting.EntrySale,
			expectedSource: "sale_document",
		},
		{
			name:           "debit note",
			operation:      fiscal.OperationDebitNote,
			controlRole:    accounting.RoleReceivable,
			controlDebit:   true,
			expectedKind:   accounting.EntrySale,
			expectedSource: "sale_document",
		},
		{
			name:           "credit note",
			operation:      fiscal.OperationCreditNote,
			controlRole:    accounting.RoleReceivable,
			controlDebit:   false,
			expectedKind:   accounting.EntryRefund,
			expectedSource: "sale_document",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			item := postingWorkItem(test.operation)
			item.SourceType = test.expectedSource
			plan, err := buildPostingPlan(
				item,
				document,
				mappings,
				"system:test",
			)
			if err != nil {
				t.Fatalf("build posting plan: %v", err)
			}
			if err := plan.Entry.ValidateForPosting(); err != nil {
				t.Fatalf("validate posting entry: %v", err)
			}
			if plan.Entry.Kind != test.expectedKind {
				t.Fatalf("unexpected entry kind: %s", plan.Entry.Kind)
			}
			if plan.Entry.Source.Type != test.expectedSource {
				t.Fatalf("unexpected source type: %s", plan.Entry.Source.Type)
			}
			if plan.Entry.Source.ID != item.SourceID {
				t.Fatalf("source id changed: %s", plan.Entry.Source.ID)
			}
			if plan.Entry.Source.IdempotencyKey !=
				"fiscal-accounting:"+item.IntentID.String() {
				t.Fatalf(
					"unexpected idempotency key: %s",
					plan.Entry.Source.IdempotencyKey,
				)
			}

			control := requireLine(
				t,
				plan.Entry.Lines,
				mappings[test.controlRole].AccountID,
			)
			if test.controlDebit {
				assertDecimal(t, control.TransactionDebit, "180")
				assertDecimal(t, control.Debit, "180")
				if control.PartyID == nil || *control.PartyID != partyID {
					t.Fatalf("control line does not retain party")
				}
			} else {
				assertDecimal(t, control.TransactionCredit, "180")
				assertDecimal(t, control.Credit, "180")
				if control.PartyID == nil || *control.PartyID != partyID {
					t.Fatalf("credit-note control line does not retain party")
				}
			}
			assertFiscalTaxSide(
				t,
				plan.Entry.Lines,
				mappings,
				test.operation == fiscal.OperationCreditNote,
			)
			debit, credit := plan.Entry.Totals()
			if !debit.Equal(credit) {
				t.Fatalf("entry is not balanced: %s != %s", debit, credit)
			}

			if test.operation == fiscal.OperationInvoice ||
				test.operation == fiscal.OperationDebitNote {
				if len(plan.OpenItems) != 1 {
					t.Fatalf("expected one receivable, got %d", len(plan.OpenItems))
				}
				assertDecimal(t, plan.OpenItems[0].OriginalAmount, "180")
				assertDecimal(t, plan.OpenItems[0].FunctionalAmount, "180")
				if plan.OpenItems[0].DueDate.Format("2006-01-02") != "2026-08-15" {
					t.Fatalf("fiscal payment due date was not retained")
				}
				if plan.OpenItems[0].SourceType != test.expectedSource {
					t.Fatalf(
						"unexpected open-item source: %s",
						plan.OpenItems[0].SourceType,
					)
				}
			} else {
				if len(plan.OpenItems) != 0 {
					t.Fatalf("credit note must not create a receivable")
				}
				if len(plan.Applications) != 1 ||
					plan.Applications[0].OpenItemID != *item.AssociatedOpenItemID {
					t.Fatalf("credit note does not apply its associated receivable")
				}
				assertDecimal(t, plan.Applications[0].Amount, "180")
			}
		})
	}
}

func TestBuildPostingPlanAddsExplicitFunctionalRounding(t *testing.T) {
	t.Parallel()

	mappings := postingMappings()
	document := fiscal.FiscalSnapshot{
		Version:     fiscal.SnapshotVersion,
		CountryCode: "AR",
		IssueDate:   "2026-07-24",
		PaymentDue:  "2026-08-15",
		Issuer:      fiscal.PartySnapshot{Name: "Emisor"},
		Receiver:    fiscal.PartySnapshot{Name: "Cliente"},
		Currency: fiscal.CurrencySnapshot{
			Code:       "USD",
			Rate:       fiscal.MustDecimal("100.15"),
			RateDate:   "2026-07-24",
			RateSource: "BNA",
		},
		Lines: []fiscal.FiscalLineSnapshot{{
			Position:    1,
			Description: "Servicio",
			Quantity:    fiscal.MustDecimal("1"),
			UnitPrice:   fiscal.MustDecimal("0.03"),
			NetAmount:   fiscal.MustDecimal("0.03"),
			TaxRate:     fiscal.MustDecimal("21"),
			TaxAmount:   fiscal.MustDecimal("0.01"),
			TotalAmount: fiscal.MustDecimal("0.04"),
		}},
		Totals: fiscal.FiscalTotalsSnapshot{
			NetTaxed: fiscal.MustDecimal("0.03"),
			VAT:      fiscal.MustDecimal("0.01"),
			Total:    fiscal.MustDecimal("0.04"),
		},
	}
	item := postingWorkItem(fiscal.OperationInvoice)
	plan, err := buildPostingPlan(item, document, mappings, "system:test")
	if err != nil {
		t.Fatalf("build foreign-currency posting: %v", err)
	}
	rounding := requireLine(
		t,
		plan.Entry.Lines,
		mappings[accounting.RoleRoundingDifference].AccountID,
	)
	assertDecimal(t, rounding.TransactionCredit, "0.01")
	assertDecimal(t, rounding.Credit, "0.01")
	if rounding.Currency.Code() != "ARS" ||
		!rounding.ExchangeRate.Equal(accounting.One) {
		t.Fatalf("rounding line must be in functional currency")
	}
	if err := plan.Entry.ValidateForPosting(); err != nil {
		t.Fatalf("validate foreign-currency posting: %v", err)
	}
}

func TestBuildPostingPlanPreservesConfirmedCostAndInventoryLines(t *testing.T) {
	t.Parallel()
	document := fiscalDocument(map[string]string{
		"accounting.on_credit":      "false",
		"accounting.payment_method": "cash",
	})
	document.Lines[0].CostAmount = fiscal.MustDecimal("60")
	document.Lines[0].CostConfirmed = true
	mappings := postingMappings()
	plan, err := buildPostingPlan(
		postingWorkItem(fiscal.OperationInvoice),
		document,
		mappings,
		"system:test",
	)
	if err != nil {
		t.Fatalf("build posting plan: %v", err)
	}
	cogs := requireLine(t, plan.Entry.Lines, mappings[accounting.RoleCOGS].AccountID)
	inventory := requireLine(
		t,
		plan.Entry.Lines,
		mappings[accounting.RoleInventory].AccountID,
	)
	assertDecimal(t, cogs.Debit, "60")
	assertDecimal(t, inventory.Credit, "60")
	if err := plan.Entry.ValidateForPosting(); err != nil {
		t.Fatalf("validate cost posting: %v", err)
	}
}

func TestBuildPostingPlanCreditNoteUsesOriginalBookValueAndFXGain(t *testing.T) {
	t.Parallel()
	partyID := uuid.New()
	document := fiscalDocument(map[string]string{
		"accounting.party_id":      partyID.String(),
		"accounting.on_credit":     "true",
		"accounting.refund_method": "credit_note",
	})
	document.Currency.Code = "USD"
	document.Currency.Rate = fiscal.MustDecimal("2")
	document.Totals.Functional = fiscal.MustDecimal("360")
	item := postingWorkItem(fiscal.OperationCreditNote)
	currency := "USD"
	openCurrency := "180"
	openFunctional := "300"
	item.AssociatedOpenCurrency = &currency
	item.AssociatedOpenCurrencyAmount = &openCurrency
	item.AssociatedOpenFunctional = &openFunctional
	mappings := postingMappings()
	plan, err := buildPostingPlan(item, document, mappings, "system:test")
	if err != nil {
		t.Fatalf("build credit-note posting plan: %v", err)
	}
	receivable := requireLine(
		t,
		plan.Entry.Lines,
		mappings[accounting.RoleReceivable].AccountID,
	)
	assertDecimal(t, receivable.TransactionCredit, "300")
	assertDecimal(t, receivable.Credit, "300")
	gain := requireLine(
		t,
		plan.Entry.Lines,
		mappings[accounting.RoleFXGain].AccountID,
	)
	assertDecimal(t, gain.Credit, "60")
	if len(plan.Applications) != 1 {
		t.Fatalf("applications = %d, want 1", len(plan.Applications))
	}
	assertDecimal(t, plan.Applications[0].FunctionalAmount, "300")
	assertDecimal(t, plan.Applications[0].ExchangeDifference, "60")
	if err := plan.Entry.ValidateForPosting(); err != nil {
		t.Fatalf("validate credit-note FX posting: %v", err)
	}
}

func TestEntriesEquivalentAggregatesExactLines(t *testing.T) {
	t.Parallel()

	accountID := uuid.New()
	sourceID := uuid.New()
	partyID := uuid.New()
	expected := accounting.JournalEntry{
		Date:               mustDate(t, "2026-07-24"),
		Kind:               accounting.EntrySale,
		PostingKind:        "primary",
		FunctionalCurrency: accounting.MustCurrency("ARS"),
		Source: accounting.EntrySource{
			Type: "sale",
			ID:   sourceID,
		},
		Lines: []accounting.JournalLine{{
			AccountID:        accountID,
			Debit:            accounting.MustDecimal("10"),
			TransactionDebit: accounting.MustDecimal("10"),
			Currency:         accounting.MustCurrency("ARS"),
			ExchangeRate:     accounting.One,
			PartyID:          &partyID,
		}},
	}
	existing := expected
	existing.Lines = []accounting.JournalLine{
		{
			AccountID:        accountID,
			Debit:            accounting.MustDecimal("4"),
			TransactionDebit: accounting.MustDecimal("4"),
			Currency:         accounting.MustCurrency("ARS"),
			ExchangeRate:     accounting.One,
			PartyID:          &partyID,
		},
		{
			AccountID:        accountID,
			Debit:            accounting.MustDecimal("6"),
			TransactionDebit: accounting.MustDecimal("6"),
			Currency:         accounting.MustCurrency("ARS"),
			ExchangeRate:     accounting.One,
			PartyID:          &partyID,
		},
	}
	if !entriesEquivalent(existing, expected) {
		t.Fatalf("semantically equal entries were considered different")
	}
	existing.Lines[1].Debit = accounting.MustDecimal("6.01")
	existing.Lines[1].TransactionDebit = accounting.MustDecimal("6.01")
	if entriesEquivalent(existing, expected) {
		t.Fatalf("different exact amounts were considered equal")
	}
}

func postingWorkItem(operation fiscal.Operation) workItem {
	associatedOpenItemID := uuid.New()
	currency := "ARS"
	openCurrency := "180"
	openFunctional := "180"
	return workItem{
		OrganizationID:               uuid.New(),
		IntentID:                     uuid.New(),
		VoucherID:                    uuid.New(),
		SourceType:                   "sale",
		SourceID:                     uuid.New(),
		Operation:                    operation,
		VoucherType:                  1,
		PointOfSale:                  3,
		VoucherNumber:                42,
		FunctionalCode:               "ARS",
		AssociatedOpenItemID:         &associatedOpenItemID,
		AssociatedOpenCurrency:       &currency,
		AssociatedOpenCurrencyAmount: &openCurrency,
		AssociatedOpenFunctional:     &openFunctional,
	}
}

func fiscalDocument(metadata map[string]string) fiscal.FiscalSnapshot {
	return fiscal.FiscalSnapshot{
		Version:     fiscal.SnapshotVersion,
		CountryCode: "AR",
		IssueDate:   "2026-07-24",
		PaymentDue:  "2026-08-15",
		Issuer:      fiscal.PartySnapshot{Name: "Emisor"},
		Receiver:    fiscal.PartySnapshot{Name: "Cliente"},
		Currency: fiscal.CurrencySnapshot{
			Code:       "ARS",
			Rate:       fiscal.MustDecimal("1"),
			RateDate:   "2026-07-24",
			RateSource: "functional",
		},
		Lines: []fiscal.FiscalLineSnapshot{
			{
				Position:    1,
				Description: "Producto",
				Quantity:    fiscal.MustDecimal("1"),
				UnitPrice:   fiscal.MustDecimal("100"),
				NetAmount:   fiscal.MustDecimal("100"),
				TaxRate:     fiscal.MustDecimal("21"),
				TaxAmount:   fiscal.MustDecimal("21"),
				TotalAmount: fiscal.MustDecimal("121"),
			},
			{
				Position:    2,
				Description: "Servicio",
				Quantity:    fiscal.MustDecimal("1"),
				UnitPrice:   fiscal.MustDecimal("50"),
				NetAmount:   fiscal.MustDecimal("50"),
				TaxRate:     fiscal.MustDecimal("10.5"),
				TaxAmount:   fiscal.MustDecimal("5.25"),
				TotalAmount: fiscal.MustDecimal("55.25"),
			},
		},
		Taxes: []fiscal.TaxSnapshot{{
			Code:       "other",
			BaseAmount: fiscal.MustDecimal("150"),
			Rate:       fiscal.MustDecimal("2.5"),
			Amount:     fiscal.MustDecimal("3.75"),
		}},
		Totals: fiscal.FiscalTotalsSnapshot{
			NetTaxed:    fiscal.MustDecimal("150"),
			VAT:         fiscal.MustDecimal("26.25"),
			OtherTaxes:  fiscal.MustDecimal("3.75"),
			Total:       fiscal.MustDecimal("180"),
			Functional:  fiscal.MustDecimal("180"),
			Perceptions: fiscal.MustDecimal("3.75"),
		},
		Metadata: metadata,
	}
}

func postingMappings() map[string]accounting.AccountMapping {
	roles := []string{
		accounting.RoleCash,
		accounting.RoleReceivable,
		accounting.RoleCreditNotePayable,
		accounting.RoleRevenue,
		accounting.RoleInventory,
		accounting.RoleCOGS,
		accounting.RoleFXGain,
		accounting.RoleFXLoss,
		accounting.RoleRoundingDifference,
		"vat_payable_21",
		"vat_payable_105",
		roleTaxesPayable,
	}
	result := make(map[string]accounting.AccountMapping, len(roles))
	for _, role := range roles {
		result[role] = accounting.AccountMapping{
			Role:      role,
			AccountID: uuid.New(),
		}
	}
	return result
}

func assertFiscalTaxSide(
	t *testing.T,
	lines []accounting.JournalLine,
	mappings map[string]accounting.AccountMapping,
	debit bool,
) {
	t.Helper()
	expectations := map[string]string{
		accounting.RoleRevenue: "150",
		"vat_payable_21":       "21",
		"vat_payable_105":      "5.25",
		roleTaxesPayable:       "3.75",
	}
	for role, expected := range expectations {
		line := requireLine(t, lines, mappings[role].AccountID)
		if debit {
			assertDecimal(t, line.TransactionDebit, expected)
		} else {
			assertDecimal(t, line.TransactionCredit, expected)
		}
	}
}

func requireLine(
	t *testing.T,
	lines []accounting.JournalLine,
	accountID uuid.UUID,
) accounting.JournalLine {
	t.Helper()
	for _, line := range lines {
		if line.AccountID == accountID {
			return line
		}
	}
	t.Fatalf("line for account %s was not found", accountID)
	return accounting.JournalLine{}
}

func assertDecimal(t *testing.T, actual accounting.Decimal, expected string) {
	t.Helper()
	want := accounting.MustDecimal(expected)
	if !actual.Equal(want) {
		t.Fatalf("unexpected decimal: got %s, want %s", actual, want)
	}
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("parse fixture date: %v", err)
	}
	return parsed
}
