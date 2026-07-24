package migrate

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestJournalPostingDependencyLocks(t *testing.T) {
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
		t.Fatalf("migrate dependency-lock database: %v", err)
	}

	const (
		orgID           = "00000000-0000-0000-0000-00000000c901"
		userID          = "integration-lock-test"
		periodID        = "00000000-0000-0000-0000-00000000c902"
		archiveAccount  = "00000000-0000-0000-0000-00000000c903"
		cashAccount     = "00000000-0000-0000-0000-00000000c904"
		otherCash       = "00000000-0000-0000-0000-00000000c905"
		revenueAccount  = "00000000-0000-0000-0000-00000000c906"
		financialID     = "00000000-0000-0000-0000-00000000c907"
		otherFinancial  = "00000000-0000-0000-0000-00000000c908"
		reconciliation  = "00000000-0000-0000-0000-00000000c909"
		committedEntry  = "00000000-0000-0000-0000-00000000c910"
		periodRaceEntry = "00000000-0000-0000-0000-00000000c911"
		archiveRace     = "00000000-0000-0000-0000-00000000c912"
		archiveRetry    = "00000000-0000-0000-0000-00000000c913"
		reconcileRace   = "00000000-0000-0000-0000-00000000c914"
		reconcileRetry  = "00000000-0000-0000-0000-00000000c915"
		assetGroup      = "00000000-0000-0000-0000-00000000c916"
		revenueGroup    = "00000000-0000-0000-0000-00000000c917"
	)
	if _, err := database.Exec(ctx, `
		INSERT INTO iam.organizations (
			id, provider, external_id, name, slug, status
		)
		VALUES (
			$1, 'clerk', 'journal_lock_test', 'Journal lock test',
			'journal-lock-test', 'provisioning'
		)
	`, orgID); err != nil {
		t.Fatalf("seed lock-test organization: %v", err)
	}

	seedTx := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = seedTx.Rollback(context.Background()) }()
	if _, err := seedTx.Exec(ctx, `
		INSERT INTO accounting.organization_settings (
			org_id, country_code, functional_currency
		)
		VALUES ($1, 'AR', 'ARS')
	`, orgID); err != nil {
		t.Fatalf("seed posting dependency settings: %v", err)
	}
	if _, err := seedTx.Exec(ctx, `
		INSERT INTO accounting.accounts (
			org_id, id, code, name, account_class, normal_balance,
			monetary_class, posting_allowed, parent_id
		)
		VALUES
			($1, $6, '1.1', 'Asset group', 'asset', 'debit', 'not_applicable', false, NULL),
			($1, $7, '4.9', 'Revenue group', 'revenue', 'credit', 'not_applicable', false, NULL),
			($1, $2, '1.1.90', 'Archive race', 'asset', 'debit', 'monetary', true, $6),
			($1, $3, '1.1.91', 'Cash race', 'asset', 'debit', 'monetary', true, $6),
			($1, $4, '1.1.92', 'Other cash', 'asset', 'debit', 'monetary', true, $6),
			($1, $5, '4.9.90', 'Lock revenue', 'revenue', 'credit', 'monetary', true, $7)
	`, orgID, archiveAccount, cashAccount, otherCash, revenueAccount,
		assetGroup, revenueGroup); err != nil {
		t.Fatalf("seed posting dependency accounts: %v", err)
	}
	if _, err := seedTx.Exec(ctx, `
		INSERT INTO accounting.periods (
			org_id, id, code, start_date, end_date
		)
		VALUES (
			$1, $2, '2026-01', DATE '2026-01-01', DATE '2026-01-31'
		)
	`, orgID, periodID); err != nil {
		t.Fatalf("seed posting dependency period: %v", err)
	}
	if _, err := seedTx.Exec(ctx, `
		INSERT INTO accounting.financial_accounts (
			org_id, id, ledger_account_id, account_type, name, currency_code
		)
		VALUES
			($1, $2, $3, 'cash', 'Cash lock resource', 'ARS'),
			($1, $4, $5, 'cash', 'Other lock resource', 'ARS')
	`, orgID, financialID, cashAccount, otherFinancial, otherCash); err != nil {
		t.Fatalf("seed posting dependency financial accounts: %v", err)
	}
	if _, err := seedTx.Exec(ctx, `
		INSERT INTO accounting.reconciliations (
			org_id, id, financial_account_id, start_date, end_date,
			opening_balance, closing_balance, created_by
		)
		VALUES (
			$1, $2, $3, DATE '2026-01-01', DATE '2026-01-31',
			0, 0, $4
		)
	`,
		orgID,
		reconciliation,
		financialID,
		userID,
	); err != nil {
		t.Fatalf("seed posting dependency reconciliation: %v", err)
	}
	if err := seedTx.Commit(ctx); err != nil {
		t.Fatalf("commit posting dependency fixtures: %v", err)
	}

	assertPeriodDependencyLock(
		t,
		ctx,
		database,
		orgID,
		userID,
		periodID,
		periodRaceEntry,
	)
	assertAccountLifecycleDependencyLock(
		t,
		ctx,
		database,
		orgID,
		userID,
		periodID,
		archiveAccount,
		revenueAccount,
		archiveRace,
		archiveRetry,
	)
	assertReconciliationDependencyLock(
		t,
		ctx,
		database,
		orgID,
		userID,
		periodID,
		cashAccount,
		revenueAccount,
		reconciliation,
		reconcileRace,
		reconcileRetry,
	)
	assertJournalLinesCannotBeAppendedAfterCommit(
		t,
		ctx,
		database,
		orgID,
		userID,
		periodID,
		archiveAccount,
		revenueAccount,
		committedEntry,
	)
	assertFinancialLinksAreImmutable(
		t,
		ctx,
		database,
		orgID,
		userID,
		financialID,
		otherFinancial,
		otherCash,
		reconciliation,
	)
}

