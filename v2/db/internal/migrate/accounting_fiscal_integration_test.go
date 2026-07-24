package migrate

import (
	"context"
	"errors"
	"testing"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func assertAccountingAndFiscalInvariants(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
	databaseURL string,
) {
	t.Helper()

	const (
		orgA               = "00000000-0000-0000-0000-00000000a001"
		orgB               = "00000000-0000-0000-0000-00000000b001"
		userA              = "00000000-0000-0000-0000-00000000a101"
		membershipA        = "00000000-0000-0000-0000-00000000a201"
		periodID           = "00000000-0000-0000-0000-00000000d001"
		februaryPeriodID   = "00000000-0000-0000-0000-00000000d002"
		marchPeriodID      = "00000000-0000-0000-0000-00000000d003"
		entryID            = "00000000-0000-0000-0000-00000000d101"
		reversalID         = "00000000-0000-0000-0000-00000000d102"
		unbalanced         = "00000000-0000-0000-0000-00000000d103"
		lockedEntry        = "00000000-0000-0000-0000-00000000d104"
		settlementEntry    = "00000000-0000-0000-0000-00000000d105"
		settlementUndo     = "00000000-0000-0000-0000-00000000d106"
		openItemID         = "00000000-0000-0000-0000-00000000d401"
		applicationID      = "00000000-0000-0000-0000-00000000d402"
		applicationUndo    = "00000000-0000-0000-0000-00000000d403"
		inflationRunID     = "00000000-0000-0000-0000-00000000d201"
		revaluationRunID   = "00000000-0000-0000-0000-00000000d301"
		pointID            = "00000000-0000-0000-0000-00000000e001"
		voucherID          = "00000000-0000-0000-0000-00000000e101"
		snapshotID         = "00000000-0000-0000-0000-00000000e201"
		purchaseID         = "00000000-0000-0000-0000-00000000e301"
		ivaPeriodID        = "00000000-0000-0000-0000-00000000e401"
		ivaSalesEntryID    = "00000000-0000-0000-0000-00000000e501"
		ivaPurchaseEntryID = "00000000-0000-0000-0000-00000000e502"
		ivaSalesReversalID = "00000000-0000-0000-0000-00000000e503"
		lateVoucherID      = "00000000-0000-0000-0000-00000000e601"
	)

	assertTenantTablesForceRLS(t, ctx, database)
	assertNoFloatingPointMoney(t, ctx, database)
	assertPendingOrganizationsPrivileges(t, ctx, database)
	assertFiscalWorkerRoutinePrivileges(t, ctx, database)
	assertHomologationEvidenceInvariants(
		t, ctx, database, databaseURL, orgA, orgB, userA,
	)

	backend := openBackendPool(t, ctx, databaseURL)
	defer backend.Close()
	fiscalWorker := openFiscalWorkerPool(t, ctx, databaseURL)
	defer fiscalWorker.Close()
	fiscalAccountingWorker := openFiscalAccountingWorkerPool(
		t,
		ctx,
		databaseURL,
	)
	defer fiscalAccountingWorker.Close()

	tx := beginTenantTx(t, ctx, backend, orgA, userA)
	var installedAccounts int
	if err := tx.QueryRow(
		ctx,
		`SELECT accounting.install_chart_template($1, 'ar-pyme', 1)`,
		orgA,
	).Scan(&installedAccounts); err != nil {
		t.Fatalf("install accounting chart: %v", err)
	}
	if installedAccounts == 0 {
		t.Fatal("chart template installed no accounts")
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.periods (
			org_id, id, code, start_date, end_date
		)
		VALUES
			($1, $2, '2026-01', DATE '2026-01-01', DATE '2026-01-31'),
			($1, $3, '2026-02', DATE '2026-02-01', DATE '2026-02-28'),
			($1, $4, '2026-03', DATE '2026-03-01', DATE '2026-03-31')
	`, orgA, periodID, februaryPeriodID, marchPeriodID); err != nil {
		t.Fatalf("insert accounting periods: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO iam.membership_permissions (
			org_id, membership_id, permission, granted_by
		)
		VALUES ($1, $2, 'accounting:manage', 'integration-test')
	`, orgA, membershipA); err != nil {
		t.Fatalf("grant accounting permission: %v", err)
	}

	var (
		receivableAccountID, revenueAccountID, cashAccountID string
		vatPayableAccountID, vatCreditAccountID              string
	)
	if err := tx.QueryRow(ctx, `
		SELECT
			max(mapping.account_id::text)
				FILTER (WHERE mapping.mapping_key = 'receivable'),
			max(mapping.account_id::text)
				FILTER (WHERE mapping.mapping_key = 'revenue'),
			max(mapping.account_id::text)
				FILTER (WHERE mapping.mapping_key = 'cash'),
			max(mapping.account_id::text)
				FILTER (WHERE mapping.mapping_key = 'vat_payable_21'),
			max(mapping.account_id::text)
				FILTER (WHERE mapping.mapping_key = 'vat_credit_21')
		FROM accounting.account_mappings AS mapping
	`).Scan(
		&receivableAccountID,
		&revenueAccountID,
		&cashAccountID,
		&vatPayableAccountID,
		&vatCreditAccountID,
	); err != nil {
		t.Fatalf("resolve accounting mappings: %v", err)
	}
	assertTxCount(
		t,
		ctx,
		tx,
		`SELECT count(*)
		   FROM accounting.account_mappings
		  WHERE mapping_key = ANY(ARRAY[
		      'vat_payable_21',
		      'vat_payable_105',
		      'vat_payable_27',
		      'vat_payable_5',
		      'vat_payable_25'
		  ]::text[])`,
		5,
	)

	insertJournalEntry(
		t,
		ctx,
		tx,
		orgA,
		entryID,
		periodID,
		"sale",
		"integration-sale",
		"",
	)
	insertJournalLine(
		t, ctx, tx, orgA, entryID, 1, receivableAccountID, "121.00", "0",
	)
	insertJournalLine(
		t, ctx, tx, orgA, entryID, 2, revenueAccountID, "0", "121.00",
	)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit balanced journal entry: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	insertJournalEntryAt(
		t,
		ctx,
		tx,
		orgA,
		settlementEntry,
		februaryPeriodID,
		"2026-02-10",
		"collection",
		"integration-collection",
		"",
	)
	insertJournalLine(
		t, ctx, tx, orgA, settlementEntry, 1, cashAccountID, "121.00", "0",
	)
	insertJournalLine(
		t, ctx, tx, orgA, settlementEntry, 2, receivableAccountID, "0", "121.00",
	)
	insertJournalEntryAt(
		t,
		ctx,
		tx,
		orgA,
		settlementUndo,
		marchPeriodID,
		"2026-03-10",
		"refund",
		"integration-collection-undo",
		"",
	)
	insertJournalLine(
		t, ctx, tx, orgA, settlementUndo, 1, receivableAccountID, "121.00", "0",
	)
	insertJournalLine(
		t, ctx, tx, orgA, settlementUndo, 2, cashAccountID, "0", "121.00",
	)
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.open_items (
			org_id,
			id,
			item_type,
			party_type,
			party_id,
			account_id,
			origin_journal_entry_id,
			origin_journal_line_id,
			document_type,
			document_id,
			currency_code,
			original_currency_amount,
			original_functional_amount,
			issued_at,
			due_date
		)
		SELECT
			line.org_id,
			$1,
			'receivable',
			'party',
			'integration-customer',
			line.account_id,
			line.journal_entry_id,
			line.id,
			'sale',
			'integration-sale',
			'ARS',
			121,
			121,
			DATE '2026-01-10',
			DATE '2026-01-20'
		  FROM accounting.journal_lines AS line
		 WHERE line.org_id = $2
		   AND line.journal_entry_id = $3
		   AND line.line_no = 1
	`, openItemID, orgA, entryID); err != nil {
		t.Fatalf("insert historical-aging open item: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.open_item_applications (
			org_id,
			id,
			open_item_id,
			settlement_journal_entry_id,
			settlement_journal_line_id,
			currency_amount,
			functional_amount,
			exchange_difference_amount,
			applied_by,
			applied_at
		)
		SELECT
			line.org_id,
			$1,
			$2,
			line.journal_entry_id,
			line.id,
			121,
			121,
			0,
			'integration-test',
			TIMESTAMPTZ '2026-02-10 12:00:00+00'
		  FROM accounting.journal_lines AS line
		 WHERE line.org_id = $3
		   AND line.journal_entry_id = $4
		   AND line.line_no = 2
	`, applicationID, openItemID, orgA, settlementEntry); err != nil {
		t.Fatalf("insert future open-item application: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.open_item_applications (
			org_id,
			id,
			open_item_id,
			settlement_journal_entry_id,
			settlement_journal_line_id,
			currency_amount,
			functional_amount,
			exchange_difference_amount,
			reverses_application_id,
			applied_by,
			applied_at
		)
		SELECT
			line.org_id,
			$1,
			$2,
			line.journal_entry_id,
			line.id,
			121,
			121,
			0,
			$3,
			'integration-test',
			TIMESTAMPTZ '2026-03-10 12:00:00+00'
		  FROM accounting.journal_lines AS line
		 WHERE line.org_id = $4
		   AND line.journal_entry_id = $5
		   AND line.line_no = 1
	`, applicationUndo, openItemID, applicationID, orgA, settlementUndo); err != nil {
		t.Fatalf("insert future open-item application reversal: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit historical-aging fixture: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	var beforeApplication, afterApplication, afterReversal bool
	if err := tx.QueryRow(ctx, `
		SELECT remaining_functional_amount = 121
		  FROM accounting.open_item_balances_as_of(DATE '2026-01-31')
		 WHERE open_item_id = $1
	`, openItemID).Scan(&beforeApplication); err != nil {
		t.Fatalf("read historical open-item balance: %v", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT remaining_functional_amount = 0
		  FROM accounting.open_item_balances_as_of(DATE '2026-02-28')
		 WHERE open_item_id = $1
	`, openItemID).Scan(&afterApplication); err != nil {
		t.Fatalf("read settled open-item balance: %v", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT remaining_functional_amount = 121
		  FROM accounting.open_item_balances_as_of(DATE '2026-03-31')
		 WHERE open_item_id = $1
	`, openItemID).Scan(&afterReversal); err != nil {
		t.Fatalf("read restored open-item balance: %v", err)
	}
	if !beforeApplication || !afterApplication || !afterReversal {
		t.Fatalf(
			"historical balance cutoff failed: before=%t after=%t reversed=%t",
			beforeApplication,
			afterApplication,
			afterReversal,
		)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit historical-aging assertions: %v", err)
	}

	var originalEntryJSON string
	if err := database.QueryRow(
		ctx,
		`SELECT to_jsonb(entry)::text
		   FROM accounting.journal_entries AS entry
		  WHERE org_id = $1 AND id = $2`,
		orgA,
		entryID,
	).Scan(&originalEntryJSON); err != nil {
		t.Fatalf("read original journal entry: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	insertJournalEntry(
		t,
		ctx,
		tx,
		orgA,
		reversalID,
		periodID,
		"reversal",
		"integration-reversal",
		entryID,
	)
	insertJournalLine(
		t, ctx, tx, orgA, reversalID, 1, receivableAccountID, "0", "121.00",
	)
	insertJournalLine(
		t, ctx, tx, orgA, reversalID, 2, revenueAccountID, "121.00", "0",
	)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit exact reversal: %v", err)
	}

	var originalEntryAfterReversal string
	if err := database.QueryRow(
		ctx,
		`SELECT to_jsonb(entry)::text
		   FROM accounting.journal_entries AS entry
		  WHERE org_id = $1 AND id = $2`,
		orgA,
		entryID,
	).Scan(&originalEntryAfterReversal); err != nil {
		t.Fatalf("read original after reversal: %v", err)
	}
	if originalEntryAfterReversal != originalEntryJSON {
		t.Fatal("reversal modified the original journal entry")
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	insertJournalEntry(
		t,
		ctx,
		tx,
		orgA,
		unbalanced,
		periodID,
		"manual",
		"unbalanced-entry",
		"",
	)
	insertJournalLine(
		t, ctx, tx, orgA, unbalanced, 1, receivableAccountID, "10.00", "0",
	)
	insertJournalLine(
		t, ctx, tx, orgA, unbalanced, 2, revenueAccountID, "0", "9.00",
	)
	if err := tx.Commit(ctx); err == nil {
		t.Fatal("unbalanced journal entry committed")
	}
	var unbalancedCount int
	if err := database.QueryRow(
		ctx,
		"SELECT count(*) FROM accounting.journal_entries WHERE org_id = $1 AND id = $2",
		orgA,
		unbalanced,
	).Scan(&unbalancedCount); err != nil {
		t.Fatalf("count rolled-back journal entry: %v", err)
	}
	if unbalancedCount != 0 {
		t.Fatalf("rolled-back journal entry count = %d, want 0", unbalancedCount)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(
		ctx,
		`UPDATE accounting.journal_entries
		    SET description = 'mutated'
		  WHERE org_id = $1 AND id = $2`,
		orgA,
		entryID,
	); err == nil {
		t.Fatal("posted journal entry accepted an update")
	}
	_ = tx.Rollback(ctx)

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	for _, checkKey := range []string{
		"unposted_documents",
		"fiscal_pending",
		"posting_errors",
		"account_mappings",
		"exchange_rates",
		"unreconciled_accounts",
	} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO accounting.period_close_checks (
				org_id, period_id, check_key, status, checked_by
			)
			VALUES ($1, $2, $3, 'passed', 'integration-test')
		`, orgA, periodID, checkKey); err != nil {
			t.Fatalf("insert period check %s: %v", checkKey, err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE accounting.periods
		   SET status = 'soft_closed',
		       version = version + 1,
		       status_changed_by = 'integration-test',
		       updated_at = now()
		 WHERE org_id = $1 AND id = $2
	`, orgA, periodID); err != nil {
		t.Fatalf("soft-close period: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE accounting.periods
		   SET status = 'locked',
		       version = version + 1,
		       status_changed_by = 'integration-test',
		       updated_at = now()
		 WHERE org_id = $1 AND id = $2
	`, orgA, periodID); err != nil {
		t.Fatalf("lock period: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit period close: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	insertJournalEntry(
		t,
		ctx,
		tx,
		orgA,
		lockedEntry,
		periodID,
		"adjustment",
		"locked-period-entry",
		"",
	)
	insertJournalLine(
		t, ctx, tx, orgA, lockedEntry, 1, receivableAccountID, "1.00", "0",
	)
	insertJournalLine(
		t, ctx, tx, orgA, lockedEntry, 2, revenueAccountID, "0", "1.00",
	)
	if err := tx.Commit(ctx); err == nil {
		t.Fatal("locked period accepted a journal entry")
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(ctx, `
		UPDATE accounting.periods
		   SET status = 'open',
		       version = version + 1,
		       status_changed_by = 'integration-test',
		       transition_reason = 'fixture reopen',
		       updated_at = now()
		 WHERE org_id = $1 AND id = $2
	`, orgA, periodID); err != nil {
		t.Fatalf("reopen locked period: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit period reopen: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.inflation_runs (
			org_id,
			id,
			period_id,
			series_code,
			source_checksum,
			created_by
		)
		VALUES ($1, $2, $3, 'FACPCE', repeat('c', 64), 'integration-test')
	`, orgA, inflationRunID, periodID); err != nil {
		t.Fatalf("insert inflation run: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.inflation_run_lines (
			org_id,
			inflation_run_id,
			line_no,
			account_id,
			origin_date,
			monetary_class,
			original_amount,
			origin_index,
			closing_index,
			coefficient,
			adjusted_amount,
			adjustment_amount
		)
		VALUES (
			$1, $2, 1, $3, DATE '2025-12-01', 'non_monetary',
			10, 100, 123.4567, 1.234567, 12.35, 2.35
		)
	`, orgA, inflationRunID, receivableAccountID); err != nil {
		t.Fatalf("insert functional-currency-rounded inflation line: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.currency_revaluation_runs (
			org_id,
			id,
			period_id,
			revaluation_date,
			source_checksum,
			created_by
		)
		VALUES (
			$1, $2, $3, DATE '2026-01-31',
			repeat('d', 64), 'integration-test'
		)
	`, orgA, revaluationRunID, periodID); err != nil {
		t.Fatalf("insert currency revaluation run: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.currency_revaluation_lines (
			org_id,
			revaluation_run_id,
			line_no,
			account_id,
			currency_code,
			currency_amount,
			carrying_amount,
			closing_rate,
			revalued_amount,
			exchange_difference_amount
		)
		VALUES (
			$1, $2, 1, $3, 'USD', 10, 12, 1.234567, 12.35, 0.35
		)
	`, orgA, revaluationRunID, receivableAccountID); err != nil {
		t.Fatalf("insert functional-currency-rounded revaluation line: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit functional-currency rounded calculations: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.inflation_run_lines (
			org_id,
			inflation_run_id,
			line_no,
			account_id,
			origin_date,
			monetary_class,
			original_amount,
			origin_index,
			closing_index,
			coefficient,
			adjusted_amount,
			adjustment_amount
		)
		VALUES (
			$1, $2, 2, $3, DATE '2025-12-01', 'non_monetary',
			10, 100, 123.4567, 1.234567, 12.345670, 2.345670
		)
	`, orgA, inflationRunID, receivableAccountID); err == nil {
		t.Fatal("inflation line accepted six-decimal instead of currency rounding")
	}
	_ = tx.Rollback(ctx)

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.currency_revaluation_lines (
			org_id,
			revaluation_run_id,
			line_no,
			account_id,
			currency_code,
			currency_amount,
			carrying_amount,
			closing_rate,
			revalued_amount,
			exchange_difference_amount
		)
		VALUES (
			$1, $2, 2, $3, 'USD',
			10, 12, 1.234567, 12.345670, 0.345670
		)
	`, orgA, revaluationRunID, receivableAccountID); err == nil {
		t.Fatal("revaluation line accepted six-decimal instead of currency rounding")
	}
	_ = tx.Rollback(ctx)

	tx = beginTenantTx(t, ctx, backend, orgB, userA)
	assertTxCount(t, ctx, tx, "SELECT count(*) FROM accounting.accounts", 0)
	assertTxCount(t, ctx, tx, "SELECT count(*) FROM iam.membership_permissions", 0)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tenant B accounting read: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.profiles (
			org_id,
			country_code,
			legal_name,
			legal_address,
			tax_condition,
			activity_start_date,
			default_currency
		)
		VALUES (
			$1,
			'AR',
			'Fixture SA',
			'{}'::jsonb,
			'responsable_inscripto',
			DATE '2020-01-01',
			'ARS'
		)
	`, orgA); err != nil {
		t.Fatalf("insert fiscal profile: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal_ar.settings (
			org_id, environment, cuit, iva_condition, enabled
		)
		VALUES (
			$1,
			'homologation',
			'20123456786',
			'responsable_inscripto',
			true
		)
	`, orgA); err != nil {
		t.Fatalf("insert AR fiscal settings: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.points_of_sale (
			org_id, id, country_code, environment, code, name
		)
		VALUES ($1, $2, 'AR', 'homologation', 1, 'Homologación')
	`, orgA, pointID); err != nil {
		t.Fatalf("insert fiscal point of sale: %v", err)
	}

	const voucherJSON = `{"issuer":"fixture","total":"121.00"}`
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.vouchers (
			org_id,
			id,
			environment,
			point_of_sale_id,
			operation,
			voucher_type,
			source_type,
			source_id,
			idempotency_key,
			command_fingerprint,
			concept,
			issue_date,
			currency_code,
			exchange_rate,
			exchange_rate_date,
			exchange_rate_source,
			net_amount,
			vat_amount,
			total_amount,
			created_by
		)
		VALUES (
			$1,
			$2,
			'homologation',
			$3,
			'invoice',
			6,
			'sale',
			'sale-1',
			'voucher-idempotency-1',
			repeat('a', 64),
			'products',
			DATE '2026-01-10',
			'ARS',
			1,
			DATE '2026-01-10',
			'functional-currency',
			100,
			21,
			121,
			'integration-test'
		)
	`, orgA, voucherID, pointID); err != nil {
		t.Fatalf("insert fiscal voucher: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.voucher_snapshots (
			org_id,
			id,
			voucher_id,
			issuer_tax_id,
			issuer_legal_name,
			issuer_tax_condition,
			issuer_address,
			issuer_activity_start_date,
			recipient_document_type,
			recipient_document_number,
			recipient_name,
			recipient_tax_condition,
			currency_code,
			exchange_rate,
			exchange_rate_date,
			exchange_rate_source,
			issue_date,
			net_amount,
			exempt_amount,
			non_taxed_amount,
			vat_amount,
			other_tributes_amount,
			total_amount,
			canonical_json,
			snapshot_sha256
		)
		VALUES (
			$1,
			$2,
			$3,
			'20123456786',
			'Fixture SA',
			'responsable_inscripto',
			'{}'::jsonb,
			DATE '2020-01-01',
			99,
			'0',
			'Consumidor Final',
			'consumidor_final',
			'ARS',
			1,
			DATE '2026-01-10',
			'functional-currency',
			DATE '2026-01-10',
			100,
			0,
			0,
			21,
			0,
			121,
			$4,
			encode(digest(convert_to($4, 'UTF8'), 'sha256'), 'hex')
		)
	`, orgA, snapshotID, voucherID, voucherJSON); err != nil {
		t.Fatalf("insert fiscal snapshot: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.voucher_lines (
			org_id,
			snapshot_id,
			line_no,
			description,
			quantity,
			unit_of_measure,
			unit_price,
			tax_treatment,
			vat_rate,
			net_amount,
			vat_amount,
			total_amount
		)
		VALUES (
			$1, $2, 1, 'Producto', 1, 'unit', 100,
			'taxable', 21, 100, 21, 121
		)
	`, orgA, snapshotID); err != nil {
		t.Fatalf("insert fiscal voucher line: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.voucher_taxes (
			org_id,
			snapshot_id,
			line_no,
			tax_type,
			authority_code,
			description,
			taxable_base,
			rate,
			amount
		)
		VALUES ($1, $2, 1, 'vat', '5', 'IVA 21%', 100, 21, 21)
	`, orgA, snapshotID); err != nil {
		t.Fatalf("insert fiscal voucher tax: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit fiscal voucher snapshot: %v", err)
	}

	tx = beginTenantTx(t, ctx, fiscalWorker, orgA, userA)
	var leasedVoucherID string
	if err := tx.QueryRow(
		ctx,
		`SELECT fiscal.lease_voucher(
			$1,
			'integration-worker',
			interval '2 minutes'
		)`,
		orgA,
	).Scan(&leasedVoucherID); err != nil {
		t.Fatalf("lease fiscal voucher: %v", err)
	}
	if leasedVoucherID != voucherID {
		t.Fatalf("leased voucher = %s, want %s", leasedVoucherID, voucherID)
	}
	if _, err := tx.Exec(
		ctx,
		`SELECT fiscal.reserve_voucher_number($1, $2, 1)`,
		orgA,
		voucherID,
	); err != nil {
		t.Fatalf("reserve voucher number: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.vouchers
		   SET status = 'authorized',
		       authorization_code = '12345678901234',
		       authorization_expires_at = DATE '2026-01-20',
		       arca_result = 'A',
		       response_sha256 = repeat('b', 64),
		       response_storage_ref = 's3://fixture/response',
		       lease_owner = NULL,
		       lease_until = NULL,
		       authorized_at = now(),
		       version = version + 1
		 WHERE org_id = $1 AND id = $2
	`, orgA, voucherID); err != nil {
		t.Fatalf("authorize fiscal voucher: %v", err)
	}
	if _, err := tx.Exec(
		ctx,
		"SET CONSTRAINTS fiscal_vouchers_posting_intent_valid IMMEDIATE",
	); err == nil {
		t.Fatal("authorized fiscal voucher committed without a posting intent")
	}
	_ = tx.Rollback(ctx)

	tx = beginTenantTx(t, ctx, fiscalWorker, orgA, userA)
	if err := tx.QueryRow(
		ctx,
		`SELECT fiscal.lease_voucher(
			$1,
			'integration-worker',
			interval '2 minutes'
		)`,
		orgA,
	).Scan(&leasedVoucherID); err != nil {
		t.Fatalf("lease fiscal voucher after atomicity rollback: %v", err)
	}
	if leasedVoucherID != voucherID {
		t.Fatalf(
			"leased voucher after rollback = %s, want %s",
			leasedVoucherID,
			voucherID,
		)
	}
	if _, err := tx.Exec(
		ctx,
		`SELECT fiscal.reserve_voucher_number($1, $2, 1)`,
		orgA,
		voucherID,
	); err != nil {
		t.Fatalf("reserve voucher number after atomicity rollback: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.vouchers
		   SET status = 'authorized',
		       authorization_code = '12345678901234',
		       authorization_expires_at = DATE '2026-01-20',
		       arca_result = 'A',
		       response_sha256 = repeat('b', 64),
		       response_storage_ref = 's3://fixture/response',
		       lease_owner = NULL,
		       lease_until = NULL,
		       authorized_at = now(),
		       version = version + 1
		 WHERE org_id = $1 AND id = $2
	`, orgA, voucherID); err != nil {
		t.Fatalf("reauthorize fiscal voucher after atomicity rollback: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.accounting_posting_intents (
			org_id,
			voucher_id,
			source_type,
			source_id,
			operation,
			snapshot_sha256,
			authority_code
		)
		SELECT
			voucher.org_id,
			voucher.id,
			voucher.source_type,
			voucher.source_id,
			voucher.operation,
			snapshot.snapshot_sha256,
			voucher.authorization_code
		FROM fiscal.vouchers AS voucher
		JOIN fiscal.voucher_snapshots AS snapshot
		  ON snapshot.org_id = voucher.org_id
		 AND snapshot.voucher_id = voucher.id
		WHERE voucher.org_id = $1
		  AND voucher.id = $2
	`, orgA, voucherID); err != nil {
		t.Fatalf("insert fiscal accounting posting intent: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit fiscal authorization: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	assertTxCount(
		t,
		ctx,
		tx,
		`SELECT count(*)
		   FROM fiscal.accounting_posting_intents
		  WHERE voucher_id = $1
		    AND status = 'pending'
		    AND snapshot_sha256 = encode(
		        digest(convert_to($2, 'UTF8'), 'sha256'),
		        'hex'
		    )`,
		1,
		voucherID,
		voucherJSON,
	)
	if _, err := tx.Exec(
		ctx,
		`UPDATE fiscal.vouchers
		    SET last_error_code = 'mutated'
		  WHERE org_id = $1 AND id = $2`,
		orgA,
		voucherID,
	); err == nil {
		t.Fatal("authorized fiscal voucher accepted an update")
	}
	_ = tx.Rollback(ctx)

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(
		ctx,
		`UPDATE fiscal.voucher_snapshots
		    SET recipient_name = 'Mutated'
		  WHERE org_id = $1 AND id = $2`,
		orgA,
		snapshotID,
	); err == nil {
		t.Fatal("fiscal snapshot accepted an update")
	}
	_ = tx.Rollback(ctx)

	const purchaseJSON = `{"supplier":"fixture","total":"121.00"}`
	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.purchase_vouchers (
			org_id,
			id,
			environment,
			supplier_id,
			supplier_tax_id,
			supplier_name,
			voucher_type,
			point_of_sale,
			voucher_number,
			issue_date,
			currency_code,
			exchange_rate,
			exchange_rate_date,
			exchange_rate_source,
			net_amount,
			vat_amount,
			total_amount,
			source_type,
			source_id,
			idempotency_key,
			canonical_json,
			snapshot_sha256,
			created_by
		)
		VALUES (
			$1,
			$2,
			'homologation',
			'supplier-1',
			'20123456786',
			'Proveedor SA',
			1,
			2,
			3,
			DATE '2026-01-11',
			'ARS',
			1,
			DATE '2026-01-11',
			'functional-currency',
			100,
			21,
			121,
			'purchase',
			'purchase-1',
			'purchase-idempotency-1',
			$3,
			encode(digest(convert_to($3, 'UTF8'), 'sha256'), 'hex'),
			'integration-test'
		)
	`, orgA, purchaseID, purchaseJSON); err != nil {
		t.Fatalf("insert purchase voucher: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.purchase_voucher_lines (
			org_id,
			purchase_voucher_id,
			line_no,
			description,
			quantity,
			unit_of_measure,
			unit_price,
			tax_treatment,
			vat_rate,
			net_amount,
			vat_amount,
			total_amount
		)
		VALUES (
			$1, $2, 1, 'Insumo', 1, 'unit', 100,
			'taxable', 21, 100, 21, 121
		)
	`, orgA, purchaseID); err != nil {
		t.Fatalf("insert purchase voucher line: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.purchase_voucher_taxes (
			org_id,
			purchase_voucher_id,
			line_no,
			tax_type,
			authority_code,
			description,
			taxable_base,
			rate,
			amount,
			creditable
		)
		VALUES (
			$1, $2, 1, 'vat', '5', 'IVA 21%', 100, 21, 21, true
		)
	`, orgA, purchaseID); err != nil {
		t.Fatalf("insert purchase voucher tax: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit purchase voucher: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	insertJournalEntry(
		t,
		ctx,
		tx,
		orgA,
		ivaSalesEntryID,
		periodID,
		"tax",
		"iva-sales-posting",
		"",
	)
	insertJournalLine(
		t, ctx, tx, orgA, ivaSalesEntryID, 1, receivableAccountID, "21", "0",
	)
	insertJournalLine(
		t, ctx, tx, orgA, ivaSalesEntryID, 2, vatPayableAccountID, "0", "21",
	)
	insertJournalEntryAt(
		t,
		ctx,
		tx,
		orgA,
		ivaPurchaseEntryID,
		periodID,
		"2026-01-11",
		"tax",
		"iva-purchase-posting",
		"",
	)
	insertJournalLine(
		t, ctx, tx, orgA, ivaPurchaseEntryID, 1, vatCreditAccountID, "21", "0",
	)
	insertJournalLine(
		t, ctx, tx, orgA, ivaPurchaseEntryID, 2, cashAccountID, "0", "21",
	)
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.voucher_accounting_links (
			org_id, voucher_id, journal_entry_id, created_by
		)
		VALUES ($1, $2, $3, 'integration-test')
	`, orgA, voucherID, ivaSalesEntryID); err != nil {
		t.Fatalf("link IVA sales accounting entry: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.purchase_voucher_accounting_links (
			org_id, purchase_voucher_id, journal_entry_id, created_by
		)
		VALUES ($1, $2, $3, 'integration-test')
	`, orgA, purchaseID, ivaPurchaseEntryID); err != nil {
		t.Fatalf("link IVA purchase accounting entry: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.iva_periods (
			org_id,
			id,
			environment,
			period_month,
			closing_balance,
			created_by
		)
		VALUES (
			$1, $2, 'homologation', DATE '2026-01-01', 0, 'integration-test'
		)
	`, orgA, ivaPeriodID); err != nil {
		t.Fatalf("insert IVA period: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.iva_period_items (
			org_id,
			iva_period_id,
			book,
			voucher_id,
			document_sha256,
			net_amount,
			exempt_amount,
			non_taxed_amount,
			vat_debit_amount
		)
		SELECT
			$1,
			$2,
			'sales',
			voucher.id,
			snapshot.snapshot_sha256,
			voucher.net_amount,
			voucher.exempt_amount,
			voucher.non_taxed_amount,
			voucher.vat_amount
		FROM fiscal.vouchers AS voucher
		JOIN fiscal.voucher_snapshots AS snapshot
		  ON snapshot.org_id = voucher.org_id
		 AND snapshot.voucher_id = voucher.id
		WHERE voucher.org_id = $1 AND voucher.id = $3
	`, orgA, ivaPeriodID, voucherID); err != nil {
		t.Fatalf("insert IVA sales item: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.iva_period_items (
			org_id,
			iva_period_id,
			book,
			purchase_voucher_id,
			document_sha256,
			net_amount,
			exempt_amount,
			non_taxed_amount,
			vat_credit_amount
		)
		SELECT
			$1,
			$2,
			'purchases',
			purchase.id,
			purchase.snapshot_sha256,
			purchase.net_amount,
			purchase.exempt_amount,
			purchase.non_taxed_amount,
			21
		FROM fiscal.purchase_vouchers AS purchase
		WHERE purchase.org_id = $1 AND purchase.id = $3
	`, orgA, ivaPeriodID, purchaseID); err != nil {
		t.Fatalf("insert IVA purchase item: %v", err)
	}
	if _, err := tx.Exec(ctx, "SAVEPOINT iva_reversal_check"); err != nil {
		t.Fatalf("create IVA reversal savepoint: %v", err)
	}
	insertJournalEntry(
		t,
		ctx,
		tx,
		orgA,
		ivaSalesReversalID,
		periodID,
		"reversal",
		"iva-sales-reversal",
		ivaSalesEntryID,
	)
	insertJournalLine(
		t, ctx, tx, orgA, ivaSalesReversalID, 1, vatPayableAccountID, "21", "0",
	)
	insertJournalLine(
		t, ctx, tx, orgA, ivaSalesReversalID, 2, receivableAccountID, "0", "21",
	)
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.iva_periods
		   SET status = 'closed',
		       version = version + 1,
		       status_changed_by = 'integration-test',
		       transition_reason = 'Must fail while posting is reversed',
		       closed_at = now(),
		       updated_at = now()
		 WHERE org_id = $1 AND id = $2
	`, orgA, ivaPeriodID); err == nil {
		t.Fatal("IVA period closed while a linked VAT posting was reversed")
	}
	if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT iva_reversal_check"); err != nil {
		t.Fatalf("rollback IVA reversal savepoint: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.iva_periods
		   SET status = 'closed',
		       version = version + 1,
		       status_changed_by = 'integration-test',
		       transition_reason = 'Integration close',
		       closed_at = now(),
		       updated_at = now()
		 WHERE org_id = $1 AND id = $2
	`, orgA, ivaPeriodID); err != nil {
		t.Fatalf("close IVA period: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit IVA period close: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.iva_period_items (
			org_id,
			iva_period_id,
			book,
			voucher_id,
			document_sha256,
			net_amount,
			exempt_amount,
			non_taxed_amount,
			vat_debit_amount
		)
		SELECT
			$1,
			$2,
			'sales',
			voucher.id,
			snapshot.snapshot_sha256,
			voucher.net_amount,
			voucher.exempt_amount,
			voucher.non_taxed_amount,
			voucher.vat_amount
		FROM fiscal.vouchers AS voucher
		JOIN fiscal.voucher_snapshots AS snapshot
		  ON snapshot.org_id = voucher.org_id
		 AND snapshot.voucher_id = voucher.id
		WHERE voucher.org_id = $1 AND voucher.id = $3
	`, orgA, ivaPeriodID, voucherID); err == nil {
		t.Fatal("closed IVA period accepted a new item")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback rejected closed IVA item: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	assertTxCount(
		t,
		ctx,
		tx,
		`SELECT count(*)
		   FROM fiscal.iva_position_view
		  WHERE iva_period_id = $1
		    AND vat_debit_amount = 21
		    AND vat_credit_amount = 21
		    AND closing_balance = 0`,
		1,
		ivaPeriodID,
	)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit IVA position read: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	var ivaExportID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO fiscal.iva_exports (
			org_id,
			iva_period_id,
			export_type,
			export_version,
			storage_ref,
			filename,
			media_type,
			artifact,
			sha256,
			created_by
		)
		VALUES (
			$1,
			$2,
			'workpaper',
			1,
			'db://fiscal/iva-exports/integration',
			'iva-simple-2026-01-homologation.zip',
			'application/zip',
			convert_to('immutable-iva-export', 'UTF8'),
			encode(
				digest(convert_to('immutable-iva-export', 'UTF8'), 'sha256'),
				'hex'
			),
			'integration-test'
		)
		RETURNING id::text
	`, orgA, ivaPeriodID).Scan(&ivaExportID); err != nil {
		t.Fatalf("insert immutable IVA export: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.iva_periods
		   SET status = 'exported',
		       version = version + 1,
		       status_changed_by = 'integration-test',
		       transition_reason = 'Integration export',
		       exported_at = now(),
		       updated_at = now()
		 WHERE org_id = $1 AND id = $2
	`, orgA, ivaPeriodID); err != nil {
		t.Fatalf("mark IVA period exported: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit IVA export: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.vouchers (
			org_id,
			id,
			environment,
			point_of_sale_id,
			operation,
			voucher_type,
			source_type,
			source_id,
			idempotency_key,
			command_fingerprint,
			concept,
			issue_date,
			currency_code,
			exchange_rate,
			exchange_rate_date,
			exchange_rate_source,
			net_amount,
			vat_amount,
			total_amount,
			created_by
		)
		VALUES (
			$1,
			$2,
			'homologation',
			$3,
			'invoice',
			6,
			'sale',
			'late-sale',
			'late-voucher-idempotency',
			repeat('c', 64),
			'products',
			DATE '2026-01-20',
			'ARS',
			1,
			DATE '2026-01-20',
			'functional-currency',
			100,
			21,
			121,
			'integration-test'
		)
	`, orgA, lateVoucherID, pointID); err == nil {
		t.Fatal("exported IVA period accepted a late fiscal document")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback rejected late fiscal document: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.iva_exports
		   SET artifact = convert_to('changed', 'UTF8')
		 WHERE org_id = $1 AND id = $2
	`, orgA, ivaExportID); err == nil {
		t.Fatal("immutable IVA export accepted an update")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback rejected IVA export update: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgA, userA)
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.iva_periods
		   SET status = 'draft',
		       closing_balance = NULL,
		       version = version + 1,
		       status_changed_by = 'integration-test',
		       transition_reason = 'Correct supplier voucher',
		       closed_at = NULL,
		       exported_at = NULL,
		       updated_at = now()
		 WHERE org_id = $1 AND id = $2
	`, orgA, ivaPeriodID); err != nil {
		t.Fatalf("reopen exported IVA period: %v", err)
	}
	assertTxCount(
		t,
		ctx,
		tx,
		`SELECT count(*) FROM fiscal.iva_exports WHERE iva_period_id = $1`,
		1,
		ivaPeriodID,
	)
	assertTxCount(
		t,
		ctx,
		tx,
		`SELECT count(*) FROM fiscal.iva_period_events WHERE iva_period_id = $1`,
		3,
		ivaPeriodID,
	)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit exported IVA reopen: %v", err)
	}

	tx = beginTenantTx(t, ctx, backend, orgB, userA)
	assertTxCount(t, ctx, tx, "SELECT count(*) FROM fiscal.vouchers", 0)
	assertTxCount(
		t,
		ctx,
		tx,
		"SELECT count(*) FROM fiscal.accounting_posting_intents",
		0,
	)
	assertTxCount(t, ctx, tx, "SELECT count(*) FROM fiscal.purchase_vouchers", 0)
	assertTxCount(t, ctx, tx, "SELECT count(*) FROM fiscal.iva_periods", 0)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tenant B fiscal read: %v", err)
	}

	assertFiscalWorkerDurability(
		t,
		ctx,
		database,
		backend,
		fiscalWorker,
		orgA,
		userA,
		pointID,
	)
	assertFiscalWorkerDataBoundary(
		t,
		ctx,
		fiscalWorker,
		orgA,
		userA,
	)
	assertFiscalAccountingWorkerBoundary(
		t,
		ctx,
		fiscalAccountingWorker,
		orgA,
		userA,
	)
}

func assertFiscalWorkerDataBoundary(
	t *testing.T,
	ctx context.Context,
	worker interface {
		Begin(context.Context) (pgx.Tx, error)
	},
	orgID string,
	userID string,
) {
	t.Helper()

	tx := beginTenantTx(t, ctx, worker, orgID, userID)
	if _, err := tx.Exec(
		ctx,
		"SELECT count(*) FROM accounting.accounts",
	); err == nil {
		_ = tx.Rollback(ctx)
		t.Fatal("ARCA worker can read accounting accounts")
	}
	_ = tx.Rollback(ctx)

	tx = beginTenantTx(t, ctx, worker, orgID, userID)
	if _, err := tx.Exec(
		ctx,
		"SELECT count(*) FROM fiscal.voucher_accounting_links",
	); err == nil {
		_ = tx.Rollback(ctx)
		t.Fatal("ARCA worker can read fiscal accounting links")
	}
	_ = tx.Rollback(ctx)
}

func assertFiscalWorkerDurability(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
	backend interface {
		Begin(context.Context) (pgx.Tx, error)
	},
	fiscalWorker interface {
		Begin(context.Context) (pgx.Tx, error)
	},
	orgID string,
	userID string,
	pointOfSaleID string,
) {
	t.Helper()
	const (
		firstVoucher   = "00000000-0000-0000-0000-00000000e501"
		secondVoucher  = "00000000-0000-0000-0000-00000000e502"
		firstSnapshot  = "00000000-0000-0000-0000-00000000e601"
		secondSnapshot = "00000000-0000-0000-0000-00000000e602"
	)

	tx := beginTenantTx(t, ctx, backend, orgID, userID)
	insertFiscalWorkerVoucherFixture(
		t, ctx, tx, orgID, pointOfSaleID, firstVoucher, firstSnapshot, "first",
	)
	insertFiscalWorkerVoucherFixture(
		t, ctx, tx, orgID, pointOfSaleID, secondVoucher, secondSnapshot, "second",
	)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit fiscal worker durability fixtures: %v", err)
	}

	tx = beginTenantTx(t, ctx, fiscalWorker, orgID, userID)
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.vouchers
		   SET status = 'processing',
		       lease_owner = 'crashed-worker:lease',
		       lease_until = now() - interval '1 minute',
		       version = version + 1
		 WHERE org_id = $1
		   AND id = $2
	`, orgID, firstVoucher); err != nil {
		t.Fatalf("create expired fiscal processing lease: %v", err)
	}
	if _, err := tx.Exec(
		ctx,
		`SELECT fiscal.reserve_voucher_number($1, $2, 1)`,
		orgID,
		firstVoucher,
	); err != nil {
		t.Fatalf("reserve durable fiscal number: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit expired numbered fiscal lease: %v", err)
	}

	var expiredLeaseDiscoverable bool
	if err := database.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			  FROM fiscal.pending_organizations(100)
			 WHERE org_id = $1
		)
	`, orgID).Scan(&expiredLeaseDiscoverable); err != nil {
		t.Fatalf("discover expired fiscal processing lease: %v", err)
	}
	if !expiredLeaseDiscoverable {
		t.Fatal("expired fiscal processing lease is not discoverable")
	}

	assertSeriesCannotAdvance(
		t, ctx, fiscalWorker, orgID, userID, secondVoucher, "processing",
	)

	tx = beginTenantTx(t, ctx, fiscalWorker, orgID, userID)
	var reclaimedID string
	if err := tx.QueryRow(ctx, `
		UPDATE fiscal.vouchers
		   SET lease_owner = 'recovery-worker:lease',
		       lease_until = now() + interval '2 minutes',
		       version = version + 1
		 WHERE org_id = $1
		   AND id = $2
		   AND status = 'processing'
		   AND lease_until <= now()
		RETURNING id::text
	`, orgID, firstVoucher).Scan(&reclaimedID); err != nil {
		t.Fatalf("reclaim expired fiscal processing lease: %v", err)
	}
	if reclaimedID != firstVoucher {
		t.Fatalf("reclaimed voucher = %s, want %s", reclaimedID, firstVoucher)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit reclaimed fiscal processing lease: %v", err)
	}

	tx = beginTenantTx(t, ctx, fiscalWorker, orgID, userID)
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.vouchers
		   SET status = 'uncertain',
		       lease_owner = NULL,
		       lease_until = NULL,
		       uncertain_at = now(),
		       last_error_code = 'authority_response_uncertain',
		       last_error_detail_redacted = 'integration fixture',
		       version = version + 1
		 WHERE org_id = $1
		   AND id = $2
	`, orgID, firstVoucher); err != nil {
		t.Fatalf("mark reclaimed voucher uncertain: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit uncertain fiscal voucher: %v", err)
	}
	assertSeriesCannotAdvance(
		t, ctx, fiscalWorker, orgID, userID, secondVoucher, "uncertain",
	)

	tx = beginTenantTx(t, ctx, fiscalWorker, orgID, userID)
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.vouchers
		   SET status = 'rejected',
		       rejected_at = now(),
		       uncertain_at = NULL,
		       version = version + 1
		 WHERE org_id = $1
		   AND id = $2
	`, orgID, firstVoucher); err != nil {
		t.Fatalf("resolve uncertain voucher as rejected: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit resolved fiscal voucher: %v", err)
	}

	tx = beginTenantTx(t, ctx, fiscalWorker, orgID, userID)
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.vouchers
		   SET status = 'processing',
		       lease_owner = 'next-worker:lease',
		       lease_until = now() + interval '2 minutes',
		       version = version + 1
		 WHERE org_id = $1
		   AND id = $2
	`, orgID, secondVoucher); err != nil {
		t.Fatalf("advance resolved fiscal series: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit next fiscal series lease: %v", err)
	}
}

func assertSeriesCannotAdvance(
	t *testing.T,
	ctx context.Context,
	backend interface {
		Begin(context.Context) (pgx.Tx, error)
	},
	orgID string,
	userID string,
	voucherID string,
	blockingState string,
) {
	t.Helper()
	tx := beginTenantTx(t, ctx, backend, orgID, userID)
	_, err := tx.Exec(ctx, `
		UPDATE fiscal.vouchers
		   SET status = 'processing',
		       lease_owner = 'overtaking-worker:lease',
		       lease_until = now() + interval '2 minutes',
		       version = version + 1
		 WHERE org_id = $1
		   AND id = $2
	`, orgID, voucherID)
	if err == nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("fiscal series advanced while earlier voucher was %s", blockingState)
	}
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) ||
		databaseError.ConstraintName != "fiscal_vouchers_one_unresolved_series_uidx" {
		_ = tx.Rollback(ctx)
		t.Fatalf(
			"series blocker error while earlier voucher was %s = %v",
			blockingState,
			err,
		)
	}
	_ = tx.Rollback(ctx)
}

func insertFiscalWorkerVoucherFixture(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
	orgID string,
	pointOfSaleID string,
	voucherID string,
	snapshotID string,
	suffix string,
) {
	t.Helper()
	canonical := `{"fixture":"worker-durability-` + suffix + `"}`
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.vouchers (
			org_id, id, environment, point_of_sale_id, operation,
			voucher_type, source_type, source_id, idempotency_key,
			command_fingerprint, concept, issue_date, currency_code,
			exchange_rate, exchange_rate_date, exchange_rate_source,
			net_amount, vat_amount, total_amount, created_by
		)
		VALUES (
			$1, $2, 'homologation', $3, 'invoice',
			1, 'sale', $4, $5,
			encode(digest(convert_to($5, 'UTF8'), 'sha256'), 'hex'),
			'products', DATE '2026-01-10', 'ARS',
			1, DATE '2026-01-10', 'functional-currency',
			100, 21, 121, 'integration-test'
		)
	`, orgID, voucherID, pointOfSaleID, "worker-"+suffix, "worker-idempotency-"+suffix); err != nil {
		t.Fatalf("insert fiscal worker voucher %s: %v", suffix, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.voucher_snapshots (
			org_id, id, voucher_id, issuer_tax_id, issuer_legal_name,
			issuer_tax_condition, issuer_address, issuer_activity_start_date,
			recipient_document_type, recipient_document_number, recipient_name,
			recipient_tax_condition, currency_code, exchange_rate,
			exchange_rate_date, exchange_rate_source, issue_date, net_amount,
			exempt_amount, non_taxed_amount, vat_amount, other_tributes_amount,
			total_amount, canonical_json, snapshot_sha256
		)
		VALUES (
			$1, $2, $3, '20123456786', 'Fixture SA',
			'responsable_inscripto', '{}'::jsonb, DATE '2020-01-01',
			99, '0', 'Consumidor Final', 'consumidor_final', 'ARS', 1,
			DATE '2026-01-10', 'functional-currency', DATE '2026-01-10',
			100, 0, 0, 21, 0, 121, $4,
			encode(digest(convert_to($4, 'UTF8'), 'sha256'), 'hex')
		)
	`, orgID, snapshotID, voucherID, canonical); err != nil {
		t.Fatalf("insert fiscal worker snapshot %s: %v", suffix, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.voucher_lines (
			org_id, snapshot_id, line_no, description, quantity,
			unit_of_measure, unit_price, tax_treatment, vat_rate,
			net_amount, vat_amount, total_amount
		)
		VALUES (
			$1, $2, 1, 'Durability fixture', 1,
			'unit', 100, 'taxable', 21, 100, 21, 121
		)
	`, orgID, snapshotID); err != nil {
		t.Fatalf("insert fiscal worker line %s: %v", suffix, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.voucher_taxes (
			org_id, snapshot_id, line_no, tax_type, authority_code,
			description, taxable_base, rate, amount
		)
		VALUES (
			$1, $2, 1, 'vat', '5', 'IVA 21%', 100, 21, 21
		)
	`, orgID, snapshotID); err != nil {
		t.Fatalf("insert fiscal worker tax %s: %v", suffix, err)
	}
}

func assertPendingOrganizationsPrivileges(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
) {
	t.Helper()
	var (
		publicCanExecute                 bool
		backendCanExecute                bool
		fiscalWorkerCanExecute           bool
		fiscalAccountingWorkerCanExecute bool
	)
	if err := database.QueryRow(ctx, `
		WITH routine AS (
			SELECT procedure.oid, procedure.proacl, procedure.proowner
			  FROM pg_proc AS procedure
			  JOIN pg_namespace AS namespace
			    ON namespace.oid = procedure.pronamespace
			 WHERE namespace.nspname = 'fiscal'
			   AND procedure.proname = 'pending_organizations'
			   AND pg_get_function_identity_arguments(procedure.oid) = 'requested_limit integer'
		)
		SELECT
			EXISTS (
				SELECT 1
				  FROM routine,
				       LATERAL aclexplode(
				           coalesce(
				               routine.proacl,
				               acldefault('f', routine.proowner)
				           )
				       ) AS privilege
				 WHERE privilege.grantee = 0
				   AND privilege.privilege_type = 'EXECUTE'
			),
			has_function_privilege(
				'pymes_backend',
				'fiscal.pending_organizations(integer)',
				'EXECUTE'
			),
			has_function_privilege(
				'pymes_fiscal_worker',
				'fiscal.pending_organizations(integer)',
				'EXECUTE'
			),
			has_function_privilege(
				'pymes_fiscal_accounting_worker',
				'fiscal.pending_organizations(integer)',
				'EXECUTE'
			)
	`).Scan(
		&publicCanExecute,
		&backendCanExecute,
		&fiscalWorkerCanExecute,
		&fiscalAccountingWorkerCanExecute,
	); err != nil {
		t.Fatalf("inspect pending organization routine privileges: %v", err)
	}
	if publicCanExecute {
		t.Fatal("PUBLIC can execute fiscal.pending_organizations")
	}
	if backendCanExecute ||
		!fiscalWorkerCanExecute ||
		!fiscalAccountingWorkerCanExecute {
		t.Fatalf(
			"pending organization execution grants: backend=%t fiscal_worker=%t fiscal_accounting_worker=%t",
			backendCanExecute,
			fiscalWorkerCanExecute,
			fiscalAccountingWorkerCanExecute,
		)
	}
}

func assertFiscalWorkerRoutinePrivileges(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
) {
	t.Helper()
	type routinePrivilege struct {
		signature string
	}
	for _, routine := range []routinePrivilege{
		{signature: "fiscal.lock_voucher_series(uuid,text,uuid,integer)"},
		{signature: "fiscal.reserve_voucher_number(uuid,uuid,bigint)"},
		{signature: "fiscal.lease_voucher(uuid,text,interval)"},
	} {
		var publicCanExecute, backendCanExecute, fiscalWorkerCanExecute bool
		if err := database.QueryRow(ctx, `
			SELECT
				EXISTS (
					SELECT 1
					  FROM pg_proc AS procedure,
					       LATERAL aclexplode(
					           coalesce(
					               procedure.proacl,
					               acldefault('f', procedure.proowner)
					           )
					       ) AS privilege
					 WHERE procedure.oid = to_regprocedure($1)
					   AND privilege.grantee = 0
					   AND privilege.privilege_type = 'EXECUTE'
				),
				has_function_privilege(
					'pymes_backend',
					$1,
					'EXECUTE'
				),
				has_function_privilege(
					'pymes_fiscal_worker',
					$1,
					'EXECUTE'
				)
		`, routine.signature).Scan(
			&publicCanExecute,
			&backendCanExecute,
			&fiscalWorkerCanExecute,
		); err != nil {
			t.Fatalf(
				"inspect fiscal worker routine %s privileges: %v",
				routine.signature,
				err,
			)
		}
		if publicCanExecute || backendCanExecute || !fiscalWorkerCanExecute {
			t.Fatalf(
				"fiscal worker routine %s grants: public=%t backend=%t fiscal_worker=%t",
				routine.signature,
				publicCanExecute,
				backendCanExecute,
				fiscalWorkerCanExecute,
			)
		}
	}
}

func assertFiscalAccountingWorkerBoundary(
	t *testing.T,
	ctx context.Context,
	worker interface {
		Begin(context.Context) (pgx.Tx, error)
	},
	orgID string,
	userID string,
) {
	t.Helper()

	tx := beginTenantTx(t, ctx, worker, orgID, userID)
	assertTxCount(
		t,
		ctx,
		tx,
		`SELECT count(*)
		   FROM fiscal.accounting_posting_intents
		  WHERE status IN ('pending', 'failed', 'posted')`,
		1,
	)
	var lockedPeriodID string
	if err := tx.QueryRow(ctx, `
		SELECT id::text
		  FROM accounting.periods
		 ORDER BY start_date, id
		 FOR UPDATE
		 LIMIT 1
	`).Scan(&lockedPeriodID); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("fiscal accounting worker cannot lock a posting period: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit fiscal accounting worker read: %v", err)
	}

	tx = beginTenantTx(t, ctx, worker, orgID, userID)
	if _, err := tx.Exec(ctx, `
		UPDATE accounting.periods
		   SET status = 'locked'
		 WHERE id::text = $1
	`, lockedPeriodID); err == nil {
		_ = tx.Rollback(ctx)
		t.Fatal("fiscal accounting worker can change accounting period status")
	}
	_ = tx.Rollback(ctx)

	tx = beginTenantTx(t, ctx, worker, orgID, userID)
	if _, err := tx.Exec(
		ctx,
		`SELECT fiscal.lease_voucher(
			$1,
			'fiscal-accounting-worker',
			interval '2 minutes'
		)`,
		orgID,
	); err == nil {
		_ = tx.Rollback(ctx)
		t.Fatal("fiscal accounting worker can lease ARCA vouchers")
	}
	_ = tx.Rollback(ctx)

	tx = beginTenantTx(t, ctx, worker, orgID, userID)
	if _, err := tx.Exec(
		ctx,
		"SELECT count(*) FROM fiscal.certificates",
	); err == nil {
		_ = tx.Rollback(ctx)
		t.Fatal("fiscal accounting worker can read fiscal certificates")
	}
	_ = tx.Rollback(ctx)
}

func beginTenantTx(
	t *testing.T,
	ctx context.Context,
	pool interface {
		Begin(context.Context) (pgx.Tx, error)
	},
	orgID string,
	userID string,
) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tenant transaction: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		SELECT
			set_config('app.org_id', $1, true),
			set_config('app.user_id', $2, true)
	`, orgID, userID); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("set tenant context: %v", err)
	}
	return tx
}

