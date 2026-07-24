package fiscalaccounting

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

func TestPostgresWorkerPostsOnceAndRetriesMissingMapping(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin := openTestPool(t, ctx, databaseURL, "pymes", "pymes")
	defer admin.Close()
	backend := openTestPool(
		t,
		ctx,
		databaseURL,
		"pymes_backend",
		"pymes_backend",
	)
	defer backend.Close()
	fiscalWorker := openTestPool(
		t,
		ctx,
		databaseURL,
		"pymes_fiscal_worker",
		"pymes_fiscal_worker",
	)
	defer fiscalWorker.Close()
	accountingWorker := openTestPool(
		t,
		ctx,
		databaseURL,
		"pymes_fiscal_accounting_worker",
		"pymes_fiscal_accounting_worker",
	)
	defer accountingWorker.Close()

	organizationID := uuid.New()
	pointOfSaleID := uuid.New()
	seedAccountingOrganization(
		t,
		ctx,
		admin,
		backend,
		organizationID,
		pointOfSaleID,
	)
	first := seedAuthorizedVoucher(
		t,
		ctx,
		backend,
		fiscalWorker,
		organizationID,
		pointOfSaleID,
		1,
		"21",
		"21",
	)

	config := DefaultConfig()
	config.WorkerID = "integration-" + uuid.NewString()
	config.ActorID = "system:fiscal-accounting-integration"
	config.RetryDelay = 0
	config.PollInterval = time.Millisecond
	worker, err := NewWorker(accountingWorker, config)
	if err != nil {
		t.Fatalf("create fiscal accounting worker: %v", err)
	}

	start := make(chan struct{})
	type outcome struct {
		result Result
		err    error
	}
	outcomes := make(chan outcome, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			result, runErr := worker.RunOnce(ctx, organizationID)
			outcomes <- outcome{result: result, err: runErr}
		}()
	}
	close(start)
	wait.Wait()
	close(outcomes)

	var posted, noWork int
	for outcome := range outcomes {
		switch {
		case outcome.err == nil:
			posted++
			if outcome.result.IntentID != first.intentID ||
				outcome.result.JournalEntryID == uuid.Nil ||
				outcome.result.Attempt != 1 {
				t.Fatalf("unexpected successful worker result: %#v", outcome.result)
			}
		case errors.Is(outcome.err, ErrNoWork):
			noWork++
		default:
			t.Fatalf("concurrent worker error: %v", outcome.err)
		}
	}
	if posted != 1 || noWork != 1 {
		t.Fatalf("concurrent outcomes: posted=%d no_work=%d", posted, noWork)
	}
	assertPostedFixture(t, ctx, backend, organizationID, first, 1)

	second := seedAuthorizedVoucher(
		t,
		ctx,
		backend,
		fiscalWorker,
		organizationID,
		pointOfSaleID,
		2,
		"15",
		"15",
	)
	failedResult, err := worker.RunOnce(ctx, organizationID)
	if !errors.Is(err, accounting.ErrMappingMissing) {
		t.Fatalf("missing mapping error = %v", err)
	}
	if failedResult.IntentID != second.intentID ||
		failedResult.ErrorCode != "mapping_missing" {
		t.Fatalf("unexpected failed worker result: %#v / %v", failedResult, err)
	}
	assertFailedFixture(t, ctx, backend, organizationID, second.intentID, 1)

	if _, err := admin.Exec(ctx, `
		INSERT INTO accounting.account_mapping_definitions (
			role, label_es, label_en, description_es, description_en,
			required, compatible_account_classes,
			compatible_normal_balances, compatible_monetary_classes,
			canonical_role, is_alias, display_order
		)
		VALUES (
			'vat_payable_15',
			'IVA débito fiscal 15%',
			'VAT payable 15%',
			'Cuenta de integración para IVA débito fiscal 15%',
			'Integration account for 15% VAT payable',
			true,
			ARRAY['liability'],
			ARRAY['credit'],
			ARRAY['monetary'],
			NULL,
			false,
			9999
		)
		ON CONFLICT (role) DO NOTHING
	`); err != nil {
		t.Fatalf("install retry mapping definition: %v", err)
	}
	withTenantTx(t, ctx, backend, organizationID, func(tx pgx.Tx) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO accounting.account_mappings (
				org_id,
				mapping_key,
				account_id,
				description
			)
			SELECT
				$1,
				'vat_payable_15',
				account_id,
				'IVA débito fiscal 15% (integration fixture)'
			  FROM accounting.account_mappings
			 WHERE org_id = $1
			   AND mapping_key = 'vat_payable_21'`,
			organizationID,
		); err != nil {
			t.Fatalf("install retry mapping: %v", err)
		}
	})
	retried, err := worker.RunOnce(ctx, organizationID)
	if err != nil {
		t.Fatalf("retry fiscal accounting intent: %v", err)
	}
	if retried.IntentID != second.intentID || retried.Attempt != 2 {
		t.Fatalf("unexpected retry result: %#v", retried)
	}
	assertPostedFixture(t, ctx, backend, organizationID, second, 2)
}

type fiscalAccountingFixture struct {
	intentID  uuid.UUID
	voucherID uuid.UUID
	sourceID  uuid.UUID
	total     string
	vat       string
}

func openTestPool(
	t *testing.T,
	ctx context.Context,
	databaseURL, user, password string,
) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse PostgreSQL test URL: %v", err)
	}
	if user != "" {
		config.ConnConfig.User = user
		config.ConnConfig.Password = password
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open PostgreSQL test pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL test pool: %v", err)
	}
	return pool
}

func seedAccountingOrganization(
	t *testing.T,
	ctx context.Context,
	admin, backend *pgxpool.Pool,
	organizationID, pointOfSaleID uuid.UUID,
) {
	t.Helper()
	if _, err := admin.Exec(ctx, `
		INSERT INTO iam.organizations (
			id,
			provider,
			external_id,
			name,
			slug,
			status
		)
		VALUES ($1, 'integration', $2, 'Fiscal Accounting', $3, 'active')`,
		organizationID,
		"org_"+organizationID.String(),
		"fiscal-accounting-"+organizationID.String(),
	); err != nil {
		t.Fatalf("insert fiscal accounting organization: %v", err)
	}
	withTenantTx(t, ctx, backend, organizationID, func(tx pgx.Tx) {
		var installed int
		if err := tx.QueryRow(ctx, `
			SELECT accounting.install_chart_template($1, 'ar-pyme', 1)`,
			organizationID,
		).Scan(&installed); err != nil {
			t.Fatalf("install accounting chart template: %v", err)
		}
		if installed == 0 {
			t.Fatal("accounting chart template installed no accounts")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO accounting.periods (
				org_id,
				code,
				start_date,
				end_date
			)
			VALUES (
				$1,
				'2026',
				DATE '2026-01-01',
				DATE '2026-12-31'
			)`,
			organizationID,
		); err != nil {
			t.Fatalf("insert accounting period: %v", err)
		}
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
				'Fiscal Accounting SA',
				'{}'::jsonb,
				'responsable_inscripto',
				DATE '2020-01-01',
				'ARS'
			)`,
			organizationID,
		); err != nil {
			t.Fatalf("insert fiscal profile: %v", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO fiscal_ar.settings (
				org_id,
				environment,
				cuit,
				iva_condition,
				enabled
			)
			VALUES (
				$1,
				'homologation',
				'20123456786',
				'responsable_inscripto',
				true
			)`,
			organizationID,
		); err != nil {
			t.Fatalf("insert AR fiscal settings: %v", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO fiscal.points_of_sale (
				org_id,
				id,
				country_code,
				environment,
				code,
				name
			)
			VALUES (
				$1,
				$2,
				'AR',
				'homologation',
				1,
				'Integración'
			)`,
			organizationID,
			pointOfSaleID,
		); err != nil {
			t.Fatalf("insert fiscal point of sale: %v", err)
		}
	})
}

func seedAuthorizedVoucher(
	t *testing.T,
	ctx context.Context,
	backend, fiscalWorker *pgxpool.Pool,
	organizationID, pointOfSaleID uuid.UUID,
	number int64,
	rate, vatAmount string,
) fiscalAccountingFixture {
	t.Helper()
	voucherID := uuid.New()
	snapshotID := uuid.New()
	intentID := uuid.New()
	sourceID := uuid.New()
	vat := fiscal.MustDecimal(vatAmount)
	total := fiscal.MustDecimal("100").Add(vat)
	document := fiscal.FiscalSnapshot{
		Version:     fiscal.SnapshotVersion,
		CountryCode: "AR",
		IssueDate:   "2026-07-24",
		Issuer: fiscal.PartySnapshot{
			Name:             "Fiscal Accounting SA",
			TaxID:            "20123456786",
			TaxCondition:     "responsable_inscripto",
			Address:          "CABA",
			ActivityStartDay: "2020-01-01",
		},
		Receiver: fiscal.PartySnapshot{
			Name:           "Consumidor Final",
			DocumentType:   "99",
			DocumentNumber: "0",
			TaxCondition:   "consumidor_final",
		},
		Currency: fiscal.CurrencySnapshot{
			Code:       "ARS",
			Rate:       fiscal.MustDecimal("1"),
			RateDate:   "2026-07-24",
			RateSource: "functional-currency",
		},
		Lines: []fiscal.FiscalLineSnapshot{{
			Position:    1,
			Description: "Servicio de integración",
			Quantity:    fiscal.MustDecimal("1"),
			UnitPrice:   fiscal.MustDecimal("100"),
			NetAmount:   fiscal.MustDecimal("100"),
			TaxCode:     "IVA" + rate,
			TaxRate:     fiscal.MustDecimal(rate),
			TaxAmount:   vat,
			TotalAmount: total,
		}},
		Totals: fiscal.FiscalTotalsSnapshot{
			NetTaxed:   fiscal.MustDecimal("100"),
			VAT:        vat,
			Total:      total,
			Functional: total,
		},
	}
	snapshot, err := fiscal.NewSnapshot(document)
	if err != nil {
		t.Fatalf("build immutable fiscal snapshot: %v", err)
	}
	authorityCode := fmt.Sprintf("%014d", number)

	withTenantTx(t, ctx, backend, organizationID, func(tx pgx.Tx) {
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
				$4,
				$5,
				$6,
				'products',
				DATE '2026-07-24',
				'ARS',
				1,
				DATE '2026-07-24',
				'functional-currency',
				100,
				$7::numeric,
				$8::numeric,
				'integration-test'
			)`,
			organizationID,
			voucherID,
			pointOfSaleID,
			sourceID.String(),
			"voucher-"+voucherID.String(),
			fmt.Sprintf("%064x", number),
			vat.String(),
			total.String(),
		); err != nil {
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
				'Fiscal Accounting SA',
				'responsable_inscripto',
				'{}'::jsonb,
				DATE '2020-01-01',
				99,
				'0',
				'Consumidor Final',
				'consumidor_final',
				'ARS',
				1,
				DATE '2026-07-24',
				'functional-currency',
				DATE '2026-07-24',
				100,
				0,
				0,
				$4::numeric,
				0,
				$5::numeric,
				$6,
				$7
			)`,
			organizationID,
			snapshotID,
			voucherID,
			vat.String(),
			total.String(),
			string(snapshot.CanonicalJSON()),
			snapshot.Hash(),
		); err != nil {
			t.Fatalf("insert fiscal voucher snapshot: %v", err)
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
				$1,
				$2,
				1,
				'Servicio de integración',
				1,
				'unit',
				100,
				'taxable',
				$3::numeric,
				100,
				$4::numeric,
				$5::numeric
			)`,
			organizationID,
			snapshotID,
			rate,
			vat.String(),
			total.String(),
		); err != nil {
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
			VALUES (
				$1,
				$2,
				1,
				'vat',
				'5',
				'IVA',
				100,
				$3::numeric,
				$4::numeric
			)`,
			organizationID,
			snapshotID,
			rate,
			vat.String(),
		); err != nil {
			t.Fatalf("insert fiscal voucher tax: %v", err)
		}
	})

	withTenantTx(t, ctx, fiscalWorker, organizationID, func(tx pgx.Tx) {
		var leased uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT fiscal.lease_voucher(
				$1,
				'integration-authorizer',
				interval '2 minutes'
			)`,
			organizationID,
		).Scan(&leased); err != nil {
			t.Fatalf("lease fiscal voucher: %v", err)
		}
		if leased != voucherID {
			t.Fatalf("leased voucher %s, want %s", leased, voucherID)
		}
		if _, err := tx.Exec(ctx, `
			SELECT fiscal.reserve_voucher_number($1, $2, $3)`,
			organizationID,
			voucherID,
			number,
		); err != nil {
			t.Fatalf("reserve fiscal voucher number: %v", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE fiscal.vouchers
			   SET status = 'authorized',
			       authorization_code = $3,
			       authorization_expires_at = DATE '2026-08-03',
			       arca_result = 'A',
			       response_sha256 = repeat('b', 64),
			       response_storage_ref = $4,
			       lease_owner = NULL,
			       lease_until = NULL,
			       authorized_at = now(),
			       version = version + 1
			 WHERE org_id = $1
			   AND id = $2`,
			organizationID,
			voucherID,
			authorityCode,
			"s3://integration/"+voucherID.String(),
		); err != nil {
			t.Fatalf("authorize fiscal voucher: %v", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO fiscal.accounting_posting_intents (
				org_id,
				id,
				voucher_id,
				source_type,
				source_id,
				operation,
				snapshot_sha256,
				authority_code
			)
			VALUES (
				$1,
				$2,
				$3,
				'sale',
				$4,
				'invoice',
				$5,
				$6
			)`,
			organizationID,
			intentID,
			voucherID,
			sourceID.String(),
			snapshot.Hash(),
			authorityCode,
		); err != nil {
			t.Fatalf("insert fiscal accounting intent: %v", err)
		}
	})
	return fiscalAccountingFixture{
		intentID:  intentID,
		voucherID: voucherID,
		sourceID:  sourceID,
		total:     total.String(),
		vat:       vat.String(),
	}
}

