package migrations

import (
	"strings"
	"testing"
)

func TestIVAPeriodItemsRejectInsertAfterCloseAndNetCreditNotes(t *testing.T) {
	body, err := Files.ReadFile("000013_fiscal_iva.sql")
	if err != nil {
		t.Fatalf("read IVA migration: %v", err)
	}
	sql := string(body)

	if !strings.Contains(
		sql,
		"BEFORE INSERT OR UPDATE OR DELETE ON fiscal.iva_period_items",
	) {
		t.Fatal("IVA period item guard does not cover INSERT")
	}
	if strings.Count(sql, "IN (3, 8, 13)") < 8 {
		t.Fatal("IVA position and closing balance do not net A/B/C credit notes")
	}
	for _, required := range []string{
		"artifact bytea NOT NULL",
		"sha256 = encode(digest(artifact, 'sha256'), 'hex')",
		"OLD.status = 'exported' AND NEW.status = 'draft'",
		"fiscal_iva_periods_accounting_reconcile",
		"fiscal.iva_entry_account_effect",
		"fiscal_iva_periods_pending_authorizations",
		"entry.entry_date = voucher.issue_date",
		"entry.entry_date = purchase.issue_date",
		"fiscal_vouchers_iva_period_guard",
		"fiscal_purchase_vouchers_iva_period_guard",
		"pg_advisory_xact_lock(hashtextextended(",
		"BEFORE INSERT ON fiscal.iva_exports",
		"BEFORE UPDATE OR DELETE ON fiscal.iva_exports",
		"ALTER TABLE fiscal.%I FORCE ROW LEVEL SECURITY",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("IVA workflow migration is missing %q", required)
		}
	}
}