func assertPeriodDependencyLock(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
	orgID string,
	userID string,
	periodID string,
	entryID string,
) {
	t.Helper()
	holder := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = holder.Rollback(context.Background()) }()
	var lockedPeriod string
	if err := holder.QueryRow(ctx, `
		SELECT id
		  FROM accounting.periods
		 WHERE org_id = $1
		   AND id = $2
		 FOR UPDATE
	`, orgID, periodID).Scan(&lockedPeriod); err != nil {
		t.Fatalf("hold period transition lock: %v", err)
	}

	contender := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = contender.Rollback(context.Background()) }()
	setShortLockTimeout(t, ctx, contender)
	err := insertLockTestEntry(
		ctx,
		contender,
		orgID,
		entryID,
		periodID,
		"period-lock-race",
	)
	assertPostgresCode(t, err, "55P03", "")
}

func assertAccountLifecycleDependencyLock(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
	orgID string,
	userID string,
	periodID string,
	accountID string,
	counterpartID string,
	raceEntryID string,
	retryEntryID string,
) {
	t.Helper()
	holder := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = holder.Rollback(context.Background()) }()
	if _, err := holder.Exec(ctx, `
		UPDATE accounting.accounts
		   SET archived_at = now(),
		       version = version + 1,
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
	`, orgID, accountID); err != nil {
		t.Fatalf("hold account archive lock: %v", err)
	}

	contender := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = contender.Rollback(context.Background()) }()
	setShortLockTimeout(t, ctx, contender)
	if err := insertLockTestEntry(
		ctx,
		contender,
		orgID,
		raceEntryID,
		periodID,
		"account-lock-race",
	); err != nil {
		t.Fatalf("insert account-race entry: %v", err)
	}
	err := insertLockTestLine(
		ctx,
		contender,
		orgID,
		raceEntryID,
		1,
		accountID,
		"1",
		"0",
	)
	assertPostgresCode(t, err, "55P03", "")
	_ = contender.Rollback(ctx)

	if err := holder.Commit(ctx); err != nil {
		t.Fatalf("commit account archive winner: %v", err)
	}

	retry := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = retry.Rollback(context.Background()) }()
	if err := insertLockTestEntry(
		ctx,
		retry,
		orgID,
		retryEntryID,
		periodID,
		"account-lock-retry",
	); err != nil {
		t.Fatalf("insert account retry entry: %v", err)
	}
	if err := insertLockTestLine(
		ctx,
		retry,
		orgID,
		retryEntryID,
		1,
		accountID,
		"1",
		"0",
	); err != nil {
		t.Fatalf("insert archived account retry line: %v", err)
	}
	if err := insertLockTestLine(
		ctx,
		retry,
		orgID,
		retryEntryID,
		2,
		counterpartID,
		"0",
		"1",
	); err != nil {
		t.Fatalf("insert account retry counterpart: %v", err)
	}
	_, err = retry.Exec(ctx, `
		SET CONSTRAINTS
			accounting.accounting_journal_entries_valid
		IMMEDIATE
	`)
	assertPostgresCode(
		t,
		err,
		"23514",
		"accounting_journal_lines_active_posting_account",
	)
	_ = retry.Rollback(ctx)

	restore := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = restore.Rollback(context.Background()) }()
	if _, err := restore.Exec(ctx, `
		UPDATE accounting.accounts
		   SET archived_at = NULL,
		       version = version + 1,
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
	`, orgID, accountID); err != nil {
		t.Fatalf("restore account after lock assertion: %v", err)
	}
	if err := restore.Commit(ctx); err != nil {
		t.Fatalf("commit account restore: %v", err)
	}
}

