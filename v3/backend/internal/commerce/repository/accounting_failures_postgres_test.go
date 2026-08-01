package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v3/backend/internal/commerce/companion"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
	"github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type periodAdjustmentAccounting struct {
	mu             sync.Mutex
	locksRemaining int
	commands       []domain.PostingCommand
}

type organizationScopedPeriodStore struct {
	*Store
	organizationID string
}

func (s organizationScopedPeriodStore) Lease(
	ctx context.Context,
	limit int,
	duration time.Duration,
) ([]domain.Event, error) {
	if limit < 1 || duration <= 0 {
		return nil, nil
	}
	now := s.Now().UTC()
	token := uuid.NewString()
	events := make([]domain.Event, 0, limit)
	for len(events) < limit {
		tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(
			ctx, "SELECT set_config('app.org_id',$1,true)", s.organizationID,
		); err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
		var event domain.Event
		err = tx.QueryRow(ctx, `
			WITH candidate AS (
			  SELECT id FROM app.outbox
			  WHERE org_id=$1 AND published_at IS NULL AND available_at <= $2
			    AND (lease_expires_at IS NULL OR lease_expires_at <= $2)
			  ORDER BY available_at,created_at
			  FOR UPDATE SKIP LOCKED
			  LIMIT 1
			)
			UPDATE app.outbox value
			SET lease_token=$3,lease_expires_at=$4,attempts=value.attempts+1
			FROM candidate
			WHERE value.id=candidate.id
			RETURNING value.id,value.org_id,value.topic,value.payload,
			          value.payload_hash,value.idempotency_key,value.request_id,
			          value.actor_ref,value.source_version,value.snapshot_digest,
			          value.correlation_id,value.available_at,value.attempts,
			          value.lease_token,value.lease_expires_at`,
			s.organizationID, now, token, now.Add(duration),
		).Scan(
			&event.ID, &event.OrganizationID, &event.Topic, &event.Payload,
			&event.PayloadHash, &event.IdempotencyKey, &event.RequestID,
			&event.ActorRef, &event.SourceVersion, &event.SnapshotDigest,
			&event.CorrelationID, &event.AvailableAt, &event.Attempts,
			&event.LeaseToken, &event.LeaseExpiresAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			break
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (a *periodAdjustmentAccounting) Post(
	_ context.Context,
	command domain.PostingCommand,
) (domain.AccountingEvent, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.commands = append(a.commands, command)
	if a.locksRemaining > 0 {
		a.locksRemaining--
		return domain.AccountingEvent{}, domain.ErrPeriodLocked
	}
	return domain.AccountingEvent{
		EventID: uuid.NewString(), CommandID: command.CommandID,
		OrganizationID: command.OrganizationID,
		IdempotencyKey: command.IdempotencyKey,
		SourceVersion:  command.SourceVersion,
		SnapshotDigest: command.SnapshotDigest,
		Status:         "posted", JournalEntryID: uuid.NewString(),
		OpenItemIDs:   []string{uuid.NewString()},
		OccurredAt:    time.Now().UTC().Format(time.RFC3339Nano),
		CorrelationID: command.CorrelationID,
	}, nil
}

func (*periodAdjustmentAccounting) Reverse(
	context.Context,
	domain.ReversalCommand,
) (domain.AccountingEvent, error) {
	return domain.AccountingEvent{}, errors.New("unexpected reversal")
}

func (*periodAdjustmentAccounting) ApplyOpenItem(
	context.Context,
	domain.AccountingApplicationCommand,
) (domain.AccountingEvent, error) {
	return domain.AccountingEvent{}, errors.New("unexpected application")
}

func (*periodAdjustmentAccounting) ReverseOpenItemApplication(
	context.Context,
	domain.AccountingApplicationReversalCommand,
) (domain.AccountingEvent, error) {
	return domain.AccountingEvent{}, errors.New("unexpected application reversal")
}

func TestPeriodLockedBecomesTenantVisibleAndAdjustmentConvergesExactlyOnce(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	prefix := fmt.Sprintf("period_%d", time.Now().UnixNano())
	organizationID := prefix + "_org"
	otherOrganizationID := prefix + "_other"
	if _, err := createReadyOrganization(t, pool, organizationID, prefix, prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := createReadyOrganization(
		t, pool, otherOrganizationID, prefix+" other", prefix+"-other",
	); err != nil {
		t.Fatal(err)
	}
	store := New(pool)
	clock := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return clock }
	purchase := domain.Purchase{
		ID: prefix + "_purchase", OrganizationID: organizationID,
		SupplierRef: "supplier", ExternalDocumentRef: prefix + "_invoice",
		IssueDate: "2026-07-31",
		Total:     domain.Money{Amount: "121", Currency: "ARS"},
		NetAmount: "100", ExemptAmount: "0",
		VATBreakdown: []domain.VATBreakdownItem{
			{Rate: "21", BaseAmount: "100", TaxAmount: "21"},
		},
		SnapshotDigest: commandPayloadDigestForTest([]byte(prefix)),
		Origin: domain.OriginMetadata{
			RequestID: "request-" + prefix, CorrelationID: "correlation-" + prefix,
			ActorRef: "user-" + prefix, SourceVersion: 3,
		},
		CorrelationID: "correlation-" + prefix,
	}
	if err := store.CreatePurchaseAndQueue(ctx, purchase); err != nil {
		t.Fatal(err)
	}
	accounting := &periodAdjustmentAccounting{locksRemaining: 2}
	scopedStore := organizationScopedPeriodStore{
		Store: store, organizationID: organizationID,
	}
	worker := usecases.DurableWorker{
		Store: scopedStore, Fiscal: companion.NewFakeFiscal(), Accounting: accounting,
		LeaseFor: time.Minute, MaxAttempts: 3,
	}
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	lockedPurchase, err := store.GetPurchase(ctx, organizationID, purchase.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lockedPurchase.Status != domain.AccountingAdjustmentRequired ||
		lockedPurchase.AccountingFailureID == "" ||
		lockedPurchase.AccountingFailureCode != domain.ErrPeriodLocked.Error() ||
		lockedPurchase.JournalEntryID != "" {
		t.Fatalf("locked purchase=%+v", lockedPurchase)
	}
	failure, err := store.GetAccountingFailure(
		ctx, organizationID, lockedPurchase.AccountingFailureID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if failure.Status != "awaiting_adjustment" ||
		failure.SourceKind != "purchase" ||
		failure.SourceID != purchase.ID ||
		failure.CommandKind != "posting" ||
		failure.Origin.RequestID != purchase.Origin.RequestID ||
		failure.Origin.ActorRef != purchase.Origin.ActorRef {
		t.Fatalf("failure=%+v", failure)
	}
	var rejected domain.PostingCommand
	if err := json.Unmarshal(failure.CommandPayload, &rejected); err != nil {
		t.Fatal(err)
	}
	if rejected.EffectiveAt.Format(time.DateOnly) != purchase.IssueDate ||
		rejected.SnapshotDigest != purchase.SnapshotDigest {
		t.Fatalf("rejected command was not preserved: %+v", rejected)
	}
	if _, err := store.GetAccountingFailure(
		ctx, otherOrganizationID, failure.ID,
	); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-tenant failure read error=%v", err)
	}

	effectiveAt := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	adjustment := domain.AccountingAdjustment{
		ID: prefix + "_adjustment", OrganizationID: organizationID,
		FailureID: failure.ID, EffectiveAt: effectiveAt,
		Reason: "July is locked; post in the authorized August period",
		Origin: domain.OriginMetadata{
			RequestID:     "adjustment-request-" + prefix,
			CorrelationID: "adjustment-correlation-" + prefix,
			ActorRef:      "admin-" + prefix, SourceVersion: 1,
		},
		CorrelationID: "adjustment-correlation-" + prefix,
	}
	command := idempotencyCommand(
		t, "adjustment-key-"+prefix, organizationID,
		domain.OperationCreateAccountingAdjustment, adjustment.ID, 1,
		struct {
			ID          string    `json:"id"`
			EffectiveAt time.Time `json:"effective_at"`
			Reason      string    `json:"reason"`
		}{adjustment.ID, adjustment.EffectiveAt, adjustment.Reason},
	)
	command.RequestID = adjustment.Origin.RequestID
	command.CorrelationID = adjustment.Origin.CorrelationID
	command.ActorRef = adjustment.Origin.ActorRef
	first, err := store.RequestAccountingAdjustmentIdempotent(
		ctx, command, failure.ID, adjustment,
	)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := store.RequestAccountingAdjustmentIdempotent(
		ctx, command, failure.ID, adjustment,
	)
	if err != nil || replayed.ID != first.ID || replayed.FailureID != first.FailureID {
		t.Fatalf("adjustment replay=%+v first=%+v err=%v", replayed, first, err)
	}
	pendingPurchase, err := store.GetPurchase(ctx, organizationID, purchase.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pendingPurchase.Status != domain.AccountingAdjustmentPending ||
		pendingPurchase.AccountingFailureID != failure.ID {
		t.Fatalf("pending purchase=%+v", pendingPurchase)
	}

	clock = clock.Add(time.Minute)
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	relockedPurchase, err := store.GetPurchase(ctx, organizationID, purchase.ID)
	if err != nil {
		t.Fatal(err)
	}
	relockedFailure, err := store.GetAccountingFailure(ctx, organizationID, failure.ID)
	if err != nil {
		t.Fatal(err)
	}
	relockedAdjustment, err := store.GetAccountingAdjustment(
		ctx, organizationID, adjustment.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if relockedPurchase.Status != domain.AccountingAdjustmentRequired ||
		relockedFailure.Status != "awaiting_adjustment" ||
		!relockedFailure.FailedEffectiveAt.Equal(effectiveAt) ||
		relockedAdjustment.Status != "period_locked" {
		t.Fatalf(
			"relocked purchase=%+v failure=%+v adjustment=%+v",
			relockedPurchase, relockedFailure, relockedAdjustment,
		)
	}
	var stillOriginal domain.PostingCommand
	if err := json.Unmarshal(relockedFailure.CommandPayload, &stillOriginal); err != nil {
		t.Fatal(err)
	}
	if stillOriginal.CommandID != rejected.CommandID ||
		stillOriginal.IdempotencyKey != rejected.IdempotencyKey ||
		!stillOriginal.EffectiveAt.Equal(rejected.EffectiveAt) {
		t.Fatalf(
			"the original rejected command was overwritten: original=%+v current=%+v",
			rejected, stillOriginal,
		)
	}
	secondEffectiveAt := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	secondAdjustment := domain.AccountingAdjustment{
		ID: prefix + "_adjustment_2", OrganizationID: organizationID,
		FailureID: failure.ID, EffectiveAt: secondEffectiveAt,
		Reason: "post in the next authorized open day",
		Origin: domain.OriginMetadata{
			RequestID:     "adjustment-request-2-" + prefix,
			CorrelationID: "adjustment-correlation-2-" + prefix,
			ActorRef:      "admin-" + prefix, SourceVersion: 2,
		},
		CorrelationID: "adjustment-correlation-2-" + prefix,
	}
	secondCommand := idempotencyCommand(
		t, "adjustment-key-2-"+prefix, organizationID,
		domain.OperationCreateAccountingAdjustment, secondAdjustment.ID, 2,
		struct {
			ID          string    `json:"id"`
			EffectiveAt time.Time `json:"effective_at"`
			Reason      string    `json:"reason"`
		}{
			secondAdjustment.ID,
			secondAdjustment.EffectiveAt,
			secondAdjustment.Reason,
		},
	)
	secondCommand.RequestID = secondAdjustment.Origin.RequestID
	secondCommand.CorrelationID = secondAdjustment.Origin.CorrelationID
	secondCommand.ActorRef = secondAdjustment.Origin.ActorRef
	if _, err := store.RequestAccountingAdjustmentIdempotent(
		ctx, secondCommand, failure.ID, secondAdjustment,
	); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Minute)
	if err := worker.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	postedPurchase, err := store.GetPurchase(ctx, organizationID, purchase.ID)
	if err != nil {
		t.Fatal(err)
	}
	if postedPurchase.Status != "posted" ||
		postedPurchase.JournalEntryID == "" ||
		postedPurchase.AccountingFailureID != "" ||
		postedPurchase.AccountingFailureCode != "" {
		t.Fatalf("posted purchase=%+v", postedPurchase)
	}
	resolvedFailure, err := store.GetAccountingFailure(ctx, organizationID, failure.ID)
	if err != nil {
		t.Fatal(err)
	}
	firstResolvedAdjustment, err := store.GetAccountingAdjustment(
		ctx, organizationID, adjustment.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	resolvedAdjustment, err := store.GetAccountingAdjustment(
		ctx, organizationID, secondAdjustment.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedFailure.Status != "resolved" ||
		firstResolvedAdjustment.Status != "period_locked" ||
		resolvedAdjustment.Status != "posted" {
		t.Fatalf(
			"failure=%+v first adjustment=%+v adjustment=%+v",
			resolvedFailure, firstResolvedAdjustment, resolvedAdjustment,
		)
	}
	if len(accounting.commands) != 3 ||
		!accounting.commands[1].EffectiveAt.Equal(effectiveAt) ||
		!accounting.commands[2].EffectiveAt.Equal(secondEffectiveAt) ||
		accounting.commands[0].CommandID == accounting.commands[1].CommandID ||
		accounting.commands[1].CommandID == accounting.commands[2].CommandID ||
		accounting.commands[0].IdempotencyKey == accounting.commands[1].IdempotencyKey ||
		accounting.commands[1].IdempotencyKey == accounting.commands[2].IdempotencyKey ||
		accounting.commands[0].SnapshotDigest != accounting.commands[1].SnapshotDigest ||
		accounting.commands[1].SnapshotDigest != accounting.commands[2].SnapshotDigest {
		t.Fatalf("accounting commands=%+v", accounting.commands)
	}
	var deadLetters int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*) FROM app.outbox_dead_letters WHERE org_id=$1`,
		organizationID,
	).Scan(&deadLetters); err != nil {
		t.Fatal(err)
	}
	if deadLetters != 0 {
		t.Fatalf("period lock created %d dead letters", deadLetters)
	}
}

func commandPayloadDigestForTest(payload []byte) string {
	return commandDigest(payload)
}

func TestAccountingFailureAndAdjustmentTablesEnforceTenantRLS(t *testing.T) {
	databaseURL := os.Getenv("PYMES_DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("PYMES_DATABASE_TEST_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	prefix := fmt.Sprintf("period_rls_%d", time.Now().UnixNano())
	organizations := []string{prefix + "_a", prefix + "_b"}
	for index, organizationID := range organizations {
		if _, err := createReadyOrganization(
			t, pool, organizationID,
			fmt.Sprintf("%s %d", prefix, index),
			fmt.Sprintf("%s-%d", prefix, index),
		); err != nil {
			t.Fatal(err)
		}
		failureID, originalEventID := uuid.New(), uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO app.accounting_failures (
				id,org_id,original_event_id,source_kind,source_id,command_kind,
				command_payload,command_digest,failed_effective_at,status,failure_code,
				request_id,actor_ref,source_version,snapshot_digest,correlation_id
			) VALUES (
				$1,$2,$3,'purchase',$4,'posting','{}'::jsonb,$5,now(),
				'awaiting_adjustment','PERIOD_LOCKED',$6,$7,1,$8,$9
			)`,
			failureID, organizationID, originalEventID, prefix+"_purchase",
			commandDigest([]byte("{}")), prefix+"_request", prefix+"_actor",
			commandDigest([]byte(organizationID)), prefix+"_correlation",
		); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO app.accounting_adjustments (
				id,org_id,failure_id,effective_at,reason,status,
				request_id,actor_ref,source_version,snapshot_digest,correlation_id
			) VALUES (
				$1,$2,$3,now(),'RLS test','period_locked',
				$4,$5,1,$6,$7
			)`,
			prefix+"_adjustment", organizationID, failureID,
			prefix+"_request", prefix+"_actor",
			commandDigest([]byte("adjustment:"+organizationID)),
			prefix+"_correlation",
		); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `
		DO $$
		BEGIN
		  IF NOT EXISTS (
		    SELECT 1 FROM pg_roles
		    WHERE rolname='pymes_v3_accounting_failure_rls_test'
		  ) THEN
		    CREATE ROLE pymes_v3_accounting_failure_rls_test;
		  END IF;
		END
		$$;
		GRANT USAGE ON SCHEMA app TO pymes_v3_accounting_failure_rls_test;
		GRANT SELECT ON app.accounting_failures,app.accounting_adjustments
		  TO pymes_v3_accounting_failure_rls_test;
	`); err != nil {
		t.Fatal(err)
	}
	for _, organizationID := range organizations {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(
			ctx, "SET LOCAL ROLE pymes_v3_accounting_failure_rls_test",
		); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err = tx.Exec(
			ctx, "SELECT set_config('app.org_id',$1,true)", organizationID,
		); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		var failures, adjustments int
		var failureOrganization, adjustmentOrganization string
		if err = tx.QueryRow(ctx, `
			SELECT count(*),COALESCE(min(org_id),'')
			FROM app.accounting_failures`,
		).Scan(&failures, &failureOrganization); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err = tx.QueryRow(ctx, `
			SELECT count(*),COALESCE(min(org_id),'')
			FROM app.accounting_adjustments`,
		).Scan(&adjustments, &adjustmentOrganization); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if failures != 1 || adjustments != 1 ||
			failureOrganization != organizationID ||
			adjustmentOrganization != organizationID {
			_ = tx.Rollback(ctx)
			t.Fatalf(
				"org=%s failures=%d/%s adjustments=%d/%s",
				organizationID, failures, failureOrganization,
				adjustments, adjustmentOrganization,
			)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
}
