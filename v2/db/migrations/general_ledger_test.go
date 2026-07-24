package migrations

import (
	"strings"
	"testing"
)

func TestGeneralLedgerMigrationAddsTenantScopedReadIndexes(t *testing.T) {
	body, err := Files.ReadFile("000022_general_ledger.sql")
	if err != nil {
		t.Fatalf("read general ledger migration: %v", err)
	}
	sql := string(body)
	for _, fragment := range []string{
		"CREATE INDEX accounting_journal_lines_general_ledger_idx",
		"org_id",
		"account_id",
		"journal_entry_id",
		"line_no",
		"CREATE INDEX accounting_journal_entries_general_ledger_idx",
		"entry_date",
		"entry_number",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("general ledger migration is missing %q", fragment)
		}
	}
}
