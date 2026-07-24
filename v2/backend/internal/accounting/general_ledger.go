package accounting

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GeneralLedgerCursor is the stable, ascending position of a posted journal
// line. A line ID is part of the cursor so two lines with the same date,
// entry number and line number can never cause a duplicate or skipped row.
type GeneralLedgerCursor struct {
	Date        time.Time
	EntryNumber int64
	LineNumber  int
	LineID      uuid.UUID
}

func (cursor GeneralLedgerCursor) Valid() bool {
	return !cursor.Date.IsZero() &&
		cursor.EntryNumber > 0 &&
		cursor.LineNumber > 0 &&
		cursor.LineID != uuid.Nil
}

// GeneralLedgerFilter keeps the query scoped to one posting account. Query
// and Origin only affect visible rows: balances always include every movement
// in the selected period.
type GeneralLedgerFilter struct {
	AccountID uuid.UUID
	From      time.Time
	To        time.Time
	Query     string
	Origin    string
	Cursor    *GeneralLedgerCursor
	Limit     int
}

func (filter GeneralLedgerFilter) Validate() error {
	if filter.AccountID == uuid.Nil {
		return fmt.Errorf("%w: account is required", ErrInvalidArgument)
	}
	if filter.From.IsZero() || filter.To.IsZero() || filter.To.Before(filter.From) {
		return fmt.Errorf("%w: invalid general ledger period", ErrInvalidArgument)
	}
	if len(strings.TrimSpace(filter.Query)) > 160 {
		return fmt.Errorf("%w: general ledger query is too long", ErrInvalidArgument)
	}
	if len(strings.TrimSpace(filter.Origin)) > 120 {
		return fmt.Errorf("%w: general ledger origin is too long", ErrInvalidArgument)
	}
	if filter.Cursor != nil && !filter.Cursor.Valid() {
		return fmt.Errorf("%w: invalid general ledger cursor", ErrInvalidArgument)
	}
	return nil
}

func (filter GeneralLedgerFilter) normalized() GeneralLedgerFilter {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Origin = strings.TrimSpace(filter.Origin)
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	return filter
}

// GeneralLedgerMovement is a single immutable posted journal line as it is
// presented in an account's Mayor. Balance is a signed exact decimal in the
// domain; the API turns it into an absolute amount plus a debit/credit side.
type GeneralLedgerMovement struct {
	EntryID     uuid.UUID
	LineID      uuid.UUID
	EntryNumber int64
	LineNumber  int
	Date        time.Time
	Reference   string
	Origin      string
	Description string
	Memo        string
	Debit       Decimal
	Credit      Decimal
	Balance     Decimal
}

// GeneralLedgerPage is an account-specific, cursor-paginated view. Opening,
// totals and closing describe the full requested period rather than the
// current filtered page, preserving a trustworthy running balance.
type GeneralLedgerPage struct {
	Account        Account
	From           time.Time
	To             time.Time
	OpeningBalance Decimal
	ClosingBalance Decimal
	TotalDebit     Decimal
	TotalCredit    Decimal
	Items          []GeneralLedgerMovement
	Total          int
	NextCursor     *GeneralLedgerCursor
}
