package migrations

import (
	"strings"
	"testing"
)

func TestHistoricalOpenItemBalancesExcludeLaterApplications(t *testing.T) {
	t.Parallel()

	body, err := Files.ReadFile("000011_accounting_operations.sql")
	if err != nil {
		t.Fatalf("read accounting operations migration: %v", err)
	}
	sql := string(body)

	for _, required := range []string{
		"CREATE FUNCTION accounting.open_item_balances_as_of(target_as_of date)",
		"SECURITY INVOKER",
		"settlement.entry_date AS application_date",
		"settlement.id = application.settlement_journal_entry_id",
		"WHERE item.issued_at <= target_as_of",
		"REVOKE ALL",
		"ON FUNCTION accounting.open_item_balances_as_of(date)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("historical open-item balance SQL does not contain %q", required)
		}
	}
	if count := strings.Count(
		sql,
		"application.application_date <= target_as_of",
	); count != 2 {
		t.Fatalf(
			"historical application cutoff appears %d times, want anchor and reversal arms",
			count,
		)
	}
}
