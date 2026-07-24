package httpserver

import (
	"testing"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
)

func TestSettlementBookFunctionalAmountUsesHistoricalBookValue(t *testing.T) {
	t.Parallel()

	item := accounting.OpenItem{
		OpenAmount:     accounting.MustDecimal("100"),
		OpenFunctional: accounting.MustDecimal("90000"),
	}
	partial, err := settlementBookFunctionalAmount(
		item,
		accounting.MustDecimal("25"),
		accounting.MustCurrency("ARS"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if partial.String() != "22500" {
		t.Fatalf("partial book amount = %s, want 22500", partial)
	}
	full, err := settlementBookFunctionalAmount(
		item,
		accounting.MustDecimal("100"),
		accounting.MustCurrency("ARS"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if full.String() != "90000" {
		t.Fatalf("full book amount = %s, want exact remaining 90000", full)
	}
}

func TestAccountingSettlementFingerprintNormalizesExactDecimals(t *testing.T) {
	t.Parallel()

	date := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	input := api.AccountingSettlementInput{
		AccountingDate:     openapi_types.Date{Time: date},
		Amount:             "10.00",
		ExchangeRate:       "1200.000",
		ExchangeRateDate:   openapi_types.Date{Time: date},
		ExchangeRateSource: " BNA ",
		OpenItemId:         uuid.New(),
		PaymentMethod:      api.AccountingPaymentMethodBankTransfer,
	}
	first, err := accountingSettlementFingerprint(
		input,
		accounting.MustDecimal("10"),
		accounting.MustDecimal("1200"),
		"BNA",
	)
	if err != nil {
		t.Fatal(err)
	}
	input.Amount = "10.0"
	input.ExchangeRate = "1200"
	second, err := accountingSettlementFingerprint(
		input,
		accounting.MustDecimal("10"),
		accounting.MustDecimal("1200"),
		"BNA",
	)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("equivalent exact inputs produced different fingerprints")
	}
	input.PaymentMethod = api.AccountingPaymentMethodCash
	changed, err := accountingSettlementFingerprint(
		input,
		accounting.MustDecimal("10"),
		accounting.MustDecimal("1200"),
		"BNA",
	)
	if err != nil {
		t.Fatal(err)
	}
	if first == changed {
		t.Fatalf("different settlement intent reused the same fingerprint")
	}
}
