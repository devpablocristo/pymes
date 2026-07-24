package migrate

import (
	"context"
	"strings"
	"testing"

	postgres "github.com/devpablocristo/platform/databases/postgres/go"
	"github.com/jackc/pgx/v5"
)

func assertHomologationEvidenceInvariants(
	t *testing.T,
	ctx context.Context,
	database *postgres.DB,
	databaseURL, orgA, orgB, userID string,
) {
	t.Helper()
	const (
		runID      = "00000000-0000-0000-0000-00000000f101"
		startedAt  = "2026-07-24T12:00:00Z"
		finishedAt = "2026-07-24T12:01:00Z"
	)
	checkHash := strings.Repeat("a", 64)
	runHash := strings.Repeat("b", 64)
	fingerprint := strings.Repeat("c", 64)
	configurationHash := strings.Repeat("d", 64)

	var (
		canSelectRuns, canInsertRuns, canUpdateRuns, canDeleteRuns         bool
		canSelectChecks, canInsertChecks, canUpdateChecks, canDeleteChecks bool
	)
	if err := database.QueryRow(ctx, `
		SELECT
			has_table_privilege('pymes_backend', 'fiscal.homologation_runs', 'SELECT'),
			has_table_privilege('pymes_backend', 'fiscal.homologation_runs', 'INSERT'),
			has_table_privilege('pymes_backend', 'fiscal.homologation_runs', 'UPDATE'),
			has_table_privilege('pymes_backend', 'fiscal.homologation_runs', 'DELETE'),
			has_table_privilege('pymes_backend', 'fiscal.homologation_checks', 'SELECT'),
			has_table_privilege('pymes_backend', 'fiscal.homologation_checks', 'INSERT'),
			has_table_privilege('pymes_backend', 'fiscal.homologation_checks', 'UPDATE'),
			has_table_privilege('pymes_backend', 'fiscal.homologation_checks', 'DELETE')
	`).Scan(
		&canSelectRuns,
		&canInsertRuns,
		&canUpdateRuns,
		&canDeleteRuns,
		&canSelectChecks,
		&canInsertChecks,
		&canUpdateChecks,
		&canDeleteChecks,
	); err != nil {
		t.Fatalf("inspect homologation evidence grants: %v", err)
	}
	if !canSelectRuns || !canInsertRuns || !canUpdateRuns || canDeleteRuns ||
		!canSelectChecks || !canInsertChecks || canUpdateChecks || canDeleteChecks {
		t.Fatalf(
			"unexpected homologation grants runs=%t/%t/%t/%t checks=%t/%t/%t/%t",
			canSelectRuns, canInsertRuns, canUpdateRuns, canDeleteRuns,
			canSelectChecks, canInsertChecks, canUpdateChecks, canDeleteChecks,
		)
	}

	backend := openBackendPool(t, ctx, databaseURL)
	defer backend.Close()
	tx := beginTenantTx(t, ctx, backend, orgA, userID)
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.homologation_runs (
			org_id, id, requested_by, started_at
		)
		VALUES ($1, $2, 'integration-test', $3::timestamptz)
	`, orgA, runID, startedAt); err != nil {
		t.Fatalf("insert running homologation evidence: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.homologation_checks (
			org_id, run_id, ordinal, kind, name, status,
			detail_redacted, evidence, evidence_sha256,
			started_at, completed_at
		)
		VALUES (
			$1, $2, 1, 'configuration', 'configuration', 'succeeded',
			'Configuration validated.', '{"validated":true}'::jsonb, $3,
			$4::timestamptz, $5::timestamptz
		)
	`, orgA, runID, checkHash, startedAt, finishedAt); err != nil {
		t.Fatalf("insert homologation evidence check: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE fiscal.homologation_runs
		   SET status = 'succeeded',
		       certificate_fingerprint_sha256 = $3,
		       configuration_sha256 = $4,
		       point_of_sale_count = 1,
		       check_count = 1,
		       success_count = 1,
		       failure_count = 0,
		       evidence = '{"read_only":true}'::jsonb,
		       evidence_sha256 = $5,
		       completed_at = $6::timestamptz
		 WHERE org_id = $1
		   AND id = $2
	`, orgA, runID, fingerprint, configurationHash, runHash, finishedAt); err != nil {
		t.Fatalf("finalize homologation evidence: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit finalized homologation evidence: %v", err)
	}
	var storedConfigurationHash string
	if err := database.QueryRow(ctx, `
		SELECT configuration_sha256::text
		  FROM fiscal.homologation_runs
		 WHERE org_id = $1
		   AND id = $2
	`, orgA, runID).Scan(&storedConfigurationHash); err != nil {
		t.Fatalf("read finalized homologation configuration fingerprint: %v", err)
	}
	if storedConfigurationHash != configurationHash {
		t.Fatalf(
			"configuration fingerprint = %q, want %q",
			storedConfigurationHash,
			configurationHash,
		)
	}

	if _, err := database.Exec(ctx, `
		UPDATE fiscal.homologation_runs
		   SET evidence_note = 'privileged rewrite'
		 WHERE org_id = $1 AND id = $2
	`, orgA, runID); err == nil {
		t.Fatal("finalized homologation run bypassed its immutability trigger")
	}
	if _, err := database.Exec(ctx, `
		UPDATE fiscal.homologation_checks
		   SET detail_redacted = 'privileged rewrite'
		 WHERE org_id = $1 AND run_id = $2 AND ordinal = 1
	`, orgA, runID); err == nil {
		t.Fatal("homologation check bypassed its immutability trigger")
	}
	if _, err := database.Exec(ctx, `
		DELETE FROM fiscal.homologation_runs
		 WHERE org_id = $1 AND id = $2
	`, orgA, runID); err == nil {
		t.Fatal("homologation run bypassed its delete-protection trigger")
	}
	if _, err := database.Exec(ctx, `
		INSERT INTO fiscal.homologation_checks (
			org_id, run_id, ordinal, kind, name, status,
			detail_redacted, evidence, evidence_sha256,
			started_at, completed_at
		)
		VALUES (
			$1, $2, 2, 'local_matrix', 'privileged-late-check', 'succeeded',
			'Late check.', '{"late":true}'::jsonb, $3,
			$4::timestamptz, $5::timestamptz
		)
	`, orgA, runID, checkHash, startedAt, finishedAt); err == nil {
		t.Fatal("a finalized homologation run accepted a privileged late check")
	}

	assertHomologationMutationRejected(t, ctx, backend, orgA, userID, `
		UPDATE fiscal.homologation_runs
		   SET evidence_note = 'rewritten'
		 WHERE org_id = $1 AND id = $2
	`, orgA, runID)
	assertHomologationMutationRejected(t, ctx, backend, orgA, userID, `
		UPDATE fiscal.homologation_checks
		   SET detail_redacted = 'rewritten'
		 WHERE org_id = $1 AND run_id = $2 AND ordinal = 1
	`, orgA, runID)
	assertHomologationMutationRejected(t, ctx, backend, orgA, userID, `
		DELETE FROM fiscal.homologation_runs
		 WHERE org_id = $1 AND id = $2
	`, orgA, runID)
	assertHomologationMutationRejected(t, ctx, backend, orgA, userID, `
		INSERT INTO fiscal.homologation_checks (
			org_id, run_id, ordinal, kind, name, status,
			detail_redacted, evidence, evidence_sha256,
			started_at, completed_at
		)
		VALUES (
			$1, $2, 2, 'local_matrix', 'late-check', 'succeeded',
			'Late check.', '{"late":true}'::jsonb, $3,
			$4::timestamptz, $5::timestamptz
		)
	`, orgA, runID, checkHash, startedAt, finishedAt)

	otherTenant := beginTenantTx(t, ctx, backend, orgB, userID)
	var visible int
	if err := otherTenant.QueryRow(ctx, `
		SELECT count(*)
		  FROM fiscal.homologation_runs
		 WHERE id = $1
	`, runID).Scan(&visible); err != nil {
		_ = otherTenant.Rollback(ctx)
		t.Fatalf("query homologation evidence from another tenant: %v", err)
	}
	if visible != 0 {
		_ = otherTenant.Rollback(ctx)
		t.Fatal("homologation evidence crossed the tenant boundary")
	}
	if err := otherTenant.Commit(ctx); err != nil {
		t.Fatalf("commit other-tenant homologation read: %v", err)
	}
}

func assertHomologationMutationRejected(
	t *testing.T,
	ctx context.Context,
	backend interface {
		Begin(context.Context) (pgx.Tx, error)
	},
	orgID, userID, statement string,
	arguments ...any,
) {
	t.Helper()
	tx := beginTenantTx(t, ctx, backend, orgID, userID)
	if _, err := tx.Exec(ctx, statement, arguments...); err == nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("homologation evidence mutation unexpectedly succeeded: %s", statement)
	}
	_ = tx.Rollback(ctx)
}
