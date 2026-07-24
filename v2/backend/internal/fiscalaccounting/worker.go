package fiscalaccounting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	accountingpostgres "github.com/devpablocristo/pymes/v2/backend/internal/accounting/postgres"
	"github.com/devpablocristo/pymes/v2/backend/internal/fiscal"
)

var (
	ErrNoWork           = errors.New("no fiscal accounting posting intent is ready")
	ErrExistingMismatch = errors.New("existing accounting entry does not match fiscal snapshot")
)

type Config struct {
	WorkerID     string
	ActorID      string
	PollInterval time.Duration
	RetryDelay   time.Duration
	MaxAttempts  int
}

func DefaultConfig() Config {
	return Config{
		WorkerID:     "fiscal-accounting-worker",
		ActorID:      "system:fiscal-accounting",
		PollInterval: time.Second,
		RetryDelay:   30 * time.Second,
		MaxAttempts:  10,
	}
}

func (config Config) validate() error {
	if strings.TrimSpace(config.WorkerID) == "" {
		return errors.New("fiscal accounting worker id is required")
	}
	if strings.TrimSpace(config.ActorID) == "" {
		return errors.New("fiscal accounting actor is required")
	}
	if config.PollInterval <= 0 {
		return errors.New("fiscal accounting poll interval must be positive")
	}
	if config.RetryDelay < 0 {
		return errors.New("fiscal accounting retry delay cannot be negative")
	}
	if config.MaxAttempts <= 0 {
		return errors.New("fiscal accounting max attempts must be positive")
	}
	return nil
}

type Result struct {
	OrganizationID uuid.UUID
	IntentID       uuid.UUID
	VoucherID      uuid.UUID
	JournalEntryID uuid.UUID
	Attempt        int
	Reconciled     bool
	ErrorCode      string
}

type Observer func(Result, error)

type clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now().UTC()
}

type Worker struct {
	pool   *pgxpool.Pool
	config Config
	clock  clock
}

func NewWorker(pool *pgxpool.Pool, config Config) (*Worker, error) {
	return newWorker(pool, config, systemClock{})
}

func newWorker(pool *pgxpool.Pool, config Config, workerClock clock) (*Worker, error) {
	if pool == nil {
		return nil, errors.New("fiscal accounting PostgreSQL pool is required")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	if workerClock == nil {
		return nil, errors.New("fiscal accounting clock is required")
	}
	return &Worker{pool: pool, config: config, clock: workerClock}, nil
}

// RunOnce holds a PostgreSQL row lock on one tenant-scoped intent for the
// complete local posting transaction. There is deliberately no process lease:
// snapshot validation, ledger posting, linking, and intent completion are all
// local database work and either commit together or roll back together.
func (worker *Worker) RunOnce(
	ctx context.Context,
	organizationID uuid.UUID,
) (Result, error) {
	if organizationID == uuid.Nil {
		return Result{}, errors.New("fiscal accounting organization is required")
	}
	now := worker.clock.Now().UTC()
	tx, err := worker.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Result{}, fmt.Errorf("begin fiscal accounting transaction: %w", err)
	}

	item, processResult, processErr := worker.processTransaction(
		ctx,
		tx,
		organizationID,
		now,
	)
	if processErr == nil {
		if commitErr := tx.Commit(ctx); commitErr == nil {
			return processResult, nil
		} else {
			processErr = fmt.Errorf("commit fiscal accounting transaction: %w", commitErr)
		}
	} else {
		_ = tx.Rollback(context.Background())
	}
	if item.IntentID == uuid.Nil {
		return processResult, processErr
	}
	// A commit error is potentially ambiguous. This update is safe: if the
	// original transaction committed, the terminal posted row cannot match.
	if markErr := worker.markFailed(
		ctx,
		organizationID,
		item.IntentID,
		now,
		processErr,
	); markErr != nil {
		return processResult, errors.Join(processErr, markErr)
	}
	processResult.ErrorCode = postingErrorCode(processErr)
	return processResult, processErr
}

