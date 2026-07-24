package migrate

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestAccountWorkflowGuardsAndAudit(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	database := openDatabase(t, ctx, databaseURL)
	resetDatabase(t, ctx, database)
	ensureTestRoles(t, ctx, database)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cleanupCancel()
		resetDatabase(t, cleanupCtx, database)
		database.Close()
	})
	if err := Up(ctx, database); err != nil {
		t.Fatalf("migrate account workflow database: %v", err)
	}

	const (
		orgID     = "00000000-0000-0000-0000-00000000a901"
		groupID   = "00000000-0000-0000-0000-00000000a902"
		accountID = "00000000-0000-0000-0000-00000000a903"
		trashID   = "00000000-0000-0000-0000-00000000a904"
		draftID   = "00000000-0000-0000-0000-00000000a905"
	)
	if _, err := database.Exec(ctx, `
		INSERT INTO iam.organizations (
			id, provider, external_id, name, slug, status
		)
		VALUES (
			$1, 'clerk', 'account_workflow_test',
			'Account workflow test', 'account-workflow-test', 'provisioning'
		)
	`, orgID); err != nil {
		t.Fatalf("seed organization: %v", err)
	}

	tx := beginTenantTx(t, ctx, database.Pool(), orgID, "account-workflow-actor")
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(
		ctx,
		"SELECT accounting.install_chart_template($1, 'ar-pyme', 1)",
		orgID,
	); err != nil {
		t.Fatalf("install chart template: %v", err)
	}

	var protectedRoots int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		  FROM accounting.accounts
		 WHERE org_id = $1
		   AND parent_id IS NULL
		   AND system_key IS NOT NULL
	`, orgID).Scan(&protectedRoots); err != nil {
		t.Fatalf("count protected roots: %v", err)
	}
	if protectedRoots != 6 {
		t.Fatalf("protected roots = %d, want 6", protectedRoots)
	}
	var aliasDefinitions, aliasMappings int
	if err := tx.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM accounting.account_mapping_definitions
			  WHERE is_alias),
			(SELECT count(*) FROM accounting.account_mappings AS mapping
			  JOIN accounting.account_mapping_definitions AS definition
			    ON definition.role = mapping.mapping_key
			 WHERE mapping.org_id = $1 AND definition.is_alias)
	`, orgID).Scan(&aliasDefinitions, &aliasMappings); err != nil {
		t.Fatalf("read alias state: %v", err)
	}
	if aliasDefinitions == 0 || aliasMappings != 0 {
		t.Fatalf(
			"alias definitions/mappings = %d/%d, want definitions and no mappings",
			aliasDefinitions,
			aliasMappings,
		)
	}

	var assetRootID string
	if err := tx.QueryRow(ctx, `
		SELECT id
		  FROM accounting.accounts
		 WHERE org_id = $1 AND system_key = 'chart-root:ar-pyme:asset'
	`, orgID).Scan(&assetRootID); err != nil {
		t.Fatalf("find asset root: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.accounts (
			org_id, id, code, name, account_class, parent_id,
			normal_balance, monetary_class, posting_allowed
		)
		VALUES
			($1, $2, '1.9', 'Otros activos', 'asset', $5,
			 'debit', 'not_applicable', false),
			($1, $3, '1.9.01', 'Cuenta con uso', 'asset', $2,
			 'debit', 'monetary', true),
			($1, $4, '1.9.02', 'Cuenta descartable', 'asset', $2,
			 'debit', 'monetary', true)
	`, orgID, groupID, accountID, trashID, assetRootID); err != nil {
		t.Fatalf("create custom accounts: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE accounting.accounts
		   SET code = '1.9.10', version = version + 1
		 WHERE org_id = $1 AND id = $2
	`, orgID, accountID); err != nil {
		t.Fatalf("edit unused account structure: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.drafts (
			org_id, id, idempotency_key, entry_date, entry_kind,
			description, created_by, updated_by, currency_code, exchange_rate
		)
		VALUES (
			$1, $2, 'account-workflow-draft', DATE '2026-07-01',
			'manual', '', 'account-workflow-actor', 'account-workflow-actor',
			'ARS', 1
		)
	`, orgID, draftID); err != nil {
		t.Fatalf("seed account dependency draft: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.draft_lines (
			org_id, draft_id, line_no, account_id, debit_amount,
			credit_amount, currency_code, currency_amount, exchange_rate
		)
		VALUES ($1, $2, 1, $3, 1, 0, 'ARS', 1, 1)
	`, orgID, draftID, accountID); err != nil {
		t.Fatalf("seed account dependency line: %v", err)
	}

	assertAccountConstraint(t, ctx, tx, "accounting_accounts_structure_locked", `
		UPDATE accounting.accounts
		   SET code = '1.9.11', version = version + 1
		 WHERE org_id = $1 AND id = $2
	`, orgID, accountID)
	assertAccountConstraint(t, ctx, tx, "accounting_accounts_system_protected", `
		UPDATE accounting.accounts
		   SET name = 'Root changed', version = version + 1
		 WHERE org_id = $1 AND id = $2
	`, orgID, assetRootID)

	if _, err := tx.Exec(ctx, `
		SELECT set_config('app.accounting_reason', 'Cuenta duplicada', true)
	`); err != nil {
		t.Fatalf("set trash reason: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE accounting.accounts
		   SET trashed_at = now(), version = version + 1
		 WHERE org_id = $1 AND id = $2
	`, orgID, trashID); err != nil {
		t.Fatalf("trash unused account: %v", err)
	}
	var actor, reason, hash string
	if err := tx.QueryRow(ctx, `
		SELECT actor, reason, snapshot_hash
		  FROM accounting.account_events
		 WHERE org_id = $1 AND account_id = $2 AND action = 'trash'
	`, orgID, trashID).Scan(&actor, &reason, &hash); err != nil {
		t.Fatalf("read trash audit: %v", err)
	}
	if actor != "account-workflow-actor" || reason != "Cuenta duplicada" ||
		len(hash) != 64 {
		t.Fatalf("trash audit actor/reason/hash = %q/%q/%q", actor, reason, hash)
	}
	assertAccountConstraint(t, ctx, tx, "", `
		UPDATE accounting.account_events
		   SET reason = 'altered'
		 WHERE org_id = $1 AND account_id = $2
	`, orgID, trashID)

	var backendCanDelete bool
	if err := tx.QueryRow(ctx, `
		SELECT has_table_privilege(
			'pymes_backend', 'accounting.accounts', 'DELETE'
		)
	`).Scan(&backendCanDelete); err != nil {
		t.Fatalf("inspect account delete privilege: %v", err)
	}
	if backendCanDelete {
		t.Fatal("pymes_backend unexpectedly has DELETE on accounting.accounts")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit account workflow fixture: %v", err)
	}
}

func assertAccountConstraint(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
	constraint string,
	sql string,
	args ...any,
) {
	t.Helper()
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		t.Fatalf("begin account assertion savepoint: %v", err)
	}
	_, execErr := savepoint.Exec(ctx, sql, args...)
	if execErr == nil {
		_ = savepoint.Rollback(ctx)
		t.Fatalf("statement unexpectedly succeeded: %s", sql)
	}
	var postgresError *pgconn.PgError
	if !errors.As(execErr, &postgresError) {
		_ = savepoint.Rollback(ctx)
		t.Fatalf("statement error = %v, want PostgreSQL error", execErr)
	}
	if constraint != "" && postgresError.ConstraintName != constraint {
		_ = savepoint.Rollback(ctx)
		t.Fatalf(
			"constraint = %q, want %q (error: %v)",
			postgresError.ConstraintName,
			constraint,
			execErr,
		)
	}
	if err := savepoint.Rollback(ctx); err != nil &&
		!errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("rollback account assertion savepoint: %v", err)
	}
}
