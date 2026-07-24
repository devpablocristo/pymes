package accounting

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TrialBalanceCursor is the stable, natural-code position of a posting
// account in the Balance de sumas y saldos.
type TrialBalanceCursor struct {
	Code      string
	AccountID uuid.UUID
}

func (cursor TrialBalanceCursor) Valid() bool {
	return strings.TrimSpace(cursor.Code) != "" && cursor.AccountID != uuid.Nil
}

// TrialBalanceFilter describes the dedicated, cursor-paginated balance.
// AccountClass is optional; its zero value means every accounting class.
type TrialBalanceFilter struct {
	From         time.Time
	To           time.Time
	Query        string
	AccountClass AccountClass
	IncludeZero  bool
	Cursor       *TrialBalanceCursor
	Limit        int
}

func (filter TrialBalanceFilter) Validate() error {
	if filter.From.IsZero() || filter.To.IsZero() || filter.To.Before(filter.From) {
		return fmt.Errorf("%w: invalid trial balance period", ErrInvalidArgument)
	}
	if len(strings.TrimSpace(filter.Query)) > 160 {
		return fmt.Errorf("%w: trial balance query is too long", ErrInvalidArgument)
	}
	if filter.AccountClass != "" && !filter.AccountClass.Valid() {
		return fmt.Errorf("%w: invalid trial balance account class", ErrInvalidArgument)
	}
	if filter.Cursor != nil && !filter.Cursor.Valid() {
		return fmt.Errorf("%w: invalid trial balance cursor", ErrInvalidArgument)
	}
	return nil
}

func (filter TrialBalanceFilter) normalized() TrialBalanceFilter {
	filter.Query = strings.TrimSpace(filter.Query)
	if filter.Cursor != nil {
		cursor := *filter.Cursor
		cursor.Code = strings.TrimSpace(cursor.Code)
		filter.Cursor = &cursor
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	return filter
}

// TrialBalanceAccountRow keeps balances signed in the domain. Positive means
// debtor and negative means creditor; API and exports split those values into
// explicit sides so users never need to interpret a negative display amount.
type TrialBalanceAccountRow struct {
	AccountID      uuid.UUID
	Code           string
	Name           string
	Class          AccountClass
	NormalBalance  NormalBalance
	Path           []string
	LifecycleState AccountLifecycleState
	OpeningBalance Decimal
	Debit          Decimal
	Credit         Decimal
	ClosingBalance Decimal
}

// TrialBalanceTotals always describe the complete filtered result, never only
// the current page. Differences are signed debit minus credit controls.
type TrialBalanceTotals struct {
	OpeningDebit       Decimal
	OpeningCredit      Decimal
	Debit              Decimal
	Credit             Decimal
	ClosingDebit       Decimal
	ClosingCredit      Decimal
	OpeningDifference  Decimal
	MovementDifference Decimal
	ClosingDifference  Decimal
}

type TrialBalancePage struct {
	From       time.Time
	To         time.Time
	Items      []TrialBalanceAccountRow
	Totals     TrialBalanceTotals
	Total      int
	NextCursor *TrialBalanceCursor
}