func (worker *Worker) processTransaction(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	now time.Time,
) (workItem, Result, error) {
	if err := bindTenant(
		ctx,
		tx,
		organizationID,
		worker.config.ActorID,
	); err != nil {
		return workItem{}, Result{}, err
	}
	item, err := worker.lockNext(ctx, tx, organizationID, now)
	if err != nil {
		return workItem{}, Result{}, err
	}
	result := Result{
		OrganizationID: organizationID,
		IntentID:       item.IntentID,
		VoucherID:      item.VoucherID,
		Attempt:        item.AttemptCount + 1,
	}

	snapshot, err := fiscal.ParseSnapshot(
		item.CanonicalSnapshot,
		item.SnapshotHash,
	)
	if err != nil {
		return item, result, fmt.Errorf("validate immutable fiscal snapshot: %w", err)
	}
	document, err := snapshot.Document()
	if err != nil {
		return item, result, err
	}
	transactor, err := accountingpostgres.NewTxTransactor(tx)
	if err != nil {
		return item, result, err
	}
	service, err := accounting.NewService(transactor)
	if err != nil {
		return item, result, err
	}
	scope := accounting.Scope{
		OrganizationID:      organizationID,
		ActorID:             worker.config.ActorID,
		CanManageAccounting: true,
	}
	mappingList, err := service.ListAccountMappings(ctx, scope)
	if err != nil {
		return item, result, fmt.Errorf("load fiscal accounting mappings: %w", err)
	}
	mappings := make(map[string]accounting.AccountMapping, len(mappingList))
	for _, mapping := range mappingList {
		mappings[mapping.Role] = mapping
	}
	plan, err := buildPostingPlan(
		item,
		document,
		mappings,
		worker.config.ActorID,
	)
	if err != nil {
		return item, result, fmt.Errorf("build fiscal accounting posting: %w", err)
	}

	existing, found, err := findExistingEntry(
		ctx,
		transactor,
		scope,
		plan.Entry.Source,
	)
	if err != nil {
		return item, result, err
	}
	journalEntryID := uuid.Nil
	if found {
		if !entriesEquivalent(existing, plan.Entry) {
			return item, result, ErrExistingMismatch
		}
		journalEntryID = existing.ID
		result.Reconciled = true
	} else {
		posted, postErr := service.PostPlan(ctx, scope, plan)
		if errors.Is(postErr, accounting.ErrIdempotencyConflict) {
			existing, found, err = findExistingEntry(
				ctx,
				transactor,
				scope,
				plan.Entry.Source,
			)
			if err != nil {
				return item, result, err
			}
			if !found || !entriesEquivalent(existing, plan.Entry) {
				return item, result, ErrExistingMismatch
			}
			journalEntryID = existing.ID
			result.Reconciled = true
		} else if postErr != nil {
			return item, result, fmt.Errorf("post fiscal accounting entry: %w", postErr)
		} else {
			journalEntryID = posted.Entry.ID
		}
	}
	if journalEntryID == uuid.Nil {
		return item, result, errors.New("fiscal accounting journal entry id is empty")
	}
	if err := linkVoucher(
		ctx,
		tx,
		organizationID,
		item.VoucherID,
		journalEntryID,
		worker.config.ActorID,
	); err != nil {
		return item, result, err
	}
	attempt, err := markPosted(
		ctx,
		tx,
		organizationID,
		item.IntentID,
		journalEntryID,
		now,
	)
	if err != nil {
		return item, result, err
	}
	result.JournalEntryID = journalEntryID
	result.Attempt = attempt
	return item, result, nil
}

