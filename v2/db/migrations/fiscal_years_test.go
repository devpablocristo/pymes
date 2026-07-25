package migrations

import (
	"strings"
	"testing"
)

func TestFiscalYearsMigrationContainsCalendarAndCloseGuards(t *testing.T) {
	body, err := Files.ReadFile("000024_accounting_fiscal_years.sql")
	if err != nil {
		t.Fatalf("read fiscal years migration: %v", err)
	}
	sql := string(body)
	for _, required := range []string{
		"CREATE TABLE accounting.fiscal_years",
		"CREATE TABLE accounting.fiscal_year_events",
		"accounting_fiscal_years_idempotency_unique",
		"ADD COLUMN fiscal_year_id uuid",
		"ADD COLUMN period_no smallint",
		"ADD COLUMN idempotency_key text",
		"accounting_period_events_idempotency_unique",
		"ALTER COLUMN to_version SET NOT NULL",
		"'pending_drafts'",
		"completed_checks <> 7",
		"accounting_periods_close_order",
		"accounting_periods_lock_order",
		"accounting_periods_reopen_order",
		"accounting_periods_future_close",
		"accounting_periods_annual_close_pending",
		"accounting_journal_entries_annual_close_freeze",
		"accounting_journal_entries_annual_close_frozen",
		"CREATE OR REPLACE FUNCTION accounting.ensure_fiscal_year",
		"CREATE OR REPLACE FUNCTION accounting.replace_empty_fiscal_calendar",
		"ALTER TABLE accounting.fiscal_years FORCE ROW LEVEL SECURITY",
		"REVOKE DELETE ON accounting.periods FROM pymes_backend",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("migration does not contain %q", required)
		}
	}
}

func TestFiscalYearsMigrationDoesNotGrantDelete(t *testing.T) {
	body, err := Files.ReadFile("000024_accounting_fiscal_years.sql")
	if err != nil {
		t.Fatalf("read fiscal years migration: %v", err)
	}
	sql := string(body)
	for _, forbidden := range []string{
		"GRANT DELETE ON accounting.fiscal_years",
		"GRANT SELECT, INSERT, UPDATE, DELETE\n        ON accounting.fiscal_years",
	} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("migration contains forbidden grant %q", forbidden)
		}
	}
}
