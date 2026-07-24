package migrate

import (
	"context"
	"os"
	"testing"
	"time"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	platformiam "github.com/devpablocristo/platform/iam/go"
	platformoutbox "github.com/devpablocristo/platform/outbox/go"
	"github.com/devpablocristo/pymes/v2/db/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestUpFromEmptyDatabaseIsRepeatableAcrossReconnect(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database := openDatabase(t, ctx, databaseURL)
	resetDatabase(t, ctx, database)
	ensureTestRoles(t, ctx, database)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		resetDatabase(t, cleanupCtx, database)
		database.Close()
	})

	if err := Up(ctx, database); err != nil {
		t.Fatalf("first Up() error = %v", err)
	}
	if err := Up(ctx, database); err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	assertSchemaState(t, ctx, database)

	database.Close()
	database = openDatabase(t, ctx, databaseURL)
	if err := Up(ctx, database); err != nil {
		t.Fatalf("Up() after reconnect error = %v", err)
	}
	assertSchemaState(t, ctx, database)
	assertTenantIsolationAndOwnerInvariant(t, ctx, database, databaseURL)
	assertAccountingAndFiscalInvariants(t, ctx, database, databaseURL)
}

func openDatabase(t *testing.T, ctx context.Context, databaseURL string) *postgres.DB {
	t.Helper()
	database, err := postgres.OpenWithConfig(ctx, databaseURL, postgres.DefaultConfig("pymes-v2-db-test"))
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	return database
}

func resetDatabase(t *testing.T, ctx context.Context, database *postgres.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, "DROP SCHEMA IF EXISTS fiscal_ar CASCADE"); err != nil {
		t.Fatalf("drop fiscal_ar schema: %v", err)
	}
	if _, err := database.Exec(ctx, "DROP SCHEMA IF EXISTS fiscal CASCADE"); err != nil {
		t.Fatalf("drop fiscal schema: %v", err)
	}
	if _, err := database.Exec(ctx, "DROP SCHEMA IF EXISTS accounting CASCADE"); err != nil {
		t.Fatalf("drop accounting schema: %v", err)
	}
	if _, err := database.Exec(ctx, "DROP SCHEMA IF EXISTS app CASCADE"); err != nil {
		t.Fatalf("drop app schema: %v", err)
	}
	if _, err := database.Exec(ctx, "DROP SCHEMA IF EXISTS iam CASCADE"); err != nil {
		t.Fatalf("drop iam schema: %v", err)
	}
	if _, err := database.Exec(ctx, "DROP TABLE IF EXISTS public.platform_outbox_messages"); err != nil {
		t.Fatalf("drop outbox table: %v", err)
	}
	if _, err := database.Exec(ctx, "DROP TABLE IF EXISTS public.platform_idempotency_records"); err != nil {
		t.Fatalf("drop idempotency table: %v", err)
	}
	if _, err := database.Exec(ctx, "DROP TABLE IF EXISTS schema_migrations"); err != nil {
		t.Fatalf("drop migration table: %v", err)
	}
}

