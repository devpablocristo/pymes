package migrations

import (
	"strings"
	"testing"
)

func TestTrialBalanceMigrationAddsPostedLineCoveringIndex(t *testing.T) {
	body, err := Files.ReadFile("000023_trial_balance.sql")
	if err != nil {
		t.Fatalf("read trial balance migration: %v", err)
	}
	sql := string(body)
	for _, fragment := range []string{
		"CREATE INDEX accounting_journal_lines_trial_balance_idx",
		"ON accounting.journal_lines",
		"org_id",
		"journal_entry_id",
		"account_id",
		"INCLUDE",
		"debit_amount",
		"credit_amount",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("trial balance migration is missing %q", fragment)
		}
	}
	for _, forbidden := range []string{
		"CREATE TABLE",
		"ALTER TABLE accounting.journal_entries DISABLE ROW LEVEL SECURITY",
		"ALTER TABLE accounting.journal_lines DISABLE ROW LEVEL SECURITY",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("trial balance migration contains forbidden %q", forbidden)
		}
	}
}
