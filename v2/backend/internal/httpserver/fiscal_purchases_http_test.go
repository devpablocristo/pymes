package httpserver

import (
	"strings"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal/ar"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func TestValidateFiscalPurchaseBuildsExactImmutableSnapshot(t *testing.T) {
	t.Parallel()

	input := fiscalPurchaseFixture()
	validated, err := validateFiscalPurchase(input)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := validated.Amounts.Net.String(), "100"; got != want {
		t.Fatalf("net = %s, want %s", got, want)
	}
	if got, want := validated.Amounts.VAT.String(), "21"; got != want {
		t.Fatalf("VAT = %s, want %s", got, want)
	}
	if got, want := validated.Amounts.CreditableVAT.String(), "21"; got != want {
		t.Fatalf("creditable VAT = %s, want %s", got, want)
	}
	if got, want := validated.Amounts.Total.String(), "121"; got != want {
		t.Fatalf("total = %s, want %s", got, want)
	}
	if len(validated.SnapshotHash) != 64 ||
		!strings.Contains(validated.CanonicalJSON, `"creditable_vat":"21"`) ||
		!strings.Contains(validated.CanonicalJSON, `"source_type":"purchase"`) {
		t.Fatalf("invalid canonical snapshot: %s / %s", validated.SnapshotHash, validated.CanonicalJSON)
	}

	replayed, err := validateFiscalPurchase(input)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.CanonicalJSON != validated.CanonicalJSON ||
		replayed.SnapshotHash != validated.SnapshotHash {
		t.Fatal("equivalent purchase requests must have a stable fingerprint")
	}
}

func TestValidateFiscalPurchaseRejectsInexactTotalsAndVATBreakdown(t *testing.T) {
	t.Parallel()

	input := fiscalPurchaseFixture()
	input.Lines[0].TotalAmount = "120.99"
	if _, err := validateFiscalPurchase(input); err == nil {
		t.Fatal("expected an inexact line total to fail")
	}

	input = fiscalPurchaseFixture()
	input.Taxes[0].Amount = "20"
	if _, err := validateFiscalPurchase(input); err == nil {
		t.Fatal("expected a VAT tax breakdown mismatch to fail")
	}

	input = fiscalPurchaseFixture()
	input.Taxes[0].Kind = api.FiscalPurchaseTaxInputKindPerception
	if _, err := validateFiscalPurchase(input); err == nil {
		t.Fatal("expected a non-VAT creditable tax to fail")
	}
}

func TestValidateFiscalPurchaseSeparatesCreditableVATFromVoucherVAT(t *testing.T) {
	t.Parallel()

	input := fiscalPurchaseFixture()
	creditable := false
	input.Taxes[0].Creditable = &creditable
	input.Taxes = append(input.Taxes, api.FiscalPurchaseTaxInput{
		Kind:          api.FiscalPurchaseTaxInputKindOtherTax,
		AuthorityCode: "INT", Description: "Impuesto interno",
		TaxableBase: "100", Rate: "1", Amount: "1",
	})
	validated, err := validateFiscalPurchase(input)
	if err != nil {
		t.Fatal(err)
	}
	if !validated.Amounts.CreditableVAT.IsZero() {
		t.Fatalf("creditable VAT = %s, want zero", validated.Amounts.CreditableVAT)
	}
	if got, want := validated.Amounts.VAT.String(), "21"; got != want {
		t.Fatalf("voucher VAT = %s, want %s", got, want)
	}
	if got, want := validated.Amounts.Total.String(), "122"; got != want {
		t.Fatalf("total = %s, want %s", got, want)
	}
}

func TestReversePurchasePlanPreservesAmountsAndSwapsSides(t *testing.T) {
	t.Parallel()

	plan := accounting.PostingPlan{
		Entry: accounting.JournalEntry{
			Kind:        accounting.EntryPurchase,
			Source:      accounting.EntrySource{Event: "purchase.received"},
			Description: "Compra 00001-00000001",
			Lines: []accounting.JournalLine{{
				Debit:            accounting.MustDecimal("121"),
				TransactionDebit: accounting.MustDecimal("121"),
			}},
		},
		OpenItems: []accounting.OpenItem{{ID: uuid.New()}},
	}
	reversePurchasePlan(&plan)
	if !plan.Entry.Lines[0].Debit.IsZero() ||
		plan.Entry.Lines[0].Credit.String() != "121" ||
		plan.Entry.Lines[0].TransactionCredit.String() != "121" {
		t.Fatalf("reversed line = %#v", plan.Entry.Lines[0])
	}
	if plan.Entry.Source.Event != "purchase.credit_note.received" ||
		plan.Entry.Kind != accounting.EntryAdjustment ||
		len(plan.OpenItems) != 0 {
		t.Fatalf("reversed plan = %#v", plan)
	}
	if purchaseVoucherSign(int(ar.CreditNoteA)).
		Cmp(fiscal.NewDecimalFromInt(-1)) != 0 {
		t.Fatal("credit notes must reduce IVA totals")
	}
}

func fiscalPurchaseFixture() api.FiscalPurchaseVoucherInput {
	issueDate := openapi_types.Date{Time: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)}
	creditable := true
	return api.FiscalPurchaseVoucherInput{
		Environment:        api.FiscalEnvironmentHomologation,
		SupplierId:         uuid.MustParse("804ce2c0-8ae7-449f-8dc1-abcb7dff2058"),
		SupplierTaxId:      "30710158211",
		SupplierName:       "Proveedor SA",
		VoucherType:        api.N1,
		PointOfSale:        1,
		VoucherNumber:      42,
		IssueDate:          issueDate,
		DueDate:            &issueDate,
		Currency:           "ARS",
		ExchangeRate:       "1",
		ExchangeRateDate:   issueDate,
		ExchangeRateSource: "user",
		SourceId:           uuid.MustParse("8a0d4559-fd4e-4854-80b4-b2ec3d2574fe"),
		Lines: []api.FiscalPurchaseLineInput{{
			Description: "Mercadería",
			Quantity:    "1", UnitOfMeasure: "unit", UnitPrice: "100",
			NetAmount: "100", VatRate: "21", VatAmount: "21",
			TotalAmount:  "121",
			TaxTreatment: api.FiscalPurchaseLineInputTaxTreatmentTaxable,
		}},
		Taxes: []api.FiscalPurchaseTaxInput{{
			Kind:          api.FiscalPurchaseTaxInputKindVat,
			AuthorityCode: "IVA", Description: "IVA 21%",
			TaxableBase: "100", Rate: "21", Amount: "21",
			Creditable: &creditable,
		}},
	}
}
