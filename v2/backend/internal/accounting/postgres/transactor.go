package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, fmt.Errorf("%w: PostgreSQL pool is required", accounting.ErrInvalidArgument)
	}
	return &Store{pool: pool}, nil
}

func (store *Store) WithinTenant(
	ctx context.Context,
	scope accounting.Scope,
	fn func(context.Context, accounting.Repositories) error,
) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("accounting postgres: begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	repository, err := bind(ctx, tx, scope)
	if err != nil {
		return err
	}
	if err := fn(ctx, repository); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return mapError(err)
	}
	return nil
}

// TxTransactor binds the service to a caller-owned pgx transaction. It is used
// when a source document and its accounting effects must commit atomically.
// Commit and rollback remain the caller's responsibility.
type TxTransactor struct {
	tx pgx.Tx
}

func NewTxTransactor(tx pgx.Tx) (*TxTransactor, error) {
	if tx == nil {
		return nil, fmt.Errorf("%w: PostgreSQL transaction is required", accounting.ErrInvalidArgument)
	}
	return &TxTransactor{tx: tx}, nil
}

func (transactor *TxTransactor) WithinTenant(
	ctx context.Context,
	scope accounting.Scope,
	fn func(context.Context, accounting.Repositories) error,
) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	repository, err := bind(ctx, transactor.tx, scope)
	if err != nil {
		return err
	}
	return fn(ctx, repository)
}

type Repository struct {
	tx    pgx.Tx
	orgID uuid.UUID
	actor string
}

func bind(ctx context.Context, tx pgx.Tx, scope accounting.Scope) (*Repository, error) {
	if _, err := tx.Exec(ctx, `
		SELECT
			set_config('app.org_id', $1, true),
			set_config('app.user_id', $2, true)
	`, scope.OrganizationID.String(), scope.ActorID); err != nil {
		return nil, fmt.Errorf("accounting postgres: apply tenant transaction context: %w", err)
	}
	return &Repository{tx: tx, orgID: scope.OrganizationID, actor: scope.ActorID}, nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return accounting.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return err
	}
	constraint := strings.ToLower(postgresError.ConstraintName)
	switch postgresError.Code {
	case "23505":
		if strings.Contains(constraint, "direct_reversal") {
			return accounting.ErrAlreadyReversed
		}
		if strings.Contains(constraint, "idempotency") {
			return fmt.Errorf("%w: %s", accounting.ErrIdempotencyConflict, postgresError.Detail)
		}
		return fmt.Errorf("%w: %s", accounting.ErrDuplicate, postgresError.Detail)
	case "40001":
		return accounting.ErrVersionConflict
	case "23P01":
		return fmt.Errorf("%w: overlapping period", accounting.ErrConflict)
	case "42501":
		return fmt.Errorf("%w: tenant context", accounting.ErrConflict)
	case "55000":
		if strings.Contains(postgresError.Message, "journal") {
			return accounting.ErrEntryImmutable
		}
		return accounting.ErrConflict
	case "23514":
		switch {
		case strings.Contains(constraint, "balanced"):
			return accounting.ErrUnbalancedEntry
		case strings.Contains(constraint, "period_locked"),
			strings.Contains(constraint, "period_soft_closed"):
			return accounting.ErrPeriodClosed
		case strings.Contains(constraint, "active_posting_account"):
			return accounting.ErrAccountNotPostable
		case strings.Contains(constraint, "closed_reconciliation"):
			return accounting.ErrReconciliationClosed
		case strings.Contains(constraint, "reversal_date"),
			strings.Contains(constraint, "functional_currency"),
			strings.Contains(constraint, "exchange_rate_date"),
			strings.Contains(constraint, "header_currency"),
			strings.Contains(constraint, "period_date"),
			strings.Contains(constraint, "currency_conversion"):
			return accounting.ErrInvalidArgument
		case strings.Contains(constraint, "direct_reversal"),
			strings.Contains(constraint, "exact_reversal"):
			return accounting.ErrAlreadyReversed
		case strings.Contains(constraint, "trash_unused"):
			return accounting.ErrAccountInUse
		case strings.Contains(constraint, "structure_locked"),
			strings.Contains(constraint, "node_type_immutable"),
			strings.Contains(constraint, "postable_leaf"):
			return accounting.ErrAccountStructureLocked
		case strings.Contains(constraint, "invalid_parent"),
			strings.Contains(constraint, "parent_not_postable"),
			strings.Contains(constraint, "parent_class"):
			return accounting.ErrInvalidAccountParent
		case strings.Contains(constraint, "hierarchy_acyclic"),
			strings.Contains(constraint, "hierarchy_cycle"):
			return accounting.ErrAccountHierarchyCycle
		case strings.Contains(constraint, "mappings_incompatible"),
			strings.Contains(constraint, "unknown_role"),
			strings.Contains(constraint, "alias_read_only"):
			return accounting.ErrMappingIncompatible
		case strings.Contains(constraint, "mapping_blocks_archive"):
			return accounting.ErrAccountMapped
		case strings.Contains(constraint, "financial_blocks_archive"):
			return accounting.ErrFinancialAccountLinked
		case strings.Contains(constraint, "active_children"):
			return accounting.ErrAccountHasActiveChildren
		case strings.Contains(constraint, "parent_inactive"):
			return accounting.ErrAccountParentInactive
		case strings.Contains(constraint, "system_protected"),
			strings.Contains(constraint, "system_key_immutable"):
			return accounting.ErrAccountProtected
		default:
			return fmt.Errorf("%w: %s", accounting.ErrConflict, postgresError.Message)
		}
	default:
		return err
	}
}

var (
	_ accounting.Transactor   = (*Store)(nil)
	_ accounting.Transactor   = (*TxTransactor)(nil)
	_ accounting.Repositories = (*Repository)(nil)
)
