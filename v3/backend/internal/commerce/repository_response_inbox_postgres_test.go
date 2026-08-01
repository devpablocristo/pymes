package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAccountingResponseInboxCommitsWithPurchaseTransitionAndReplaysExactly(t *testing.T) {
	ctx, pool := responseInboxPostgresPool(t)
	prefix := responseInboxPostgresPrefix("atomic")
	organizationID := prefix + "_org"
	responseInboxCreateReadyOrganization(t, pool, organizationID, prefix)

	now := time.Date(2026, 7, 31, 17, 0, 0, 0, time.UTC)
	store := New(pool)
	store.Now = func() time.Time { return now }
	purchase := responseInboxPurchase(prefix+"_purchase", organizationID, "ARS", "")
	if err := store.CreatePurchaseAndQueue(ctx, purchase); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.GetPurchase(ctx, organizationID, purchase.ID)
	if err != nil {
		t.Fatal(err)
	}
	result := responseInboxAccountingEvent(prefix, purchase)

	if err := store.MarkPurchasePosted(ctx, purchase, result); err != nil {
		t.Fatal(err)
	}
	first := responseInboxPurchaseState(t, ctx, pool, organizationID, purchase.ID, result.CommandID)
	if first.status != "posted" ||
		first.journalEntryID != result.JournalEntryID ||
		first.openItemID != result.OpenItemIDs[0] ||
		first.inboxRows != 1 {
		t.Fatalf("transition and response were not committed together: %+v", first)
	}

	store.Now = func() time.Time { return now.Add(time.Hour) }
	if err := store.MarkPurchasePosted(ctx, purchase, result); err != nil {
		t.Fatalf("exact response replay failed: %v", err)
	}
	replayed := responseInboxPurchaseState(t, ctx, pool, organizationID, purchase.ID, result.CommandID)
	if replayed != first {
		t.Fatalf("exact replay changed durable state:\nfirst=%+v\nreplay=%+v", first, replayed)
	}
}

func TestAccountingResponseInboxRejectsChangedReplayAndDoesNotApplyTransition(t *testing.T) {
	ctx, pool := responseInboxPostgresPool(t)
	prefix := responseInboxPostgresPrefix("conflict")
	organizationID := prefix + "_org"
	responseInboxCreateReadyOrganization(t, pool, organizationID, prefix)

	store := New(pool)
	store.Now = func() time.Time { return time.Date(2026, 7, 31, 18, 0, 0, 0, time.UTC) }
	purchase := responseInboxPurchase(prefix+"_purchase", organizationID, "ARS", "")
	if err := store.CreatePurchaseAndQueue(ctx, purchase); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.GetPurchase(ctx, organizationID, purchase.ID)
	if err != nil {
		t.Fatal(err)
	}
	accepted := responseInboxAccountingEvent(prefix, purchase)
	if err := store.MarkPurchasePosted(ctx, purchase, accepted); err != nil {
		t.Fatal(err)
	}

	responseInboxResetPurchaseToConfirmed(t, ctx, pool, organizationID, purchase.ID)
	changed := accepted
	changed.EventID = prefix + "_changed_event"
	changed.JournalEntryID = prefix + "_changed_journal"
	changed.OpenItemIDs = []string{prefix + "_changed_open_item"}
	if err := store.MarkPurchasePosted(ctx, purchase, changed); !errors.Is(err, domain.ErrIdempotencyKeyReused) {
		t.Fatalf("changed response error=%v, want ErrIdempotencyKeyReused", err)
	}

	state := responseInboxPurchaseState(t, ctx, pool, organizationID, purchase.ID, accepted.CommandID)
	if state.status != "confirmed" || state.journalEntryID != "" || state.openItemID != "" {
		t.Fatalf("conflicting response applied a local transition: %+v", state)
	}
	if state.inboxRows != 1 ||
		state.inboxJournalEntryID != accepted.JournalEntryID ||
		state.inboxOpenItemID != accepted.OpenItemIDs[0] {
		t.Fatalf("conflicting response changed the immutable inbox: %+v", state)
	}
}