func (worker *Worker) lockNext(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	now time.Time,
) (workItem, error) {
	var (
		item                 workItem
		sourceID             string
		operation            string
		voucherOperation     string
		voucherSourceType    string
		voucherSourceID      string
		voucherSnapshotHash  string
		voucherAuthorityCode string
	)
	eligibleBefore := now.Add(-worker.config.RetryDelay)
	err := tx.QueryRow(ctx, `
		SELECT
			intent.id,
			intent.voucher_id,
			intent.source_type,
			intent.source_id,
			intent.operation,
			intent.snapshot_sha256,
			intent.authority_code,
			intent.attempt_count,
			voucher.operation,
			voucher.source_type,
			voucher.source_id,
			voucher.authorization_code,
			voucher.voucher_type,
			voucher.voucher_number,
			point_of_sale.code,
			snapshot.snapshot_sha256,
			snapshot.canonical_json,
			coalesce(settings.functional_currency, ''),
			associated_open_item.open_item_id,
			associated_open_item.currency_code,
			associated_open_item.remaining_currency_amount::text,
			associated_open_item.remaining_functional_amount::text
		FROM fiscal.accounting_posting_intents AS intent
		JOIN fiscal.vouchers AS voucher
		  ON voucher.org_id = intent.org_id
		 AND voucher.id = intent.voucher_id
		JOIN fiscal.points_of_sale AS point_of_sale
		  ON point_of_sale.org_id = voucher.org_id
		 AND point_of_sale.id = voucher.point_of_sale_id
		JOIN fiscal.voucher_snapshots AS snapshot
		  ON snapshot.org_id = voucher.org_id
		 AND snapshot.voucher_id = voucher.id
		LEFT JOIN accounting.organization_settings AS settings
		  ON settings.org_id = intent.org_id
		LEFT JOIN fiscal.voucher_associations AS association
		  ON association.org_id = voucher.org_id
		 AND association.voucher_id = voucher.id
		LEFT JOIN fiscal.vouchers AS associated_voucher
		  ON associated_voucher.org_id = association.org_id
		 AND associated_voucher.id = association.associated_voucher_id
		LEFT JOIN LATERAL (
			SELECT
				balance.open_item_id,
				balance.currency_code::text,
				balance.remaining_currency_amount,
				balance.remaining_functional_amount
			  FROM accounting.open_item_balances_view AS balance
			 WHERE balance.org_id = associated_voucher.org_id
			   AND balance.item_type = 'receivable'
			   AND balance.document_id = associated_voucher.source_id
			   AND balance.remaining_currency_amount > 0
			   AND balance.remaining_functional_amount > 0
			 ORDER BY balance.issued_at, balance.open_item_id
			 LIMIT 1
		) AS associated_open_item ON true
		WHERE intent.org_id = $1
		  AND intent.status IN ('pending', 'failed')
		  AND intent.attempt_count < $2
		  AND (
			intent.status = 'pending'
			OR intent.updated_at <= $3
		  )
		  AND voucher.status = 'authorized'
		ORDER BY intent.created_at, intent.id
		FOR UPDATE OF intent SKIP LOCKED
		LIMIT 1`,
		organizationID,
		worker.config.MaxAttempts,
		eligibleBefore,
	).Scan(
		&item.IntentID,
		&item.VoucherID,
		&item.SourceType,
		&sourceID,
		&operation,
		&item.SnapshotHash,
		&item.AuthorityCode,
		&item.AttemptCount,
		&voucherOperation,
		&voucherSourceType,
		&voucherSourceID,
		&voucherAuthorityCode,
		&item.VoucherType,
		&item.VoucherNumber,
		&item.PointOfSale,
		&voucherSnapshotHash,
		&item.CanonicalSnapshot,
		&item.FunctionalCode,
		&item.AssociatedOpenItemID,
		&item.AssociatedOpenCurrency,
		&item.AssociatedOpenCurrencyAmount,
		&item.AssociatedOpenFunctional,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return workItem{}, ErrNoWork
	}
	if err != nil {
		return workItem{}, fmt.Errorf("lock fiscal accounting posting intent: %w", err)
	}
	item.OrganizationID = organizationID
	item.Operation = fiscal.Operation(operation)
	item.SourceID, err = uuid.Parse(sourceID)
	if err != nil || item.SourceID == uuid.Nil {
		return item, errors.New("fiscal accounting source id is not a UUID")
	}
	if !item.Operation.Valid() ||
		voucherOperation != operation ||
		voucherSourceType != item.SourceType ||
		voucherSourceID != sourceID ||
		voucherSnapshotHash != item.SnapshotHash ||
		voucherAuthorityCode != item.AuthorityCode {
		return item, errors.New("fiscal accounting intent does not match authorized voucher")
	}
	if strings.TrimSpace(item.FunctionalCode) == "" {
		return item, errors.New("fiscal accounting organization settings are missing")
	}
	return item, nil
}

func findExistingEntry(
	ctx context.Context,
	transactor accounting.Transactor,
	scope accounting.Scope,
	source accounting.EntrySource,
) (accounting.JournalEntry, bool, error) {
	var entry accounting.JournalEntry
	err := transactor.WithinTenant(
		ctx,
		scope,
		func(ctx context.Context, repositories accounting.Repositories) error {
			var err error
			entry, err = repositories.FindEntryBySource(ctx, source)
			return err
		},
	)
	if errors.Is(err, accounting.ErrNotFound) {
		return accounting.JournalEntry{}, false, nil
	}
	if err != nil {
		return accounting.JournalEntry{}, false, fmt.Errorf(
			"find fiscal accounting entry by source: %w",
			err,
		)
	}
	return entry, true, nil
}

func linkVoucher(
	ctx context.Context,
	tx pgx.Tx,
	organizationID, voucherID, journalEntryID uuid.UUID,
	actor string,
) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO fiscal.voucher_accounting_links (
			org_id,
			voucher_id,
			journal_entry_id,
			created_by
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`,
		organizationID,
		voucherID,
		journalEntryID,
		actor,
	); err != nil {
		return fmt.Errorf("link fiscal voucher to journal entry: %w", err)
	}
	var linkedEntryID uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT journal_entry_id
		FROM fiscal.voucher_accounting_links
		WHERE org_id = $1
		  AND voucher_id = $2`,
		organizationID,
		voucherID,
	).Scan(&linkedEntryID); err != nil {
		return fmt.Errorf("verify fiscal voucher accounting link: %w", err)
	}
	if linkedEntryID != journalEntryID {
		return errors.New("fiscal voucher is linked to a different journal entry")
	}
	return nil
}