func assertPostedFixture(
	t *testing.T,
	ctx context.Context,
	backend *pgxpool.Pool,
	organizationID uuid.UUID,
	fixture fiscalAccountingFixture,
	attempt int,
) {
	t.Helper()
	withTenantTx(t, ctx, backend, organizationID, func(tx pgx.Tx) {
		var (
			status        string
			storedAttempt int
			entryID       uuid.UUID
			entryCount    int
			debit         string
			credit        string
		)
		if err := tx.QueryRow(ctx, `
			SELECT status, attempt_count, journal_entry_id
			  FROM fiscal.accounting_posting_intents
			 WHERE org_id = $1
			   AND id = $2`,
			organizationID,
			fixture.intentID,
		).Scan(&status, &storedAttempt, &entryID); err != nil {
			t.Fatalf("read posted fiscal accounting intent: %v", err)
		}
		if status != "posted" || storedAttempt != attempt || entryID == uuid.Nil {
			t.Fatalf(
				"unexpected intent state: %s/%d/%s",
				status,
				storedAttempt,
				entryID,
			)
		}
		if err := tx.QueryRow(ctx, `
			SELECT
				count(DISTINCT entry.id),
				sum(line.debit_amount)::text,
				sum(line.credit_amount)::text
			  FROM accounting.journal_entries AS entry
			  JOIN accounting.journal_lines AS line
			    ON line.org_id = entry.org_id
			   AND line.journal_entry_id = entry.id
			 WHERE entry.org_id = $1
			   AND entry.source_type = 'sale'
			   AND entry.source_id = $2`,
			organizationID,
			fixture.sourceID.String(),
		).Scan(&entryCount, &debit, &credit); err != nil {
			t.Fatalf("read fiscal accounting journal entry: %v", err)
		}
		debitAmount, debitErr := accounting.ParseAmount(debit)
		creditAmount, creditErr := accounting.ParseAmount(credit)
		expectedAmount := accounting.MustDecimal(fixture.total)
		if entryCount != 1 ||
			debitErr != nil ||
			creditErr != nil ||
			!debitAmount.Equal(expectedAmount) ||
			!creditAmount.Equal(expectedAmount) {
			t.Fatalf(
				"unexpected journal totals: entries=%d debit=%s credit=%s want=%s",
				entryCount,
				debit,
				credit,
				fixture.total,
			)
		}
		var linked uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT journal_entry_id
			  FROM fiscal.voucher_accounting_links
			 WHERE org_id = $1
			   AND voucher_id = $2`,
			organizationID,
			fixture.voucherID,
		).Scan(&linked); err != nil {
			t.Fatalf("read voucher accounting link: %v", err)
		}
		if linked != entryID {
			t.Fatalf("voucher link %s does not match intent %s", linked, entryID)
		}
	})
}

func assertFailedFixture(
	t *testing.T,
	ctx context.Context,
	backend *pgxpool.Pool,
	organizationID, intentID uuid.UUID,
	attempt int,
) {
	t.Helper()
	withTenantTx(t, ctx, backend, organizationID, func(tx pgx.Tx) {
		var status, code, detail string
		var storedAttempt int
		if err := tx.QueryRow(ctx, `
			SELECT
				status,
				attempt_count,
				last_error_code,
				last_error_detail_redacted
			  FROM fiscal.accounting_posting_intents
			 WHERE org_id = $1
			   AND id = $2`,
			organizationID,
			intentID,
		).Scan(&status, &storedAttempt, &code, &detail); err != nil {
			t.Fatalf("read failed fiscal accounting intent: %v", err)
		}
		if status != "failed" ||
			storedAttempt != attempt ||
			code != "mapping_missing" ||
			detail != "fiscal accounting posting failed (mapping_missing)" {
			t.Fatalf(
				"unexpected failed state: %s/%d/%s/%s",
				status,
				storedAttempt,
				code,
				detail,
			)
		}
	})
}

func withTenantTx(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	organizationID uuid.UUID,
	work func(pgx.Tx),
) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tenant fixture transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	if _, err := tx.Exec(ctx, `
		SELECT
			set_config('app.org_id', $1, true),
			set_config('app.user_id', 'integration-test', true)`,
		organizationID.String(),
	); err != nil {
		t.Fatalf("bind tenant fixture: %v", err)
	}
	work(tx)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tenant fixture: %v", err)
	}
}
