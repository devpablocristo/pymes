package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

type dependencyQuery struct {
	sql string
	ids []uuid.UUID
}

type dependencyLockTx struct {
	pgx.Tx
	calls   []dependencyQuery
	results [][]uuid.UUID
}

func (tx *dependencyLockTx) Query(
	_ context.Context,
	query string,
	args ...any,
) (pgx.Rows, error) {
	ids, ok := args[1].([]uuid.UUID)
	if !ok {
		return nil, fmt.Errorf("dependency lock ids have type %T", args[1])
	}
	tx.calls = append(tx.calls, dependencyQuery{
		sql: strings.Join(strings.Fields(query), " "),
		ids: append([]uuid.UUID(nil), ids...),
	})
	index := len(tx.calls) - 1
	var values []uuid.UUID
	if index < len(tx.results) {
		values = tx.results[index]
	}
	return &dependencyRows{values: values}, nil
}

type dependencyRows struct {
	values []uuid.UUID
	index  int
	closed bool
}

func (rows *dependencyRows) Close() {
	rows.closed = true
}

func (rows *dependencyRows) Err() error {
	return nil
}

func (rows *dependencyRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("SELECT")
}

func (rows *dependencyRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (rows *dependencyRows) Next() bool {
	if rows.closed || rows.index >= len(rows.values) {
		rows.closed = true
		return false
	}
	rows.index++
	return true
}

func (rows *dependencyRows) Scan(destinations ...any) error {
	if rows.index == 0 || rows.index > len(rows.values) {
		return fmt.Errorf("dependency row is not positioned")
	}
	if len(destinations) != 1 {
		return fmt.Errorf("dependency row destinations = %d", len(destinations))
	}
	destination, ok := destinations[0].(*uuid.UUID)
	if !ok {
		return fmt.Errorf("dependency destination has type %T", destinations[0])
	}
	*destination = rows.values[rows.index-1]
	return nil
}

func (rows *dependencyRows) Values() ([]any, error) {
	if rows.index == 0 || rows.index > len(rows.values) {
		return nil, fmt.Errorf("dependency row is not positioned")
	}
	return []any{rows.values[rows.index-1]}, nil
}

func (rows *dependencyRows) RawValues() [][]byte {
	return nil
}

func (rows *dependencyRows) Conn() *pgx.Conn {
	return nil
}

func TestLockPostingDependenciesUsesDeterministicSharedOrder(t *testing.T) {
	t.Parallel()

	accountA := uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	accountB := uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	financialAccount := uuid.MustParse(
		"00000000-0000-0000-0000-00000000000f",
	)
	tx := &dependencyLockTx{
		results: [][]uuid.UUID{
			{accountA, accountB},
			{financialAccount},
		},
	}
	repository := &Repository{
		tx:    tx,
		orgID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	}
	err := repository.lockPostingDependencies(
		context.Background(),
		[]accounting.JournalLine{
			{AccountID: accountB},
			{AccountID: accountA},
			{AccountID: accountB},
		},
	)
	if err != nil {
		t.Fatalf("lockPostingDependencies() error = %v", err)
	}
	if len(tx.calls) != 2 {
		t.Fatalf("dependency queries = %d, want 2", len(tx.calls))
	}
	if !strings.Contains(tx.calls[0].sql, "accounting.accounts") ||
		!strings.Contains(tx.calls[1].sql, "accounting.financial_accounts") {
		t.Fatalf("dependency lock order = %#v", tx.calls)
	}
	for index, call := range tx.calls {
		if !strings.Contains(call.sql, "ORDER BY") ||
			!strings.HasSuffix(call.sql, "FOR SHARE") {
			t.Fatalf("dependency query %d is not ordered FOR SHARE: %s", index, call.sql)
		}
		if len(call.ids) != 2 ||
			call.ids[0] != accountA ||
			call.ids[1] != accountB {
			t.Fatalf("dependency query %d ids = %v", index, call.ids)
		}
	}
}

func TestLockReconciliationFinancialAccountsUsesExclusiveOrder(t *testing.T) {
	t.Parallel()

	accountA := uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	accountB := uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	tx := &dependencyLockTx{
		results: [][]uuid.UUID{{accountA, accountB}},
	}
	repository := &Repository{
		tx:    tx,
		orgID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	}
	err := repository.lockReconciliationFinancialAccounts(
		context.Background(),
		accountB,
		accountA,
		accountB,
	)
	if err != nil {
		t.Fatalf("lockReconciliationFinancialAccounts() error = %v", err)
	}
	if len(tx.calls) != 1 {
		t.Fatalf("dependency queries = %d, want 1", len(tx.calls))
	}
	call := tx.calls[0]
	if !strings.Contains(call.sql, "ORDER BY") ||
		!strings.HasSuffix(call.sql, "FOR UPDATE") {
		t.Fatalf("reconciliation dependency query = %s", call.sql)
	}
	if len(call.ids) != 2 ||
		call.ids[0] != accountA ||
		call.ids[1] != accountB {
		t.Fatalf("reconciliation dependency ids = %v", call.ids)
	}
}
