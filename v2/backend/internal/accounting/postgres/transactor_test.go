package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

func TestMapErrorReturnsStableAccountingErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		postgres *pgconn.PgError
		expected error
	}{
		{
			name: "idempotency",
			postgres: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "accounting_journal_entries_idempotency_unique",
			},
			expected: accounting.ErrIdempotencyConflict,
		},
		{
			name: "concurrent direct reversal",
			postgres: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "accounting_journal_entries_direct_reversal_uidx",
			},
			expected: accounting.ErrAlreadyReversed,
		},
		{
			name: "balance",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_journal_entries_balanced",
			},
			expected: accounting.ErrUnbalancedEntry,
		},
		{
			name: "locked period",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_journal_entries_period_locked",
			},
			expected: accounting.ErrPeriodClosed,
		},
		{
			name: "closed reconciliation",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_journal_lines_closed_reconciliation",
			},
			expected: accounting.ErrReconciliationClosed,
		},
		{
			name: "reversal date",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_journal_entries_reversal_date",
			},
			expected: accounting.ErrInvalidArgument,
		},
		{
			name: "period date",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_journal_entries_period_date",
			},
			expected: accounting.ErrInvalidArgument,
		},
		{
			name: "currency conversion",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_journal_lines_currency_conversion",
			},
			expected: accounting.ErrInvalidArgument,
		},
		{
			name: "financial account ledger is immutable",
			postgres: &pgconn.PgError{
				Code: "55000",
				ConstraintName: "accounting_financial_accounts_" +
					"ledger_account_immutable",
			},
			expected: accounting.ErrConflict,
		},
		{
			name: "period close checklist",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_periods_close_checklist",
			},
			expected: accounting.ErrFiscalYearNotReady,
		},
		{
			name: "period pending drafts",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_periods_pending_drafts",
			},
			expected: accounting.ErrFiscalYearNotReady,
		},
		{
			name: "fiscal year close checklist",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_fiscal_years_close_checklist",
			},
			expected: accounting.ErrFiscalYearNotReady,
		},
		{
			name: "period reopen reason",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_periods_reopen_reason",
			},
			expected: accounting.ErrInvalidArgument,
		},
		{
			name: "annual close transition",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_fiscal_years_annual_transition",
			},
			expected: accounting.ErrAnnualClosePending,
		},
		{
			name: "annual close posting freeze",
			postgres: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_journal_entries_annual_close_frozen",
			},
			expected: accounting.ErrAnnualClosePending,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := mapError(testCase.postgres); !errors.Is(err, testCase.expected) {
				t.Fatalf("mapError() = %v, want %v", err, testCase.expected)
			}
		})
	}
}

type constraintExecResult struct {
	tag pgconn.CommandTag
	err error
}

type constraintTx struct {
	pgx.Tx
	calls   []string
	results []constraintExecResult
}

func (tx *constraintTx) Exec(
	_ context.Context,
	query string,
	_ ...any,
) (pgconn.CommandTag, error) {
	tx.calls = append(tx.calls, strings.Join(strings.Fields(query), " "))
	index := len(tx.calls) - 1
	if index < len(tx.results) {
		return tx.results[index].tag, tx.results[index].err
	}
	return pgconn.NewCommandTag("SET CONSTRAINTS"), nil
}

func TestValidateDeferredConstraintsForcesAndRestoresMode(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		validate func(context.Context, *Repository) error
		want     []string
	}{
		"draft": {
			validate: func(ctx context.Context, repository *Repository) error {
				return repository.validateDraftConstraints(ctx)
			},
			want: []string{
				"accounting.accounting_drafts_currency_consistency",
				"accounting.accounting_draft_lines_currency_consistency",
			},
		},
		"journal": {
			validate: func(ctx context.Context, repository *Repository) error {
				return repository.validateJournalConstraints(ctx)
			},
			want: []string{
				"accounting.accounting_journal_entries_valid",
				"accounting.accounting_journal_lines_entry_valid",
				"accounting.accounting_journal_entries_workflow_invariants",
				"accounting.accounting_journal_lines_workflow_invariants",
				"accounting.accounting_journal_lines_closed_reconciliation",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tx := &constraintTx{}
			repository := &Repository{tx: tx}
			if err := testCase.validate(context.Background(), repository); err != nil {
				t.Fatalf("validate deferred constraints: %v", err)
			}
			if len(tx.calls) != 2 {
				t.Fatalf("Exec calls = %d, want 2", len(tx.calls))
			}
			if !strings.HasSuffix(tx.calls[0], "IMMEDIATE") {
				t.Fatalf("first command = %q, want IMMEDIATE", tx.calls[0])
			}
			if !strings.HasSuffix(tx.calls[1], "DEFERRED") {
				t.Fatalf("second command = %q, want DEFERRED", tx.calls[1])
			}
			for _, constraint := range testCase.want {
				for index, call := range tx.calls {
					if !strings.Contains(call, constraint) {
						t.Fatalf(
							"command %d does not force %s: %q",
							index,
							constraint,
							call,
						)
					}
				}
			}
		})
	}
}

func TestValidateDeferredConstraintsMapsImmediateDatabaseError(t *testing.T) {
	t.Parallel()

	tx := &constraintTx{
		results: []constraintExecResult{{
			err: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "accounting_journal_lines_closed_reconciliation",
			},
		}},
	}
	repository := &Repository{tx: tx}
	err := repository.validateJournalConstraints(context.Background())
	if !errors.Is(err, accounting.ErrReconciliationClosed) {
		t.Fatalf("validate journal constraints error = %v", err)
	}
	if len(tx.calls) != 1 {
		t.Fatalf("Exec calls after failed IMMEDIATE = %d, want 1", len(tx.calls))
	}
}

func TestDiscardDraftValidatesBeforeReturningToCallerOwnedTransaction(
	t *testing.T,
) {
	t.Parallel()

	tx := &constraintTx{
		results: []constraintExecResult{
			{tag: pgconn.NewCommandTag("UPDATE 1")},
			{},
			{},
		},
	}
	repository := &Repository{
		tx:    tx,
		orgID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	}
	err := repository.DiscardDraft(
		context.Background(),
		uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		1,
		"actor",
		"reason",
	)
	if err != nil {
		t.Fatalf("DiscardDraft() error = %v", err)
	}
	if len(tx.calls) != 3 {
		t.Fatalf("Exec calls = %d, want UPDATE plus two constraint commands", len(tx.calls))
	}
	if !strings.HasPrefix(tx.calls[1], "SET CONSTRAINTS") ||
		!strings.HasSuffix(tx.calls[1], "IMMEDIATE") {
		t.Fatalf("second command = %q, want forced constraint validation", tx.calls[1])
	}
}

func TestNewRejectsNilPool(t *testing.T) {
	t.Parallel()

	if _, err := New(nil); !errors.Is(err, accounting.ErrInvalidArgument) {
		t.Fatalf("New(nil) error = %v", err)
	}
}