func insertJournalEntry(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
	orgID string,
	entryID string,
	periodID string,
	entryKind string,
	idempotencyKey string,
	reversesEntryID string,
) {
	t.Helper()
	insertJournalEntryAt(
		t,
		ctx,
		tx,
		orgID,
		entryID,
		periodID,
		"2026-01-10",
		entryKind,
		idempotencyKey,
		reversesEntryID,
	)
}

func insertJournalEntryAt(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
	orgID string,
	entryID string,
	periodID string,
	entryDate string,
	entryKind string,
	idempotencyKey string,
	reversesEntryID string,
) {
	t.Helper()
	if reversesEntryID == "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO accounting.journal_entries (
				org_id,
				id,
				entry_date,
				period_id,
				entry_kind,
				description,
				functional_currency,
				source_type,
				source_id,
				posting_kind,
				idempotency_key,
				created_by
			)
			VALUES (
				$1,
				$2,
				$6::date,
				$3,
				$4,
				'Integration entry',
				'ARS',
				'integration',
				$5,
				'primary',
				$5,
				'integration-test'
			)
		`,
			orgID,
			entryID,
			periodID,
			entryKind,
			idempotencyKey,
			entryDate,
		); err != nil {
			t.Fatalf("insert journal entry: %v", err)
		}
		return
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.journal_entries (
			org_id,
			id,
			entry_date,
			period_id,
			entry_kind,
			description,
			functional_currency,
			posting_kind,
			idempotency_key,
			reverses_entry_id,
			reversal_reason,
			reversed_by,
			created_by
		)
		VALUES (
			$1,
			$2,
			$6::date,
			$3,
			'reversal',
			'Integration reversal',
			'ARS',
			'reversal',
			$4,
			$5,
			'Integration reversal',
			'integration-test',
			'integration-test'
		)
	`,
		orgID,
		entryID,
		periodID,
		idempotencyKey,
		reversesEntryID,
		entryDate,
	); err != nil {
		t.Fatalf("insert reversal entry: %v", err)
	}
}

