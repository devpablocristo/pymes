package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type generalLedgerSummaryRow struct {
	opening string
	debit   string
	credit  string
	total   int64
}

func (row generalLedgerSummaryRow) Scan(destinations ...any) error {
	if len(destinations) != 4 {
		return fmt.Errorf("summary destinations = %d", len(destinations))
	}
	*destinations[0].(*string) = row.opening
	*destinations[1].(*string) = row.debit
	*destinations[2].(*string) = row.credit
	*destinations[3].(*int64) = row.total
	return nil
}

type generalLedgerRow struct {
	entryID     uuid.UUID
	lineID      uuid.UUID
	entryNumber int64
	lineNumber  int
	date        time.Time
	reference   string
	origin      string
	description string
	memo        string
	debit       string
	credit      string
	balance     string
}

type generalLedgerRows struct {
	values []generalLedgerRow
	index  int
	closed bool
}

func (rows *generalLedgerRows) Close() { rows.closed = true }

func (rows *generalLedgerRows) Err() error { return nil }

func (rows *generalLedgerRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("SELECT")
}

func (rows *generalLedgerRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (rows *generalLedgerRows) Next() bool {
	if rows.closed || rows.index >= len(rows.values) {
		return false
	}
	rows.index++
	return true
}

func (rows *generalLedgerRows) Scan(destinations ...any) error {
	if rows.index == 0 || rows.index > len(rows.values) {
		return fmt.Errorf("general ledger row is not positioned")
	}
	if len(destinations) != 12 {
		return fmt.Errorf("movement destinations = %d", len(destinations))
	}
	value := rows.values[rows.index-1]
	*destinations[0].(*uuid.UUID) = value.entryID
	*destinations[1].(*uuid.UUID) = value.lineID
	*destinations[2].(*int64) = value.entryNumber
	*destinations[3].(*int) = value.lineNumber
	*destinations[4].(*time.Time) = value.date
	*destinations[5].(*string) = value.reference
	*destinations[6].(*string) = value.origin
	*destinations[7].(*string) = value.description
	*destinations[8].(*string) = value.memo
	*destinations[9].(*string) = value.debit
	*destinations[10].(*string) = value.credit
	*destinations[11].(*string) = value.balance
	return nil
}

func (rows *generalLedgerRows) Values() ([]any, error) { return nil, nil }

func (rows *generalLedgerRows) RawValues() [][]byte { return nil }

func (rows *generalLedgerRows) Conn() *pgx.Conn { return nil }

type generalLedgerQueryTx struct {
	pgx.Tx
	summary   generalLedgerSummaryRow
	rows      *generalLedgerRows
	rowQuery  string
	lineQuery string
	rowArgs   []any
	lineArgs  []any
}

func (tx *generalLedgerQueryTx) QueryRow(
	_ context.Context,
	query string,
	args ...any,
) pgx.Row {
	tx.rowQuery = strings.Join(strings.Fields(query), " ")
	tx.rowArgs = append([]any(nil), args...)
	return tx.summary
}

func (tx *generalLedgerQueryTx) Query(
	_ context.Context,
	query string,
	args ...any,
) (pgx.Rows, error) {
	tx.lineQuery = strings.Join(strings.Fields(query), " ")
	tx.lineArgs = append([]any(nil), args...)
	return tx.rows, nil
}

func TestListGeneralLedgerCarriesOpeningIntoFilteredRunningBalance(t *testing.T) {
	t.Parallel()

	organizationID := uuid.New()
	accountID := uuid.New()
	day := time.Date(2026, time.July, 24, 0, 0, 0, 0, time.UTC)
	firstLineID := uuid.New()
	secondLineID := uuid.New()
	tx := &generalLedgerQueryTx{
		summary: generalLedgerSummaryRow{
			opening: "100", debit: "25", credit: "5", total: 2,
		},
		rows: &generalLedgerRows{values: []generalLedgerRow{
			{
				entryID: uuid.New(), lineID: firstLineID, entryNumber: 10,
				lineNumber: 1, date: day, reference: "FC-10", origin: "sale",
				description: "Venta", memo: "Primera", debit: "25", credit: "0", balance: "25",
			},
			{
				entryID: uuid.New(), lineID: secondLineID, entryNumber: 11,
				lineNumber: 1, date: day, reference: "NC-11", origin: "refund",
				description: "Nota", memo: "Segunda", debit: "0", credit: "5", balance: "20",
			},
		}},
	}
	repository := &Repository{tx: tx, orgID: organizationID}
	page, err := repository.ListGeneralLedger(context.Background(), accounting.GeneralLedgerFilter{
		AccountID: accountID,
		From:      day,
		To:        day,
		Query:     "venta",
		Origin:    "sale",
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("list general ledger: %v", err)
	}
	if page.OpeningBalance.String() != "100" || page.ClosingBalance.String() != "120" {
		t.Fatalf("period balances = %s/%s", page.OpeningBalance, page.ClosingBalance)
	}
	if len(page.Items) != 1 || page.Items[0].Balance.String() != "125" {
		t.Fatalf("page running balance = %#v", page.Items)
	}
	if page.NextCursor == nil || page.NextCursor.LineID != firstLineID {
		t.Fatalf("next cursor = %#v", page.NextCursor)
	}
	if len(tx.rowArgs) != 6 || tx.rowArgs[0] != organizationID || tx.rowArgs[1] != accountID {
		t.Fatalf("summary args = %#v", tx.rowArgs)
	}
	for _, fragment := range []string{
		"line.org_id = $1", "line.account_id = $2", "FILTER", "entry.entry_date < $3",
	} {
		if !strings.Contains(tx.rowQuery, fragment) {
			t.Fatalf("summary query misses %q: %s", fragment, tx.rowQuery)
		}
	}
	for _, fragment := range []string{
		"sum(line.debit_amount - line.credit_amount) OVER", "ORDER BY entry.entry_date, entry.entry_number, line.line_no, line.id", "lower(origin) = lower($6)",
	} {
		if !strings.Contains(tx.lineQuery, fragment) {
			t.Fatalf("line query misses %q: %s", fragment, tx.lineQuery)
		}
	}
}