func assertReconciliationDependencyLock(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
	orgID string,
	userID string,
	periodID string,
	accountID string,
	counterpartID string,
	reconciliationID string,
	raceEntryID string,
	retryEntryID string,
) {
	t.Helper()
	holder := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = holder.Rollback(context.Background()) }()
	if _, err := holder.Exec(ctx, `
		UPDATE accounting.reconciliations
		   SET status = 'closed',
		       version = version + 1,
		       status_changed_by = $3,
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
	`, orgID, reconciliationID, userID); err != nil {
		t.Fatalf("hold reconciliation close lock: %v", err)
	}

	contender := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = contender.Rollback(context.Background()) }()
	setShortLockTimeout(t, ctx, contender)
	if err := insertLockTestEntry(
		ctx,
		contender,
		orgID,
		raceEntryID,
		periodID,
		"reconciliation-lock-race",
	); err != nil {
		t.Fatalf("insert reconciliation-race entry: %v", err)
	}
	err := insertLockTestLine(
		ctx,
		contender,
		orgID,
		raceEntryID,
		1,
		accountID,
		"1",
		"0",
	)
	assertPostgresCode(t, err, "55P03", "")
	_ = contender.Rollback(ctx)

	if err := holder.Commit(ctx); err != nil {
		t.Fatalf("commit reconciliation close winner: %v", err)
	}

	retry := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = retry.Rollback(context.Background()) }()
	if err := insertLockTestEntry(
		ctx,
		retry,
		orgID,
		retryEntryID,
		periodID,
		"reconciliation-lock-retry",
	); err != nil {
		t.Fatalf("insert reconciliation retry entry: %v", err)
	}
	if err := insertLockTestLine(
		ctx,
		retry,
		orgID,
		retryEntryID,
		1,
		accountID,
		"1",
		"0",
	); err != nil {
		t.Fatalf("insert closed reconciliation retry line: %v", err)
	}
	if err := insertLockTestLine(
		ctx,
		retry,
		orgID,
		retryEntryID,
		2,
		counterpartID,
		"0",
		"1",
	); err != nil {
		t.Fatalf("insert reconciliation retry counterpart: %v", err)
	}
	_, err = retry.Exec(ctx, `
		SET CONSTRAINTS
			accounting.accounting_journal_lines_closed_reconciliation
		IMMEDIATE
	`)
	assertPostgresCode(
		t,
		err,
		"23514",
		"accounting_journal_lines_closed_reconciliation",
	)

}

