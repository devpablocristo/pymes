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

type trialBalanceSummaryRow struct {
	total         int64
	openingDebit  string
	openingCredit string
	debit         string
	credit        string
	closingDebit  string
	closingCredit string
}

func (row trialBalanceSummaryRow) Scan(destinations ...any) error {
	if len(destinations) != 7 {
		return fmt.Errorf("trial balance summary destinations = %d", len(destinations))
	}
	*destinations[0].(*int64) = row.total
	*destinations[1].(*string) = row.openingDebit
	*destinations[2].(*string) = row.openingCredit
	*destinations[3].(*string) = row.debit
	*destinations[4].(*string) = row.credit
	*destinations[5].(*string) = row.closingDebit
	*destinations[6].(*string) = row.closingCredit
	return nil
}

type trialBalanceRow struct {
	accountID      uuid.UUID
	code           string
	name           string
	class          accounting.AccountClass
	normalBalance  accounting.NormalBalance
	path           []string
	lifecycleState accounting.AccountLifecycleState
	opening        string
	debit          string
	credit         string
	closing        string
}

type trialBalanceRows struct {
	values []trialBalanceRow
	index  int
	closed bool
}

func (rows *trialBalanceRows) Close() { rows.closed = true }

func (rows *trialBalanceRows) Err() error { return nil }

func (rows *trialBalanceRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("SELECT")
}

