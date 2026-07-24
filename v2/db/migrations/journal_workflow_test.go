package migrations

import (
	"strings"
	"testing"
)

func TestJournalWorkflowMigrationKeepsDraftsFlexibleAndAuditImmutable(t *testing.T) {
	t.Parallel()

	body, err := Files.ReadFile("000019_journal_workflow.sql")
	if err != nil {
		t.Fatalf("read journal workflow migration: %v", err)
	}
	sql := string(body)
	for _, required := range []string{
		"DROP CONSTRAINT IF EXISTS drafts_description_check",
		"ADD COLUMN reference text",
		"ADD COLUMN currency_code char(3)",
		"ADD COLUMN exchange_rate numeric(24, 10)",
		"ALTER COLUMN currency_code SET NOT NULL",
		"ALTER COLUMN exchange_rate SET NOT NULL",
		"accounting_draft_lines_historical_currency",
		"length(reference) <= 160",
		"CREATE OR REPLACE FUNCTION accounting.lock_journal_entry_period()",
		"CREATE OR REPLACE FUNCTION accounting.lock_journal_line_dependencies()",
		"CREATE OR REPLACE FUNCTION accounting.lock_reconciliation_financial_account()",
		"CREATE TRIGGER accounting_journal_entries_dependency_lock",
		"CREATE TRIGGER accounting_journal_lines_dependency_lock",
		"CREATE TRIGGER accounting_reconciliations_dependency_lock",
		"ORDER BY financial_account.id\n     FOR SHARE",
		"FOR UPDATE;\n    RETURN NEW;",
		"ADD COLUMN creation_transaction_id xid8 NOT NULL",
		"ALTER COLUMN creation_transaction_id DROP DEFAULT",
		"NEW.creation_transaction_id := pg_current_xact_id()",
		"entry.creation_transaction_id = pg_current_xact_id()",
		"accounting_journal_lines_posted_entry_immutable",
		"accounting_reconciliations_financial_account_immutable",
		"accounting_financial_accounts_ledger_account_immutable",
		"CREATE OR REPLACE FUNCTION accounting.validate_journal_workflow_invariants()",
		"accounting_journal_entries_functional_currency",
		"accounting_journal_entries_reversal_date",
		"accounting_journal_lines_exchange_rate_date",
		"CREATE INDEX accounting_drafts_reference_idx",
		"CREATE INDEX accounting_journal_entries_reference_idx",
		"WHERE status = 'discarded'",
		"discarded_at = updated_at",
		"CREATE TABLE accounting.journal_events",
		"action IN ('create', 'update', 'discard', 'post', 'reverse')",
		"before_snapshot jsonb",
		"after_snapshot jsonb NOT NULL",
		"snapshot_hash text NOT NULL",
		"CREATE OR REPLACE FUNCTION accounting.journal_snapshot_hash(",
		"public.digest(",
		"CREATE CONSTRAINT TRIGGER accounting_drafts_audit",
		"CREATE CONSTRAINT TRIGGER accounting_journal_entries_audit",
		"CREATE TRIGGER accounting_journal_events_immutable",
		"ALTER TABLE accounting.journal_events FORCE ROW LEVEL SECURITY",
		"GRANT SELECT ON accounting.journal_events TO pymes_backend",
		"accounting.financial_accounts,\n            accounting.reconciliations",
		"GRANT UPDATE (updated_at) ON\n            accounting.accounts,\n            accounting.financial_accounts\n        TO pymes_fiscal_accounting_worker",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("journal workflow SQL does not contain %q", required)
		}
	}

	if strings.Contains(sql, "GRANT DELETE ON accounting.journal_entries") ||
		strings.Contains(sql, "GRANT UPDATE ON accounting.journal_entries") {
		t.Fatal("journal workflow must not grant mutation of posted entries")
	}
	if strings.Contains(sql, "FOR SHARE OF reconciliation") {
		t.Fatal("posting must serialize on the financial account without inverse reconciliation row locks")
	}

	preflight := strings.Index(
		sql,
		"accounting_draft_lines_historical_currency",
	)
	backfill := strings.Index(sql, "UPDATE accounting.drafts AS draft")
	if preflight < 0 || backfill < 0 || preflight >= backfill {
		t.Fatal("historical draft currency preflight must run before backfill")
	}
}
