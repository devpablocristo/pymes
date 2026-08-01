package usecases

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
)

func TestBuildSalePostingSplitsFiscalNetVATAndTotal(t *testing.T) {
	sale := postingTestSale("FA", "ARS", "121", json.RawMessage(`{
		"issue_date":"2026-07-31",
		"currency":"ARS",
		"totals":{"net":"90","vat":"21","exempt":"10","total":"121"}
	}`))

	command, err := buildSalePostingCommand(sale, nil)
	if err != nil {
		t.Fatal(err)
	}
	if command.SourceType != "sales_invoice" || command.ExchangeRate != "1.000000" ||
		command.EffectiveAt.Format(time.DateOnly) != "2026-07-31" {
		t.Fatalf("command=%+v", command)
	}
	if len(command.Lines) != 3 {
		t.Fatalf("lines=%+v", command.Lines)
	}
	assertPostingLine(t, command.Lines[0], "1200", "121.00", "0.00", "121.00", true)
	assertPostingLine(t, command.Lines[1], "4100", "0.00", "100.00", "100.00", false)
	assertPostingLine(t, command.Lines[2], "2200", "0.00", "21.00", "21.00", false)
}

func TestBuildSalePostingOmitsZeroVATJournalLine(t *testing.T) {
	sale := postingTestSale("FC", "ARS", "120", json.RawMessage(`{
		"issue_date":"2026-07-31",
		"currency":"ARS",
		"totals":{"net":"100","vat":"0","exempt":"20","total":"120"}
	}`))
	command, err := buildSalePostingCommand(sale, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(command.Lines) != 2 {
		t.Fatalf("lines=%+v", command.Lines)
	}
	assertPostingLine(t, command.Lines[0], "1200", "120.00", "0.00", "120.00", true)
	assertPostingLine(t, command.Lines[1], "4100", "0.00", "120.00", "120.00", false)
}

func TestBuildSalePostingNormalizesForeignCurrencyAndRounding(t *testing.T) {
	for _, currency := range []string{"USD", "EUR"} {
		t.Run(currency, func(t *testing.T) {
			raw := strings.ReplaceAll(`{
				"issue_date":"2026-07-31",
				"currency":"CURRENCY",
				"exchange_rate":"1250.1234567",
				"totals":{"net":"100.005","vat":"21.006","exempt":"0","total":"121.011"}
			}`, "CURRENCY", currency)
			sale := postingTestSale("FB", currency, "121.011", json.RawMessage(raw))

			command, err := buildSalePostingCommand(sale, nil)
			if err != nil {
				t.Fatal(err)
			}
			if command.ExchangeRate != "1250.123457" {
				t.Fatalf("exchange_rate=%q", command.ExchangeRate)
			}
			assertPostingLine(t, command.Lines[0], "1200", "121.01", "0.00", "151277.44", true)
			assertPostingLine(t, command.Lines[1], "4100", "0.00", "100.00", "125012.35", false)
			assertPostingLine(t, command.Lines[2], "2200", "0.00", "21.01", "26265.09", false)
		})
	}
}

func TestBuildSalePostingRejectsInconsistentFiscalSnapshot(t *testing.T) {
	tests := []struct {
		name     string
		currency string
		total    string
		snapshot string
	}{
		{
			name: "components do not add to total", currency: "ARS", total: "121",
			snapshot: `{"issue_date":"2026-07-31","currency":"ARS","totals":{"net":"100","vat":"20","exempt":"0","total":"121"}}`,
		},
		{
			name: "sale total differs", currency: "ARS", total: "122",
			snapshot: `{"issue_date":"2026-07-31","currency":"ARS","totals":{"net":"100","vat":"21","exempt":"0","total":"121"}}`,
		},
		{
			name: "sale currency differs", currency: "USD", total: "121",
			snapshot: `{"issue_date":"2026-07-31","currency":"ARS","totals":{"net":"100","vat":"21","exempt":"0","total":"121"}}`,
		},
		{
			name: "foreign rate missing", currency: "USD", total: "121",
			snapshot: `{"issue_date":"2026-07-31","currency":"USD","totals":{"net":"100","vat":"21","exempt":"0","total":"121"}}`,
		},
		{
			name: "ARS rate is not one", currency: "ARS", total: "121",
			snapshot: `{"issue_date":"2026-07-31","currency":"ARS","exchange_rate":"2","totals":{"net":"100","vat":"21","exempt":"0","total":"121"}}`,
		},
		{
			name: "unsupported currency", currency: "GBP", total: "121",
			snapshot: `{"issue_date":"2026-07-31","currency":"GBP","exchange_rate":"1250","totals":{"net":"100","vat":"21","exempt":"0","total":"121"}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sale := postingTestSale("FA", test.currency, test.total, json.RawMessage(test.snapshot))
			if _, err := buildSalePostingCommand(sale, nil); err == nil || !strings.Contains(err.Error(), "VALIDATION_ERROR") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestBuildSalePostingLinksCreditAndDebitNotesToOriginal(t *testing.T) {
	original := postingTestSale("FA", "ARS", "121", json.RawMessage(`{
		"issue_date":"2026-07-01",
		"currency":"ARS",
		"totals":{"net":"100","vat":"21","exempt":"0","total":"121"}
	}`))
	original.ID = "sale_original"
	original.Voucher = domain.VoucherReference{PointOfSale: 3, DocumentType: "FA", VoucherNumber: 27}
	original.JournalEntryID = "journal_original"
	original.SnapshotDigest = strings.Repeat("d", 64)

	for _, test := range []struct {
		documentType string
		sourceType   string
		debit1200    string
		credit1200   string
	}{
		{documentType: "NCA", sourceType: "sales_credit_note", debit1200: "0.00", credit1200: "12.10"},
		{documentType: "NDA", sourceType: "sales_debit_note", debit1200: "12.10", credit1200: "0.00"},
	} {
		t.Run(test.documentType, func(t *testing.T) {
			note := postingTestSale(test.documentType, "ARS", "12.10", json.RawMessage(`{
				"issue_date":"2026-07-31",
				"currency":"ARS",
				"totals":{"net":"10","vat":"2.10","exempt":"0","total":"12.10"},
				"associated_voucher":{"point_of_sale":3,"document_type":"FA","voucher_number":27,"issue_date":"2026-07-01"}
			}`))
			note.SourceDocumentID = original.ID

			command, err := buildSalePostingCommand(note, &original)
			if err != nil {
				t.Fatal(err)
			}
			if command.SourceType != test.sourceType || command.OriginalJournalEntryID != original.JournalEntryID ||
				command.RelatedSource == nil || command.RelatedSource.ID != original.ID ||
				command.RelatedSource.Type != "sales_invoice" || command.RelatedSource.Digest != original.SnapshotDigest ||
				!strings.Contains(command.Description, original.ID) ||
				!strings.Contains(command.Description, original.JournalEntryID) {
				t.Fatalf("command=%+v", command)
			}
			assertPostingLine(t, command.Lines[0], "1200", test.debit1200, test.credit1200, "12.10", true)
			if test.documentType == "NCA" {
				assertPostingLine(t, command.Lines[1], "4100", "10.00", "0.00", "10.00", false)
				assertPostingLine(t, command.Lines[2], "2200", "2.10", "0.00", "2.10", false)
			} else {
				assertPostingLine(t, command.Lines[1], "4100", "0.00", "10.00", "10.00", false)
				assertPostingLine(t, command.Lines[2], "2200", "0.00", "2.10", "2.10", false)
			}
		})
	}

	note := postingTestSale("NCA", "ARS", "12.10", json.RawMessage(`{
		"issue_date":"2026-07-31",
		"currency":"ARS",
		"totals":{"net":"10","vat":"2.10","exempt":"0","total":"12.10"},
		"associated_voucher":{"point_of_sale":3,"document_type":"FA","voucher_number":999,"issue_date":"2026-07-01"}
	}`))
	note.SourceDocumentID = original.ID
	if _, err := buildSalePostingCommand(note, &original); err == nil || !strings.Contains(err.Error(), "associated voucher") {
		t.Fatalf("err=%v", err)
	}
}

func postingTestSale(documentType, currency, amount string, snapshot json.RawMessage) domain.Sale {
	return domain.Sale{
		ID:             "sale_test",
		OrganizationID: "org_test",
		RecipientRef:   "party_test",
		Voucher: domain.VoucherReference{
			PointOfSale: 1, DocumentType: documentType, VoucherNumber: 1,
		},
		Total:          domain.Money{Amount: amount, Currency: currency},
		SnapshotDigest: strings.Repeat("a", 64),
		CorrelationID:  "correlation_test",
		FiscalSnapshot: snapshot,
		CreatedAt:      time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC),
	}
}

func assertPostingLine(t *testing.T, line domain.PostingLine, account, debit, credit, functional string, openItem bool) {
	t.Helper()
	if line.AccountCode != account || line.Debit.Amount != debit || line.Credit.Amount != credit ||
		line.Debit.Currency != line.Credit.Currency || line.FunctionalAmount != functional || line.OpenItem != openItem {
		t.Fatalf("line=%+v", line)
	}
}