func assertJournalLinesCannotBeAppendedAfterCommit(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
	orgID string,
	userID string,
	periodID string,
	accountID string,
	counterpartID string,
	entryID string,
) {
	t.Helper()
	create := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = create.Rollback(context.Background()) }()
	savepoint, err := create.Begin(ctx)
	if err != nil {
		t.Fatalf("begin journal creation savepoint: %v", err)
	}
	defer func() { _ = savepoint.Rollback(context.Background()) }()
	if err := insertLockTestEntry(
		ctx,
		savepoint,
		orgID,
		entryID,
		periodID,
		"committed-entry",
	); err != nil {
		t.Fatalf("insert committed entry: %v", err)
	}
	if err := insertLockTestLine(
		ctx, savepoint, orgID, entryID, 1, accountID, "1", "0",
	); err != nil {
		t.Fatalf("insert committed debit: %v", err)
	}
	if err := insertLockTestLine(
		ctx, savepoint, orgID, entryID, 2, counterpartID, "0", "1",
	); err != nil {
		t.Fatalf("insert committed credit: %v", err)
	}
	if err := savepoint.Commit(ctx); err != nil {
		t.Fatalf("release journal creation savepoint: %v", err)
	}
	if err := create.Commit(ctx); err != nil {
		t.Fatalf("commit immutable entry fixture: %v", err)
	}

	appendTx := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = appendTx.Rollback(context.Background()) }()
	err = insertLockTestLine(
		ctx,
		appendTx,
		orgID,
		entryID,
		3,
		accountID,
		"1",
		"0",
	)
	assertPostgresCode(
		t,
		err,
		"55000",
		"accounting_journal_lines_posted_entry_immutable",
	)
}

func assertFinancialLinksAreImmutable(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
	orgID string,
	userID string,
	financialAccountID string,
	otherFinancialID string,
	otherLedgerAccountID string,
	reconciliationID string,
) {
	t.Helper()
	financialTx := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = financialTx.Rollback(context.Background()) }()
	_, err := financialTx.Exec(ctx, `
		UPDATE accounting.financial_accounts
		   SET ledger_account_id = $3
		 WHERE org_id = $1
		   AND id = $2
	`, orgID, financialAccountID, otherLedgerAccountID)
	assertPostgresCode(
		t,
		err,
		"55000",
		"accounting_financial_accounts_ledger_account_immutable",
	)

	reconciliationTx := beginTenantTx(t, ctx, database.Pool(), orgID, userID)
	defer func() { _ = reconciliationTx.Rollback(context.Background()) }()
	_, err = reconciliationTx.Exec(ctx, `
		UPDATE accounting.reconciliations
		   SET financial_account_id = $3,
		       version = version + 1
		 WHERE org_id = $1
		   AND id = $2
	`, orgID, reconciliationID, otherFinancialID)
	assertPostgresCode(
		t,
		err,
		"55000",
		"accounting_reconciliations_financial_account_immutable",
	)
}

func setShortLockTimeout(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout = '150ms'"); err != nil {
		t.Fatalf("set short lock timeout: %v", err)
	}
}

func insertLockTestEntry(
	ctx context.Context,
	tx pgx.Tx,
	orgID string,
	entryID string,
	periodID string,
	idempotencyKey string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO accounting.journal_entries (
			org_id, id, entry_date, period_id, entry_kind, description,
			functional_currency, source_type, source_id, posting_kind,
			idempotency_key, created_by
		)
		VALUES (
			$1, $2, DATE '2026-01-10', $3, 'manual', 'Lock test',
			'ARS', 'integration', $4, 'primary', $4, 'integration-lock-test'
		)
	`, orgID, entryID, periodID, idempotencyKey)
	return err
}

func insertLockTestLine(
	ctx context.Context,
	tx pgx.Tx,
	orgID string,
	entryID string,
	lineNo int,
	accountID string,
	debit string,
	credit string,
) error {
	amount := debit
	if amount == "0" {
		amount = credit
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO accounting.journal_lines (
			org_id, journal_entry_id, line_no, account_id,
			debit_amount, credit_amount, currency_code, currency_amount,
			exchange_rate
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'ARS', $7, 1)
	`, orgID, entryID, lineNo, accountID, debit, credit, amount)
	return err
}

func assertPostgresCode(
	t *testing.T,
	err error,
	code string,
	constraint string,
) {
	t.Helper()
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		t.Fatalf("error = %v, want PostgreSQL %s", err, code)
	}
	if postgresError.Code != code {
		t.Fatalf(
			"PostgreSQL code = %s, want %s (error %v)",
			postgresError.Code,
			code,
			err,
		)
	}
	if constraint != "" && postgresError.ConstraintName != constraint {
		t.Fatalf(
			"PostgreSQL constraint = %q, want %q",
			postgresError.ConstraintName,
			constraint,
		)
	}
}