func insertJournalLine(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
	orgID string,
	entryID string,
	lineNo int,
	accountID string,
	debit string,
	credit string,
) {
	t.Helper()
	amount := debit
	if amount == "0" {
		amount = credit
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO accounting.journal_lines (
			org_id,
			journal_entry_id,
			line_no,
			account_id,
			debit_amount,
			credit_amount,
			currency_code,
			currency_amount,
			exchange_rate
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'ARS', $7, 1)
	`, orgID, entryID, lineNo, accountID, debit, credit, amount); err != nil {
		t.Fatalf("insert journal line %d: %v", lineNo, err)
	}
}

func assertTenantTablesForceRLS(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
) {
	t.Helper()
	var unprotected string
	if err := database.QueryRow(ctx, `
		SELECT coalesce(string_agg(
			format('%I.%I', namespace.nspname, relation.relname),
			', '
			ORDER BY namespace.nspname, relation.relname
		), '')
		FROM pg_class AS relation
		JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		WHERE relation.relkind = 'r'
		  AND namespace.nspname IN ('accounting', 'fiscal', 'fiscal_ar')
		  AND EXISTS (
			  SELECT 1
			  FROM pg_attribute AS attribute
			  WHERE attribute.attrelid = relation.oid
			    AND attribute.attname = 'org_id'
			    AND NOT attribute.attisdropped
		  )
		  AND (NOT relation.relrowsecurity OR NOT relation.relforcerowsecurity)
	`).Scan(&unprotected); err != nil {
		t.Fatalf("inspect tenant RLS: %v", err)
	}
	if unprotected != "" {
		t.Fatalf("tenant tables without forced RLS: %s", unprotected)
	}
}

func assertNoFloatingPointMoney(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
) {
	t.Helper()
	var floatingColumns string
	if err := database.QueryRow(ctx, `
		SELECT coalesce(string_agg(
			format('%I.%I.%I', table_schema, table_name, column_name),
			', '
			ORDER BY table_schema, table_name, ordinal_position
		), '')
		FROM information_schema.columns
		WHERE table_schema IN ('accounting', 'fiscal', 'fiscal_ar')
		  AND data_type IN ('real', 'double precision')
	`).Scan(&floatingColumns); err != nil {
		t.Fatalf("inspect floating point columns: %v", err)
	}
	if floatingColumns != "" {
		t.Fatalf("floating point persistence found: %s", floatingColumns)
	}
}
