package postgres

import (
	"errors"
	"testing"

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
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := mapError(testCase.postgres); !errors.Is(err, testCase.expected) {
				t.Fatalf("mapError() = %v, want %v", err, testCase.expected)
			}
		})
	}
}

func TestNewRejectsNilPool(t *testing.T) {
	t.Parallel()

	if _, err := New(nil); !errors.Is(err, accounting.ErrInvalidArgument) {
		t.Fatalf("New(nil) error = %v", err)
	}
}
