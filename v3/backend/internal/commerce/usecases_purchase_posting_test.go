package commerce

import (
	"strconv"
	"strings"
	"testing"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
)

func TestBuildPurchasePostingSplitsExpenseInputVATAndPayable(t *testing.T) {
	purchase := postingTestPurchase()
	command, err := buildPurchasePostingCommand(purchase)
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	if command.EffectiveAt.Format("2006-01-02") != purchase.IssueDate {
		t.Fatalf("effective date = %s", command.EffectiveAt)
	}
	if command.ExchangeRate != "1.000000" || len(command.Lines) != 3 {
		t.Fatalf("unexpected command: %#v", command)
	}
	assertPostingLine(t, command.Lines[0], "5100", "105.00", "0.00", "105.00", false)
	assertPostingLine(t, command.Lines[1], "1300", "21.00", "0.00", "21.00", false)
	assertPostingLine(t, command.Lines[2], "2100", "0.00", "126.00", "126.00", true)
	if !command.Lines[2].OpenItem || command.Lines[2].PartyRef != purchase.SupplierRef {
		t.Fatalf("payable is not an open supplier item: %#v", command.Lines[2])
	}
}

func TestBuildPurchasePostingForeignCurrencyBalancesFunctionalResidual(t *testing.T) {
	purchase := postingTestPurchase()
	purchase.Total.Currency = "USD"
	purchase.ExchangeRate = "1025.123456"
	command, err := buildPurchasePostingCommand(purchase)
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	if command.ExchangeRate != "1025.123456" {
		t.Fatalf("exchange rate = %q", command.ExchangeRate)
	}
	var debit, credit int64
	for _, line := range command.Lines {
		value := decimalMinor(t, line.FunctionalAmount)
		if line.Debit.Amount != "0.00" {
			debit += value
		}
		if line.Credit.Amount != "0.00" {
			credit += value
		}
	}
	if debit != credit {
		t.Fatalf("functional posting does not balance: debit=%d credit=%d", debit, credit)
	}
}

func TestBuildPurchasePostingOmitsZeroInputVAT(t *testing.T) {
	purchase := postingTestPurchase()
	purchase.Total.Amount = "100.00"
	purchase.NetAmount = "0"
	purchase.ExemptAmount = "100.00"
	purchase.VATBreakdown = nil
	command, err := buildPurchasePostingCommand(purchase)
	if err != nil {
		t.Fatalf("build command: %v", err)
	}
	if len(command.Lines) != 2 || command.Lines[0].AccountCode != "5100" ||
		command.Lines[1].AccountCode != "2100" {
		t.Fatalf("unexpected zero-VAT lines: %#v", command.Lines)
	}
}

func postingTestPurchase() domain.Purchase {
	return domain.Purchase{
		ID: "purchase-1", OrganizationID: "org-1", SupplierRef: "supplier-1",
		ExternalDocumentRef: "0001-00000001", IssueDate: "2026-07-31",
		Total:     domain.Money{Amount: "126.00", Currency: "ARS"},
		NetAmount: "100.00", ExemptAmount: "5.00",
		VATBreakdown: []domain.VATBreakdownItem{{
			Rate: "21", BaseAmount: "100.00", TaxAmount: "21.00",
		}},
		SnapshotDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CorrelationID:  "purchase:purchase-1",
	}
}

func decimalMinor(t *testing.T, value string) int64 {
	t.Helper()
	parts := strings.Split(value, ".")
	if len(parts) != 2 || len(parts[1]) != 2 {
		t.Fatalf("not a fixed two-decimal value: %q", value)
	}
	result, err := strconv.ParseInt(parts[0]+parts[1], 10, 64)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return result
}