func TestAccountingResponseInboxRollsBackWhenPurchaseCannotTransition(t *testing.T) {
	ctx, pool := responseInboxPostgresPool(t)
	prefix := responseInboxPostgresPrefix("rollback")
	organizationID := prefix + "_org"
	responseInboxCreateReadyOrganization(t, pool, organizationID, prefix)

	store := New(pool)
	store.Now = func() time.Time { return time.Date(2026, 7, 31, 19, 0, 0, 0, time.UTC) }
	purchase := responseInboxPurchase(prefix+"_purchase", organizationID, "ARS", "")
	if err := store.CreatePurchaseAndQueue(ctx, purchase); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.GetPurchase(ctx, organizationID, purchase.ID)
	if err != nil {
		t.Fatal(err)
	}
	responseInboxSetPurchaseStatus(t, ctx, pool, organizationID, purchase.ID, "reversed")
	result := responseInboxAccountingEvent(prefix, purchase)

	if err := store.MarkPurchasePosted(ctx, purchase, result); err == nil {
		t.Fatal("posting a non-confirmed purchase succeeded; inbox must not commit without its local transition")
	}
	state := responseInboxPurchaseState(t, ctx, pool, organizationID, purchase.ID, result.CommandID)
	if state.status != "reversed" || state.inboxRows != 0 {
		t.Fatalf("failed transition was not atomic: %+v", state)
	}
}