func assertSchemaState(t *testing.T, ctx context.Context, database *postgres.DB) {
	t.Helper()
	var schemaExists bool
	if err := database.QueryRow(ctx, "SELECT to_regnamespace('app') IS NOT NULL").Scan(&schemaExists); err != nil {
		t.Fatalf("query app schema: %v", err)
	}
	if !schemaExists {
		t.Fatal("app schema does not exist")
	}
	var provisioningTableExists bool
	if err := database.QueryRow(
		ctx,
		"SELECT to_regclass('app.organization_provisioning_requests') IS NOT NULL",
	).Scan(&provisioningTableExists); err != nil {
		t.Fatalf("query organization provisioning table: %v", err)
	}
	if !provisioningTableExists {
		t.Fatal("organization provisioning table does not exist")
	}
	var provisioningOutboxViewExists bool
	if err := database.QueryRow(
		ctx,
		"SELECT to_regclass('app.organization_provisioning_outbox_messages') IS NOT NULL",
	).Scan(&provisioningOutboxViewExists); err != nil {
		t.Fatalf("query organization provisioning outbox view: %v", err)
	}
	if !provisioningOutboxViewExists {
		t.Fatal("organization provisioning outbox view does not exist")
	}
	var lifecycleAuditTableExists bool
	if err := database.QueryRow(
		ctx,
		"SELECT to_regclass('app.lifecycle_audit_events') IS NOT NULL",
	).Scan(&lifecycleAuditTableExists); err != nil {
		t.Fatalf("query lifecycle audit table: %v", err)
	}
	if !lifecycleAuditTableExists {
		t.Fatal("lifecycle audit table does not exist")
	}
	for _, relation := range []string{
		"accounting.accounts",
		"accounting.journal_entries",
		"accounting.journal_lines",
		"accounting.reconciliations",
		"fiscal.vouchers",
		"fiscal.voucher_snapshots",
		"fiscal.accounting_posting_intents",
		"fiscal.purchase_vouchers",
		"fiscal.iva_periods",
		"fiscal.homologation_runs",
		"fiscal.homologation_checks",
		"fiscal_ar.settings",
	} {
		var exists bool
		if err := database.QueryRow(
			ctx,
			"SELECT to_regclass($1) IS NOT NULL",
			relation,
		).Scan(&exists); err != nil {
			t.Fatalf("query relation %s: %v", relation, err)
		}
		if !exists {
			t.Fatalf("relation %s does not exist", relation)
		}
	}
	var lifecycleColumnCount int
	if err := database.QueryRow(ctx, `
		SELECT count(*)
		FROM information_schema.columns
		WHERE table_schema = 'iam'
		  AND table_name IN ('organizations', 'users')
		  AND column_name IN ('archived_at', 'trashed_at', 'purge_after')
	`).Scan(&lifecycleColumnCount); err != nil {
		t.Fatalf("query lifecycle columns: %v", err)
	}
	if lifecycleColumnCount != 6 {
		t.Fatalf("lifecycle column count = %d, want 6", lifecycleColumnCount)
	}

	var productMigrationCount int
	if err := database.QueryRow(
		ctx,
		"SELECT count(*) FROM schema_migrations WHERE scope = $1",
		migrations.Scope,
	).Scan(&productMigrationCount); err != nil {
		t.Fatalf("query product migrations: %v", err)
	}
	if productMigrationCount != 18 {
		t.Fatalf("product migration count = %d, want 18", productMigrationCount)
	}

	var iamMigrationCount int
	if err := database.QueryRow(
		ctx,
		"SELECT count(*) FROM schema_migrations WHERE scope = $1",
		platformiam.MigrationScope,
	).Scan(&iamMigrationCount); err != nil {
		t.Fatalf("query IAM migrations: %v", err)
	}
	if iamMigrationCount != 1 {
		t.Fatalf("IAM migration count = %d, want 1", iamMigrationCount)
	}

	var outboxMigrationCount int
	if err := database.QueryRow(
		ctx,
		"SELECT count(*) FROM schema_migrations WHERE scope = $1",
		platformoutbox.MigrationScope,
	).Scan(&outboxMigrationCount); err != nil {
		t.Fatalf("query outbox migrations: %v", err)
	}
	if outboxMigrationCount != 1 {
		t.Fatalf("outbox migration count = %d, want 1", outboxMigrationCount)
	}

	var idempotencyMigrationCount int
	if err := database.QueryRow(
		ctx,
		"SELECT count(*) FROM schema_migrations WHERE scope = $1",
		IdempotencyScope,
	).Scan(&idempotencyMigrationCount); err != nil {
		t.Fatalf("query idempotency migrations: %v", err)
	}
	if idempotencyMigrationCount != 1 {
		t.Fatalf("idempotency migration count = %d, want 1", idempotencyMigrationCount)
	}
}

