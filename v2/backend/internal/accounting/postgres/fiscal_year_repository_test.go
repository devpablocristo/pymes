package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

type fiscalYearCountRow struct {
	total int
}

func (row fiscalYearCountRow) Scan(destinations ...any) error {
	if len(destinations) != 1 {
		return fmt.Errorf(
			"fiscal year count destinations = %d",
			len(destinations),
		)
	}
	*destinations[0].(*int) = row.total
	return nil
}

type emptyFiscalYearRows struct {
	*trialBalanceRows
}

type fiscalYearListTx struct {
	pgx.Tx
	totalQuery string
	listQuery  string
	totalArgs  []any
	listArgs   []any
	total      int
}

func (tx *fiscalYearListTx) QueryRow(
	_ context.Context,
	query string,
	args ...any,
) pgx.Row {
	tx.totalQuery = strings.Join(strings.Fields(query), " ")
	tx.totalArgs = append([]any(nil), args...)
	return fiscalYearCountRow{total: tx.total}
}

func (tx *fiscalYearListTx) Query(
	_ context.Context,
	query string,
	args ...any,
) (pgx.Rows, error) {
	tx.listQuery = strings.Join(strings.Fields(query), " ")
	tx.listArgs = append([]any(nil), args...)
	return &trialBalanceRows{}, nil
}

func TestListFiscalYearsKeepsFilteredTotalWhenCursorPageIsEmpty(
	t *testing.T,
) {
	t.Parallel()

	organizationID := uuid.New()
	tx := &fiscalYearListTx{total: 4}
	repository := &Repository{tx: tx, orgID: organizationID}
	cursor := encodeFiscalYearCursor(accounting.FiscalYearSummary{
		ID:        uuid.New(),
		StartDate: time.Date(2020, time.July, 1, 0, 0, 0, 0, time.UTC),
	})
	page, err := repository.ListFiscalYears(
		context.Background(),
		accounting.FiscalYearFilter{
			Query: "2026-06-30",
			State: accounting.FiscalYearClosed,
			After: cursor,
			Limit: 20,
		},
	)
	if err != nil {
		t.Fatalf("ListFiscalYears() error = %v", err)
	}
	if page.Total != 4 || len(page.Items) != 0 {
		t.Fatalf("fiscal year page = %d/%d", len(page.Items), page.Total)
	}
	for _, query := range []string{tx.totalQuery, tx.listQuery} {
		for _, fragment := range []string{
			"code ILIKE",
			"start_date::text ILIKE",
			"end_date::text ILIKE",
		} {
			if !strings.Contains(query, fragment) {
				t.Fatalf("query misses %q: %s", fragment, query)
			}
		}
	}
	if len(tx.totalArgs) != 3 ||
		tx.totalArgs[0] != organizationID ||
		tx.totalArgs[1] != "2026-06-30" ||
		tx.totalArgs[2] != accounting.FiscalYearClosed {
		t.Fatalf("total args = %#v", tx.totalArgs)
	}
	if len(tx.listArgs) != 6 || tx.listArgs[5] != 21 {
		t.Fatalf("list args = %#v", tx.listArgs)
	}
}

type transitionEventRow struct {
	periodID   uuid.UUID
	target     accounting.PeriodStatus
	from       int64
	localDate  time.Time
	returnDate bool
}

func (row transitionEventRow) Scan(destinations ...any) error {
	if row.returnDate {
		if len(destinations) != 1 {
			return fmt.Errorf("local date destinations = %d", len(destinations))
		}
		*destinations[0].(*time.Time) = row.localDate
		return nil
	}
	if len(destinations) != 3 {
		return fmt.Errorf(
			"period transition destinations = %d",
			len(destinations),
		)
	}
	*destinations[0].(*uuid.UUID) = row.periodID
	*destinations[1].(*accounting.PeriodStatus) = row.target
	*destinations[2].(**int64) = &row.from
	return nil
}

type transitionEventTx struct {
	pgx.Tx
	row   transitionEventRow
	query string
	args  []any
}

func (tx *transitionEventTx) QueryRow(
	_ context.Context,
	query string,
	args ...any,
) pgx.Row {
	tx.query = strings.Join(strings.Fields(query), " ")
	tx.args = append([]any(nil), args...)
	return tx.row
}

func TestPeriodTransitionWasAppliedValidatesTheOriginalCommand(t *testing.T) {
	t.Parallel()

	organizationID := uuid.New()
	periodID := uuid.New()
	tx := &transitionEventTx{row: transitionEventRow{
		periodID: periodID,
		target:   accounting.PeriodSoftClosed,
		from:     3,
	}}
	repository := &Repository{tx: tx, orgID: organizationID}
	applied, err := repository.PeriodTransitionWasApplied(
		context.Background(),
		periodID,
		accounting.PeriodSoftClosed,
		3,
		" close-july ",
	)
	if err != nil || !applied {
		t.Fatalf("PeriodTransitionWasApplied() = %v, %v", applied, err)
	}
	if !strings.Contains(tx.query, "idempotency_key = $2") ||
		strings.Contains(tx.query, "period_id =") {
		t.Fatalf("transition replay lookup is not tenant-global: %s", tx.query)
	}
	if len(tx.args) != 2 || tx.args[0] != organizationID ||
		tx.args[1] != "close-july" {
		t.Fatalf("transition replay args = %#v", tx.args)
	}

	_, err = repository.PeriodTransitionWasApplied(
		context.Background(),
		periodID,
		accounting.PeriodSoftClosed,
		2,
		"close-july",
	)
	if !errors.Is(err, accounting.ErrIdempotencyConflict) {
		t.Fatalf("payload mismatch error = %v", err)
	}
}

func TestAccountingLocalDateUsesOrganizationTimezone(t *testing.T) {
	t.Parallel()

	organizationID := uuid.New()
	expected := time.Date(2026, time.July, 24, 0, 0, 0, 0, time.UTC)
	tx := &transitionEventTx{row: transitionEventRow{
		localDate:  expected,
		returnDate: true,
	}}
	repository := &Repository{tx: tx, orgID: organizationID}
	got, err := repository.AccountingLocalDate(context.Background())
	if err != nil {
		t.Fatalf("AccountingLocalDate() error = %v", err)
	}
	if !got.Equal(expected) {
		t.Fatalf("AccountingLocalDate() = %v, want %v", got, expected)
	}
	for _, fragment := range []string{
		"AT TIME ZONE",
		"setting.timezone",
		"setting.org_id = $1",
	} {
		if !strings.Contains(tx.query, fragment) {
			t.Fatalf("local date query misses %q: %s", fragment, tx.query)
		}
	}
}
