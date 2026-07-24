package migrations

import (
	"strings"
	"testing"
)

func TestAccountWorkflowMigrationContainsSecurityAndLifecycleGuards(t *testing.T) {
	body, err := Files.ReadFile("000021_accounting_accounts_workflow.sql")
	if err != nil {
		t.Fatalf("read account workflow migration: %v", err)
	}
	sql := string(body)
	required := []string{
		"CREATE TABLE accounting.account_mapping_definitions",
		"compatible_account_classes text[]",
		"compatible_normal_balances text[]",
		"compatible_monetary_classes text[]",
		"CREATE TABLE accounting.account_events",
		"snapshot_hash",
		"ALTER TABLE accounting.account_events FORCE ROW LEVEL SECURITY",
		"CREATE TRIGGER accounting_account_events_immutable",
		"accounting_accounts_structure_locked",
		"accounting_accounts_mapping_blocks_archive",
		"accounting_accounts_financial_blocks_archive",
		"accounting_accounts_active_children",
		"accounting_accounts_parent_inactive",
		"accounting_accounts_system_protected",
		"accounting_account_mappings_incompatible",
		"REVOKE DELETE ON accounting.accounts, accounting.account_mappings",
		"DELETE FROM accounting.account_mappings AS mapping",
		"DELETE FROM accounting.chart_template_mappings AS mapping",
	}
	for _, fragment := range required {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("account workflow migration is missing %q", fragment)
		}
	}
}