func ensureTestRoles(t *testing.T, ctx context.Context, database *postgres.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
		DO $roles$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
				CREATE ROLE pymes_backend
					LOGIN
					PASSWORD 'pymes_backend'
					NOBYPASSRLS
					NOINHERIT;
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_iam_worker') THEN
				CREATE ROLE pymes_iam_worker
					LOGIN
					PASSWORD 'pymes_iam_worker'
					NOBYPASSRLS
					NOINHERIT;
			END IF;
			IF NOT EXISTS (
				SELECT 1 FROM pg_roles WHERE rolname = 'pymes_fiscal_worker'
			) THEN
				CREATE ROLE pymes_fiscal_worker
					LOGIN
					PASSWORD 'pymes_fiscal_worker'
					NOBYPASSRLS
					NOINHERIT;
			END IF;
			IF NOT EXISTS (
				SELECT 1
				  FROM pg_roles
				 WHERE rolname = 'pymes_fiscal_accounting_worker'
			) THEN
				CREATE ROLE pymes_fiscal_accounting_worker
					LOGIN
					PASSWORD 'pymes_fiscal_accounting_worker'
					NOBYPASSRLS
					NOINHERIT;
			END IF;
		END
		$roles$;
		ALTER ROLE pymes_backend
			LOGIN
			PASSWORD 'pymes_backend'
			NOBYPASSRLS
			NOINHERIT;
		ALTER ROLE pymes_iam_worker
			LOGIN
			PASSWORD 'pymes_iam_worker'
			NOBYPASSRLS
			NOINHERIT;
		ALTER ROLE pymes_fiscal_worker
			LOGIN
			PASSWORD 'pymes_fiscal_worker'
			NOBYPASSRLS
			NOINHERIT;
		ALTER ROLE pymes_fiscal_accounting_worker
			LOGIN
			PASSWORD 'pymes_fiscal_accounting_worker'
			NOBYPASSRLS
			NOINHERIT;
	`); err != nil {
		t.Fatalf("ensure backend test role: %v", err)
	}
}

func assertTenantIsolationAndOwnerInvariant(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
	databaseURL string,
) {
	t.Helper()
	const (
		orgA         = "00000000-0000-0000-0000-00000000a001"
		orgB         = "00000000-0000-0000-0000-00000000b001"
		userA        = "00000000-0000-0000-0000-00000000a101"
		userB        = "00000000-0000-0000-0000-00000000b101"
		userPending  = "00000000-0000-0000-0000-00000000a102"
		membershipA  = "00000000-0000-0000-0000-00000000a201"
		membershipB  = "00000000-0000-0000-0000-00000000b201"
		membershipP  = "00000000-0000-0000-0000-00000000a202"
		invitationA  = "00000000-0000-0000-0000-00000000a301"
		invitationB  = "00000000-0000-0000-0000-00000000b301"
		orgBootstrap = "00000000-0000-0000-0000-00000000c001"
		requestBoot  = "00000000-0000-0000-0000-00000000c101"
		inviteBoot   = "00000000-0000-0000-0000-00000000c201"
	)

	if _, err := database.Exec(ctx, `
		INSERT INTO iam.organizations (id, provider, external_id, name, slug, status)
		VALUES
			($1, 'clerk', 'org_a', 'Org A', 'org-a', 'provisioning'),
			($2, 'clerk', 'org_b', 'Org B', 'org-b', 'provisioning')
	`, orgA, orgB); err != nil {
		t.Fatalf("seed IAM organizations: %v", err)
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO iam.users (
			id, provider, external_id, primary_email, email_verified, name, status
		)
		VALUES
			($1, 'clerk', 'user_a', 'a@example.test', true, 'User A', 'active'),
			($2, 'clerk', 'user_b', 'b@example.test', true, 'User B', 'active'),
			($3, 'clerk', 'user_pending', 'pending@example.test', false, 'Pending', 'active')
	`, userA, userB, userPending); err != nil {
		t.Fatalf("seed IAM users: %v", err)
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO iam.memberships (
			id, org_id, user_id, provider, external_id, role, status, joined_at
		)
		VALUES
			($5, $1, $3, 'clerk', 'membership_a', 'admin', 'active', now()),
			($6, $2, $4, 'clerk', 'membership_b', 'admin', 'active', now()),
			($7, $1, $8, 'clerk', 'membership_pending', 'member', 'active', now())
	`,
		orgA,
		orgB,
		userA,
		userB,
		membershipA,
		membershipB,
		membershipP,
		userPending,
	); err != nil {
		t.Fatalf("seed IAM memberships: %v", err)
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO app.global_user_roles (user_id, role, status)
		VALUES ($1, 'owner', 'active')
	`, userA); err != nil {
		t.Fatalf("seed global owner: %v", err)
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO iam.invitations (
			id, org_id, provider, external_id, email_normalized, role, status, expires_at
		)
		VALUES
			($3, $1, 'clerk', 'invitation_a', 'invite-a@example.test', 'member', 'pending', now() + interval '1 day'),
			($4, $2, 'clerk', 'invitation_b', 'invite-b@example.test', 'member', 'pending', now() + interval '1 day')
	`, orgA, orgB, invitationA, invitationB); err != nil {
		t.Fatalf("seed IAM invitations: %v", err)
	}
	if _, err := database.Exec(ctx, `
		UPDATE iam.organizations
		   SET status = 'active'
		 WHERE id IN ($1, $2)
	`, orgA, orgB); err != nil {
		t.Fatalf("activate IAM organizations: %v", err)
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO iam.invitations (
			org_id, provider, email_normalized, role, status, expires_at
		)
		VALUES ($1, 'clerk', 'not-owner@example.test', 'owner', 'pending', now() + interval '1 day')
	`, orgA); err == nil {
		t.Fatal("tenant owner invitation was accepted")
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO iam.organizations (id, provider, name, slug, status)
		VALUES ($1, 'clerk', 'Bootstrap Org', 'bootstrap-org', 'provisioning')
	`, orgBootstrap); err != nil {
		t.Fatalf("seed bootstrap organization: %v", err)
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO app.organization_provisioning_requests (
			id,
			organization_id,
			provider,
			slug,
			organization_name,
			owner_email_normalized,
			payload_sha256,
			outbox_message_id,
			status
		)
		VALUES (
			$2,
			$1,
			'clerk',
			'bootstrap-org',
			'Bootstrap Org',
			'bootstrap-owner@example.test',
			repeat('a', 64),
			'bootstrap-owner-outbox',
			'queued'
		)
	`, orgBootstrap, requestBoot); err != nil {
		t.Fatalf("seed bootstrap provisioning request: %v", err)
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO iam.invitations (
			org_id, provider, email_normalized, role, status, expires_at
		)
		VALUES ($1, 'clerk', 'wrong-owner@example.test', 'owner', 'pending', now() + interval '1 day')
	`, orgBootstrap); err == nil {
		t.Fatal("owner invitation with mismatched bootstrap email was accepted")
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO iam.invitations (
			id, org_id, provider, email_normalized, role, status, expires_at
		)
		VALUES (
			$2,
			$1,
			'clerk',
			'bootstrap-owner@example.test',
			'admin',
			'pending',
			now() + interval '1 day'
		)
	`, orgBootstrap, inviteBoot); err != nil {
		t.Fatalf("bootstrap admin invitation was rejected: %v", err)
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO public.platform_outbox_messages (
			id,
			idempotency_key,
			topic,
			payload,
			available_at,
			max_attempts,
			created_at,
			updated_at
		)
		VALUES
			(
				'worker-visible',
				'worker-visible',
				'iam.organization.provision.requested.v1',
				'{}'::text::bytea,
				now(),
				12,
				now(),
				now()
			),
			(
				'worker-hidden',
				'worker-hidden',
				'iam.invitation.create.requested.v1',
				'{}'::text::bytea,
				now(),
				12,
				now(),
				now()
			)
	`); err != nil {
		t.Fatalf("seed outbox topic boundary: %v", err)
	}

	backend := openBackendPool(t, ctx, databaseURL)
	defer backend.Close()

	assertCount(t, ctx, backend, "SELECT count(*) FROM public.platform_outbox_messages", 2)
	assertCount(t, ctx, backend, "SELECT count(*) FROM public.platform_idempotency_records", 0)
	assertCount(t, ctx, backend, "SELECT count(*) FROM iam.organizations", 0)
	assertCount(t, ctx, backend, "SELECT count(*) FROM iam.memberships", 0)
	assertCount(t, ctx, backend, "SELECT count(*) FROM iam.invitations", 0)
	assertCount(
		t,
		ctx,
		backend,
		"SELECT count(*) FROM iam.resolve_active_membership('clerk', 'user_a', 'org_a')",
		1,
	)
	assertCount(
		t,
		ctx,
		backend,
		"SELECT count(*) FROM iam.resolve_active_membership('clerk', 'user_a', 'org_b')",
		0,
	)
	assertCount(
		t,
		ctx,
		backend,
		"SELECT count(*) FROM iam.resolve_active_membership('clerk', 'user_pending', 'org_a')",
		0,
	)
	assertCount(
		t,
		ctx,
		backend,
		"SELECT count(*) FROM iam.list_active_organizations('clerk', 'user_pending')",
		0,
	)

	tx, err := backend.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tenant transaction: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		SELECT
			set_config('app.user_id', $1, true),
			set_config('app.org_id', $2, true)
	`, userA, orgA); err != nil {
		t.Fatalf("set tenant context: %v", err)
	}
	assertTxCount(t, ctx, tx, "SELECT count(*) FROM iam.organizations", 1)
	assertTxCount(t, ctx, tx, "SELECT count(*) FROM iam.memberships", 2)
	assertTxCount(t, ctx, tx, "SELECT count(*) FROM iam.invitations", 1)
	assertTxCount(
		t,
		ctx,
		tx,
		"SELECT count(*) FROM iam.organizations WHERE id = $1",
		0,
		orgB,
	)
	assertTxCount(
		t,
		ctx,
		tx,
		"SELECT count(*) FROM iam.memberships WHERE org_id = $1",
		0,
		orgB,
	)
	assertTxCount(
		t,
		ctx,
		tx,
		"SELECT count(*) FROM iam.invitations WHERE org_id = $1",
		0,
		orgB,
	)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tenant transaction: %v", err)
	}
	assertCount(t, ctx, backend, "SELECT count(*) FROM iam.organizations", 0)
	assertCount(t, ctx, backend, "SELECT count(*) FROM iam.memberships", 0)
	assertCount(t, ctx, backend, "SELECT count(*) FROM iam.invitations", 0)

	ownerTx, err := backend.Begin(ctx)
	if err != nil {
		t.Fatalf("begin owner transaction: %v", err)
	}
	if _, err := ownerTx.Exec(ctx, `
		SELECT
			set_config('app.user_id', $1, true),
			set_config('app.org_id', $2, true),
			set_config('app.actor_provider', 'clerk', true),
			set_config('app.actor_subject', 'user_a', true)
	`, userA, orgA); err != nil {
		t.Fatalf("set owner context: %v", err)
	}
	assertTxCount(t, ctx, ownerTx, "SELECT count(*) FROM iam.organizations", 3)
	assertTxCount(t, ctx, ownerTx, "SELECT count(*) FROM iam.memberships", 3)
	assertTxCount(t, ctx, ownerTx, "SELECT count(*) FROM iam.invitations", 3)
	assertTxCount(
		t,
		ctx,
		ownerTx,
		"SELECT count(*) FROM iam.organizations WHERE id = $1",
		1,
		orgB,
	)
	var effectiveRole string
	if err := ownerTx.QueryRow(
		ctx,
		`SELECT role FROM iam.resolve_active_membership('clerk', 'user_a', 'org_a')`,
	).Scan(&effectiveRole); err != nil {
		t.Fatalf("resolve global owner membership: %v", err)
	}
	if effectiveRole != "owner" {
		t.Fatalf("effective global owner role = %q", effectiveRole)
	}
	if _, err := ownerTx.Exec(
		ctx,
		`UPDATE iam.organizations SET archived_at = now() WHERE id = $1`,
		orgA,
	); err != nil {
		t.Fatalf("archive organization fixture: %v", err)
	}
	assertTxCount(
		t,
		ctx,
		ownerTx,
		"SELECT count(*) FROM iam.resolve_active_membership('clerk', 'user_a', 'org_a')",
		0,
	)
	if _, err := ownerTx.Exec(
		ctx,
		`UPDATE iam.organizations SET archived_at = NULL WHERE id = $1`,
		orgA,
	); err != nil {
		t.Fatalf("unarchive organization fixture: %v", err)
	}
	if err := ownerTx.Commit(ctx); err != nil {
		t.Fatalf("commit global owner transaction: %v", err)
	}

	worker := openIAMWorkerPool(t, ctx, databaseURL)
	defer worker.Close()
	assertCount(t, ctx, worker, "SELECT count(*) FROM public.platform_outbox_messages", 1)
	assertCount(
		t,
		ctx,
		worker,
		"SELECT count(*) FROM app.organization_provisioning_outbox_messages",
		1,
	)
	command, err := worker.Exec(
		ctx,
		"UPDATE public.platform_outbox_messages SET last_error = 'forbidden' WHERE id = 'worker-hidden'",
	)
	if err != nil {
		t.Fatalf("worker hidden-topic update returned an unexpected error: %v", err)
	}
	if command.RowsAffected() != 0 {
		t.Fatalf("worker updated %d hidden-topic rows", command.RowsAffected())
	}
	if _, err := worker.Exec(ctx, "SELECT count(*) FROM iam.memberships"); err == nil {
		t.Fatal("IAM worker can read memberships outside its responsibility")
	}
}