func markPosted(
	ctx context.Context,
	tx pgx.Tx,
	organizationID, intentID, journalEntryID uuid.UUID,
	now time.Time,
) (int, error) {
	var attempt int
	err := tx.QueryRow(ctx, `
		UPDATE fiscal.accounting_posting_intents
		   SET status = 'posted',
		       journal_entry_id = $3,
		       attempt_count = attempt_count + 1,
		       last_error_code = NULL,
		       last_error_detail_redacted = NULL,
		       last_attempt_at = $4,
		       posted_at = $4
		 WHERE org_id = $1
		   AND id = $2
		   AND status IN ('pending', 'failed')
		RETURNING attempt_count`,
		organizationID,
		intentID,
		journalEntryID,
		now,
	).Scan(&attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, errors.New("fiscal accounting intent was completed concurrently")
	}
	if err != nil {
		return 0, fmt.Errorf("mark fiscal accounting intent posted: %w", err)
	}
	return attempt, nil
}

func (worker *Worker) markFailed(
	ctx context.Context,
	organizationID, intentID uuid.UUID,
	now time.Time,
	cause error,
) error {
	tx, err := worker.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin fiscal accounting failure transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	if err := bindTenant(
		ctx,
		tx,
		organizationID,
		worker.config.ActorID,
	); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE fiscal.accounting_posting_intents
		   SET status = 'failed',
		       journal_entry_id = NULL,
		       attempt_count = attempt_count + 1,
		       last_error_code = $3,
		       last_error_detail_redacted = $4,
		       last_attempt_at = $5,
		       posted_at = NULL,
		       failed_at = $5
		 WHERE org_id = $1
		   AND id = $2
		   AND status IN ('pending', 'failed')`,
		organizationID,
		intentID,
		postingErrorCode(cause),
		redactedPostingError(cause),
		now,
	)
	if err != nil {
		return fmt.Errorf("mark fiscal accounting intent failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// A concurrent/ambiguous commit may already have made the intent
		// terminal. In that case there is no failure state left to record.
		var status string
		if err := tx.QueryRow(ctx, `
			SELECT status
			FROM fiscal.accounting_posting_intents
			WHERE org_id = $1
			  AND id = $2`,
			organizationID,
			intentID,
		).Scan(&status); err != nil {
			return fmt.Errorf("verify fiscal accounting failure state: %w", err)
		}
		if status != "posted" {
			return fmt.Errorf("unexpected fiscal accounting intent status %q", status)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit fiscal accounting failure transaction: %w", err)
	}
	return nil
}

func bindTenant(
	ctx context.Context,
	tx pgx.Tx,
	organizationID uuid.UUID,
	actor string,
) error {
	if _, err := tx.Exec(ctx, `
		SELECT
			set_config('app.org_id', $1, true),
			set_config('app.user_id', $2, true)`,
		organizationID.String(),
		actor,
	); err != nil {
		return fmt.Errorf("bind fiscal accounting tenant: %w", err)
	}
	return nil
}

func postingErrorCode(err error) string {
	switch {
	case errors.Is(err, accounting.ErrMappingMissing):
		return "mapping_missing"
	case errors.Is(err, accounting.ErrPeriodClosed):
		return "period_closed"
	case errors.Is(err, accounting.ErrUnbalancedEntry):
		return "unbalanced_entry"
	case errors.Is(err, accounting.ErrAccountArchived):
		return "account_archived"
	case errors.Is(err, accounting.ErrAccountNotPostable):
		return "account_not_postable"
	case errors.Is(err, accounting.ErrIdempotencyConflict),
		errors.Is(err, ErrExistingMismatch):
		return "source_conflict"
	default:
		return "posting_failed"
	}
}

func redactedPostingError(err error) string {
	return "fiscal accounting posting failed (" + postingErrorCode(err) + ")"
}

// Run polls one organization. Tenant discovery stays outside this package so
// every data access continues to require an explicit, verified organization.
func (worker *Worker) Run(
	ctx context.Context,
	organizationID uuid.UUID,
	observe Observer,
) error {
	for {
		result, err := worker.RunOnce(ctx, organizationID)
		if err != nil && !errors.Is(err, ErrNoWork) && observe != nil {
			observe(result, err)
		} else if err == nil && observe != nil {
			observe(result, nil)
		}
		timer := time.NewTimer(worker.config.PollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		case <-timer.C:
		}
	}
}