func TestServiceResponseInboxEnforcesTenantRLS(t *testing.T) {
	ctx, pool := responseInboxPostgresPool(t)
	prefix := responseInboxPostgresPrefix("rls")
	organizationA := prefix + "_org_a"
	organizationB := prefix + "_org_b"
	responseInboxCreateReadyOrganization(t, pool, organizationA, prefix+"-a")
	responseInboxCreateReadyOrganization(t, pool, organizationB, prefix+"-b")

	store := New(pool)
	store.Now = func() time.Time { return time.Date(2026, 7, 31, 20, 0, 0, 0, time.UTC) }
	requests := make(map[string]string, 2)
	for index, organizationID := range []string{organizationA, organizationB} {
		purchase := responseInboxPurchase(fmt.Sprintf("%s_purchase_%d", prefix, index), organizationID, "ARS", "")
		if err := store.CreatePurchaseAndQueue(ctx, purchase); err != nil {
			t.Fatal(err)
		}
		purchase, err := store.GetPurchase(ctx, organizationID, purchase.ID)
		if err != nil {
			t.Fatal(err)
		}
		result := responseInboxAccountingEvent(fmt.Sprintf("%s_%d", prefix, index), purchase)
		if err := store.MarkPurchasePosted(ctx, purchase, result); err != nil {
			t.Fatal(err)
		}
		requests[organizationID] = result.CommandID
	}

	responseInboxPrepareRLSRole(t, ctx, pool)
	for _, organizationID := range []string{organizationA, organizationB} {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "SET LOCAL ROLE pymes_v3_response_inbox_rls_test"); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		var count int
		var visibleOrganization, visibleRequest string
		err = tx.QueryRow(ctx, `
			SELECT count(*),COALESCE(min(org_id),''),COALESCE(min(request_id),'')
			FROM app.service_response_inbox`,
		).Scan(&count, &visibleOrganization, &visibleRequest)
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if count != 1 || visibleOrganization != organizationID || visibleRequest != requests[organizationID] {
			_ = tx.Rollback(ctx)
			t.Fatalf(
				"RLS context=%s exposed count=%d org=%s request=%s",
				organizationID, count, visibleOrganization, visibleRequest,
			)
		}
		otherOrganization := organizationA
		if organizationID == organizationA {
			otherOrganization = organizationB
		}
		if err = tx.QueryRow(
			ctx,
			"SELECT count(*) FROM app.service_response_inbox WHERE org_id=$1",
			otherOrganization,
		).Scan(&count); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if count != 0 {
			_ = tx.Rollback(ctx)
			t.Fatalf("RLS context=%s read org=%s", organizationID, otherOrganization)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SET LOCAL ROLE pymes_v3_response_inbox_rls_test"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationA); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO app.service_response_inbox (
			org_id,service,operation,request_id,idempotency_key,source_version,
			snapshot_digest,correlation_id,payload_hash,response
		) VALUES ($1,'accounting','rls-cross-tenant',$2,$3,1,$4,$5,$6,'{}'::jsonb)`,
		organizationB,
		prefix+"_cross_request",
		prefix+"_cross_idempotency",
		strings.Repeat("d", 64),
		prefix+":cross",
		strings.Repeat("e", 64),
	)
	if err == nil {
		t.Fatal("RLS accepted a cross-tenant inbox insert")
	}
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) || pgError.Code != "42501" {
		t.Fatalf("cross-tenant insert error=%v, want PostgreSQL RLS violation", err)
	}
}

func TestPurchaseTaxAndForeignExchangeConstraints(t *testing.T) {
	ctx, pool := responseInboxPostgresPool(t)
	prefix := responseInboxPostgresPrefix("purchase_constraints")
	organizationID := prefix + "_org"
	responseInboxCreateReadyOrganization(t, pool, organizationID, prefix)

	store := New(pool)
	store.Now = func() time.Time { return time.Date(2026, 7, 31, 21, 0, 0, 0, time.UTC) }
	validARS := responseInboxPurchase(prefix+"_ars", organizationID, "ARS", "")
	if err := store.CreatePurchaseAndQueue(ctx, validARS); err != nil {
		t.Fatalf("valid ARS purchase rejected: %v", err)
	}
	validUSD := responseInboxPurchase(prefix+"_usd", organizationID, "USD", "875.123456")
	if err := store.CreatePurchaseAndQueue(ctx, validUSD); err != nil {
		t.Fatalf("valid FX purchase rejected: %v", err)
	}
	for _, purchase := range []domain.Purchase{validARS, validUSD} {
		persisted, err := store.GetPurchase(ctx, organizationID, purchase.ID)
		if err != nil {
			t.Fatal(err)
		}
		if persisted.Total != purchase.Total ||
			persisted.NetAmount != purchase.NetAmount ||
			persisted.ExemptAmount != purchase.ExemptAmount ||
			persisted.ExchangeRate != purchase.ExchangeRate ||
			len(persisted.VATBreakdown) != 1 ||
			persisted.VATBreakdown[0] != purchase.VATBreakdown[0] {
			t.Fatalf("purchase accounting amounts changed on persistence:\ninput=%+v\nstored=%+v", purchase, persisted)
		}
	}

	invalidTax := responseInboxPurchase(prefix+"_invalid_tax", organizationID, "ARS", "")
	invalidTax.Total.Amount = "120.00"
	invalidTax.VATBreakdown[0].TaxAmount = "20.00"
	responseInboxRequirePurchaseConstraint(
		t, ctx, pool, invalidTax, "purchases_tax_components_valid",
	)

	missingFX := responseInboxPurchase(prefix+"_missing_fx", organizationID, "USD", "")
	responseInboxRequirePurchaseConstraint(
		t, ctx, pool, missingFX, "purchases_exchange_rate_valid",
	)

	invalidARSRate := responseInboxPurchase(prefix+"_invalid_ars_rate", organizationID, "ARS", "2")
	responseInboxRequirePurchaseConstraint(
		t, ctx, pool, invalidARSRate, "purchases_exchange_rate_valid",
	)

	invalidFXRate := responseInboxPurchase(prefix+"_invalid_fx_rate", organizationID, "EUR", "0")
	responseInboxRequirePurchaseConstraint(
		t, ctx, pool, invalidFXRate, "purchases_exchange_rate_valid",
	)
}

type responseInboxPostgresState struct {
	status                string
	journalEntryID        string
	openItemID            string
	updatedAt             time.Time
	inboxRows             int
	inboxJournalEntryID   string
	inboxOpenItemID       string
	inboxPayloadHash      string
	inboxResponseReceived time.Time
}

func responseInboxPostgresPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	var inbox, purchaseConstraints bool
	if err = pool.QueryRow(ctx, `
		SELECT
			to_regclass('app.service_response_inbox') IS NOT NULL,
			EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conrelid='app.purchases'::regclass
				  AND conname IN (
				    'purchases_tax_components_valid',
				    'purchases_exchange_rate_valid'
				  )
				HAVING count(*)=2
			)`,
	).Scan(&inbox, &purchaseConstraints); err != nil {
		t.Fatal(err)
	}
	if !inbox || !purchaseConstraints {
		t.Fatalf("database is missing response inbox or purchase IVA/FX migrations")
	}
	return ctx, pool
}

func responseInboxPostgresPrefix(label string) string {
	return "ri_" + label + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func responseInboxCreateReadyOrganization(
	t *testing.T,
	pool *pgxpool.Pool,
	organizationID string,
	slug string,
) {
	t.Helper()
	if _, err := createReadyOrganization(t, pool, organizationID, organizationID, slug); err != nil {
		t.Fatal(err)
	}
}

func responseInboxPurchase(id, organizationID, currency, exchangeRate string) domain.Purchase {
	return domain.Purchase{
		ID:                  id,
		OrganizationID:      organizationID,
		SupplierRef:         id + "_supplier",
		ExternalDocumentRef: id + "_invoice",
		IssueDate:           "2026-07-31",
		Total:               domain.Money{Amount: "121.00", Currency: currency},
		NetAmount:           "100.00",
		ExemptAmount:        "0.00",
		VATBreakdown: []domain.VATBreakdownItem{{
			Rate:       "21",
			BaseAmount: "100.00",
			TaxAmount:  "21.00",
		}},
		ExchangeRate:   exchangeRate,
		SnapshotDigest: strings.Repeat("a", 64),
		CorrelationID:  id + ":correlation",
	}
}

func responseInboxAccountingEvent(prefix string, purchase domain.Purchase) domain.AccountingEvent {
	return domain.AccountingEvent{
		EventID:        prefix + "_event",
		CommandID:      prefix + "_command",
		OrganizationID: purchase.OrganizationID,
		IdempotencyKey: prefix + "_idempotency",
		SourceVersion:  1,
		SnapshotDigest: purchase.SnapshotDigest,
		Status:         "posted",
		JournalEntryID: prefix + "_journal",
		OpenItemIDs:    []string{prefix + "_open_item"},
		OccurredAt:     "2026-07-31T17:00:00Z",
		CorrelationID:  purchase.CorrelationID,
	}
}

func responseInboxPurchaseState(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	organizationID string,
	purchaseID string,
	requestID string,
) responseInboxPostgresState {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		t.Fatal(err)
	}
	var state responseInboxPostgresState
	if err = tx.QueryRow(ctx, `
		SELECT status,COALESCE(journal_entry_id,''),COALESCE(open_item_id,''),updated_at
		FROM app.purchases
		WHERE org_id=$1 AND id=$2`,
		organizationID, purchaseID,
	).Scan(&state.status, &state.journalEntryID, &state.openItemID, &state.updatedAt); err != nil {
		t.Fatal(err)
	}
	var response []byte
	if err = tx.QueryRow(ctx, `
		SELECT
			count(*),
			COALESCE(min(response->>'journal_entry_id'),''),
			COALESCE(min(response->'open_item_ids'->>0),''),
			COALESCE(min(payload_hash),''),
			COALESCE(min(received_at),'epoch'::timestamptz),
			COALESCE(min(response::text),'{}')
		FROM app.service_response_inbox
		WHERE org_id=$1 AND service='accounting' AND request_id=$2`,
		organizationID, requestID,
	).Scan(
		&state.inboxRows,
		&state.inboxJournalEntryID,
		&state.inboxOpenItemID,
		&state.inboxPayloadHash,
		&state.inboxResponseReceived,
		&response,
	); err != nil {
		t.Fatal(err)
	}
	if state.inboxRows > 0 && !json.Valid(response) {
		t.Fatalf("inbox response is not JSON: %q", response)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return state
}

func responseInboxResetPurchaseToConfirmed(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	organizationID string,
	purchaseID string,
) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE app.purchases
		SET status='confirmed',journal_entry_id=NULL,open_item_id=NULL
		WHERE org_id=$1 AND id=$2`,
		organizationID, purchaseID,
	); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func responseInboxSetPurchaseStatus(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	organizationID string,
	purchaseID string,
	status string,
) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `
		UPDATE app.purchases SET status=$1 WHERE org_id=$2 AND id=$3`,
		status, organizationID, purchaseID,
	); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func responseInboxPrepareRLSRole(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_roles
				WHERE rolname='pymes_v3_response_inbox_rls_test'
			) THEN
				CREATE ROLE pymes_v3_response_inbox_rls_test;
			END IF;
		END
		$$;
		ALTER ROLE pymes_v3_response_inbox_rls_test
			NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
		GRANT USAGE ON SCHEMA app TO pymes_v3_response_inbox_rls_test;
		GRANT SELECT,INSERT ON app.service_response_inbox
			TO pymes_v3_response_inbox_rls_test`); err != nil {
		t.Fatal(err)
	}
}

func responseInboxRequirePurchaseConstraint(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	purchase domain.Purchase,
	constraintName string,
) {
	t.Helper()
	err := responseInboxInsertRawPurchase(ctx, pool, purchase)
	if err == nil {
		t.Fatalf("invalid purchase %s passed constraint %s", purchase.ID, constraintName)
	}
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) || pgError.ConstraintName != constraintName {
		t.Fatalf(
			"invalid purchase %s error=%v constraint=%q, want %q",
			purchase.ID, err, pgErrorConstraintName(pgError), constraintName,
		)
	}
}

func responseInboxInsertRawPurchase(
	ctx context.Context,
	pool *pgxpool.Pool,
	purchase domain.Purchase,
) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(
		ctx,
		"SELECT set_config('app.org_id',$1,true)",
		purchase.OrganizationID,
	); err != nil {
		return err
	}
	breakdown, err := json.Marshal(purchase.VATBreakdown)
	if err != nil {
		return err
	}
	var exchangeRate any
	if purchase.ExchangeRate != "" {
		exchangeRate = purchase.ExchangeRate
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO app.purchases (
			id,org_id,supplier_ref,external_document_ref,issue_date,amount,currency,
			net_amount,exempt_amount,vat_breakdown,exchange_rate,status,
			snapshot_digest,request_id,actor_ref,source_version,correlation_id
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'confirmed',
			$12,'request:purchase-constraint','system:test',1,$13
		)`,
		purchase.ID,
		purchase.OrganizationID,
		purchase.SupplierRef,
		purchase.ExternalDocumentRef,
		purchase.IssueDate,
		purchase.Total.Amount,
		purchase.Total.Currency,
		purchase.NetAmount,
		purchase.ExemptAmount,
		breakdown,
		exchangeRate,
		purchase.SnapshotDigest,
		purchase.CorrelationID,
	)
	if err != nil {
		return err
	}
	// The helper is used only for negative constraint assertions. A successful
	// statement is deliberately rolled back so a regression cannot seed an
	// invalid row that would prevent the corrected migration from being
	// reapplied to the same disposable integration database.
	return nil
}

func pgErrorConstraintName(value *pgconn.PgError) string {
	if value == nil {
		return ""
	}
	return value.ConstraintName
}