func openBackendPool(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	cfg.ConnConfig.User = "pymes_backend"
	cfg.ConnConfig.Password = "pymes_backend"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open backend pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping backend pool: %v", err)
	}
	return pool
}

func openIAMWorkerPool(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse IAM worker test database URL: %v", err)
	}
	cfg.ConnConfig.User = "pymes_iam_worker"
	cfg.ConnConfig.Password = "pymes_iam_worker"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open IAM worker pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping IAM worker pool: %v", err)
	}
	return pool
}

func openFiscalWorkerPool(
	t *testing.T,
	ctx context.Context,
	databaseURL string,
) *pgxpool.Pool {
	t.Helper()
	return openRolePool(
		t,
		ctx,
		databaseURL,
		"pymes_fiscal_worker",
		"pymes_fiscal_worker",
	)
}

func openFiscalAccountingWorkerPool(
	t *testing.T,
	ctx context.Context,
	databaseURL string,
) *pgxpool.Pool {
	t.Helper()
	return openRolePool(
		t,
		ctx,
		databaseURL,
		"pymes_fiscal_accounting_worker",
		"pymes_fiscal_accounting_worker",
	)
}

func openRolePool(
	t *testing.T,
	ctx context.Context,
	databaseURL string,
	role string,
	password string,
) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse %s test database URL: %v", role, err)
	}
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = password
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open %s pool: %v", role, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping %s pool: %v", role, err)
	}
	return pool
}

type countQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func assertCount(
	t *testing.T,
	ctx context.Context,
	querier countQuerier,
	query string,
	want int,
	args ...any,
) {
	t.Helper()
	var got int
	if err := querier.QueryRow(ctx, query, args...).Scan(&got); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if got != want {
		t.Fatalf("count = %d, want %d", got, want)
	}
}

func assertTxCount(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
	query string,
	want int,
	args ...any,
) {
	t.Helper()
	assertCount(t, ctx, tx, query, want, args...)
}