func (rows *trialBalanceRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (rows *trialBalanceRows) Next() bool {
	if rows.closed || rows.index >= len(rows.values) {
		return false
	}
	rows.index++
	return true
}

func (rows *trialBalanceRows) Scan(destinations ...any) error {
	if rows.index == 0 || rows.index > len(rows.values) {
		return fmt.Errorf("trial balance row is not positioned")
	}
	if len(destinations) != 11 {
		return fmt.Errorf("trial balance row destinations = %d", len(destinations))
	}
	value := rows.values[rows.index-1]
	*destinations[0].(*uuid.UUID) = value.accountID
	*destinations[1].(*string) = value.code
	*destinations[2].(*string) = value.name
	*destinations[3].(*accounting.AccountClass) = value.class
	*destinations[4].(*accounting.NormalBalance) = value.normalBalance
	*destinations[5].(*[]string) = append([]string(nil), value.path...)
	*destinations[6].(*accounting.AccountLifecycleState) = value.lifecycleState
	*destinations[7].(*string) = value.opening
	*destinations[8].(*string) = value.debit
	*destinations[9].(*string) = value.credit
	*destinations[10].(*string) = value.closing
	return nil
}

func (rows *trialBalanceRows) Values() ([]any, error) { return nil, nil }

func (rows *trialBalanceRows) RawValues() [][]byte { return nil }

func (rows *trialBalanceRows) Conn() *pgx.Conn { return nil }

type trialBalanceQueryTx struct {
	pgx.Tx
	summary      trialBalanceSummaryRow
	rows         *trialBalanceRows
	summaryQuery string
	rowsQuery    string
	summaryArgs  []any
	rowsArgs     []any
}

func (tx *trialBalanceQueryTx) QueryRow(
	_ context.Context,
	query string,
	args ...any,
) pgx.Row {
	tx.summaryQuery = strings.Join(strings.Fields(query), " ")
	tx.summaryArgs = append([]any(nil), args...)
	return tx.summary
}

func (tx *trialBalanceQueryTx) Query(
	_ context.Context,
	query string,
	args ...any,
) (pgx.Rows, error) {
	tx.rowsQuery = strings.Join(strings.Fields(query), " ")
	tx.rowsArgs = append([]any(nil), args...)
	return tx.rows, nil
}

func TestListTrialBalanceCalculatesGlobalControlsAndNaturalCursor(t *testing.T) {
	t.Parallel()

	organizationID := uuid.New()
	firstAccountID := uuid.New()
	secondAccountID := uuid.New()
	from := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	tx := &trialBalanceQueryTx{
		summary: trialBalanceSummaryRow{
			total:         2,
			openingDebit:  "999999999999999999.123456",
			openingCredit: "50.123456",
			debit:         "100.000001",
			credit:        "100.000001",
			closingDebit:  "999999999999999999.123456",
			closingCredit: "50.123456",
		},
		rows: &trialBalanceRows{values: []trialBalanceRow{
			{
				accountID: firstAccountID, code: "1.2", name: "Clientes",
				class: accounting.AccountAsset, normalBalance: accounting.NormalDebit,
				path:           []string{"Activo", "Créditos", "Clientes"},
				lifecycleState: accounting.AccountActive,
				opening:        "100", debit: "25", credit: "5", closing: "120",
			},
			{
				accountID: secondAccountID, code: "2.1", name: "Proveedores",
				class: accounting.AccountLiability, normalBalance: accounting.NormalCredit,
				path:           []string{"Pasivo", "Deudas comerciales", "Proveedores"},
				lifecycleState: accounting.AccountArchived,
				opening:        "-50", debit: "0", credit: "25", closing: "-75",
			},
		}},
	}
	repository := &Repository{tx: tx, orgID: organizationID}
	page, err := repository.ListTrialBalance(context.Background(), accounting.TrialBalanceFilter{
		From:         from,
		To:           to,
		Query:        "comercial",
		AccountClass: accounting.AccountLiability,
		IncludeZero:  true,
		Limit:        1,
	})
	if err != nil {
		t.Fatalf("list trial balance: %v", err)
	}
	if page.Total != 2 || len(page.Items) != 1 {
		t.Fatalf("page size = %d/%d", len(page.Items), page.Total)
	}
	if page.Items[0].OpeningBalance.String() != "100" ||
		page.Items[0].ClosingBalance.String() != "120" ||
		strings.Join(page.Items[0].Path, "/") != "Activo/Créditos/Clientes" {
		t.Fatalf("first row = %#v", page.Items[0])
	}
	if page.NextCursor == nil ||
		page.NextCursor.Code != "1.2" ||
		page.NextCursor.AccountID != firstAccountID {
		t.Fatalf("next cursor = %#v", page.NextCursor)
	}
	if page.Totals.OpeningDifference.String() != "999999999999999949" ||
		page.Totals.MovementDifference.String() != "0" ||
		page.Totals.ClosingDifference.String() != "999999999999999949" {
		t.Fatalf("control totals = %#v", page.Totals)
	}
	if len(tx.summaryArgs) != 6 || tx.summaryArgs[0] != organizationID ||
		tx.summaryArgs[1] != from || tx.summaryArgs[2] != to ||
		tx.summaryArgs[3] != string(accounting.AccountLiability) ||
		tx.summaryArgs[4] != "comercial" || tx.summaryArgs[5] != true {
		t.Fatalf("summary args = %#v", tx.summaryArgs)
	}
	if len(tx.rowsArgs) != 9 || tx.rowsArgs[8] != 2 {
		t.Fatalf("row args = %#v", tx.rowsArgs)
	}

	for _, fragment := range []string{
		"entry.org_id = $1",
		"entry.entry_date < $2",
		"entry.entry_date BETWEEN $2 AND $3",
		"account.posting_allowed",
		"account.trashed_at IS NULL",
		"array_to_string(path, ' > ')",
		"$6::boolean AND lifecycle_state = 'active'",
	} {
		if !strings.Contains(tx.summaryQuery, fragment) {
			t.Fatalf("summary query misses %q: %s", fragment, tx.summaryQuery)
		}
	}
	for _, forbidden := range []string{
		"account.archived_at IS NULL",
		"account_class IN ('asset', 'liability', 'equity')",
	} {
		if strings.Contains(tx.summaryQuery, forbidden) {
			t.Fatalf("summary query improperly restricts history with %q: %s", forbidden, tx.summaryQuery)
		}
	}
	for _, fragment := range []string{
		"accounting.account_code_sort_key(code)",
		"accounting.account_code_sort_key($7::text)",
		"ORDER BY accounting.account_code_sort_key(code), account_id",
		"LIMIT $9",
	} {
		if !strings.Contains(tx.rowsQuery, fragment) {
			t.Fatalf("rows query misses %q: %s", fragment, tx.rowsQuery)
		}
	}
}
