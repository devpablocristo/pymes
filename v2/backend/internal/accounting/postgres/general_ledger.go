package postgres

import (
	"context"
	"fmt"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

// ListGeneralLedger retrieves only the journal lines for the selected
// account. The period running balance is calculated before query/origin
// filtering so a user never sees a fabricated balance after narrowing the
// visible rows.
func (repository *Repository) ListGeneralLedger(
	ctx context.Context,
	filter accounting.GeneralLedgerFilter,
) (accounting.GeneralLedgerPage, error) {
	result := accounting.GeneralLedgerPage{
		From:  filter.From,
		To:    filter.To,
		Items: make([]accounting.GeneralLedgerMovement, 0, filter.Limit),
	}

	var (
		openingText string
		debitText   string
		creditText  string
		total       int64
	)
	if err := repository.tx.QueryRow(ctx, `
		SELECT
			coalesce(
				sum(line.debit_amount - line.credit_amount)
					FILTER (WHERE entry.entry_date < $3),
				0
			)::text,
			coalesce(
				sum(line.debit_amount)
					FILTER (WHERE entry.entry_date BETWEEN $3 AND $4),
				0
			)::text,
			coalesce(
				sum(line.credit_amount)
					FILTER (WHERE entry.entry_date BETWEEN $3 AND $4),
				0
			)::text,
			count(*) FILTER (
				WHERE entry.entry_date BETWEEN $3 AND $4
				  AND (
					$5::text = ''
					OR entry.entry_number::text = $5
					OR coalesce(entry.reference, '') ILIKE '%' || $5 || '%'
					OR coalesce(nullif(entry.source_type, ''), entry.entry_kind) ILIKE '%' || $5 || '%'
					OR entry.description ILIKE '%' || $5 || '%'
					OR line.description ILIKE '%' || $5 || '%'
				  )
				  AND (
					$6::text = ''
					OR lower(coalesce(nullif(entry.source_type, ''), entry.entry_kind)) = lower($6)
				  )
			)
		  FROM accounting.journal_lines AS line
		  JOIN accounting.journal_entries AS entry
			ON entry.org_id = line.org_id
		   AND entry.id = line.journal_entry_id
		 WHERE line.org_id = $1
		   AND line.account_id = $2
	`,
		repository.orgID,
		filter.AccountID,
		filter.From,
		filter.To,
		filter.Query,
		filter.Origin,
	).Scan(&openingText, &debitText, &creditText, &total); err != nil {
		return accounting.GeneralLedgerPage{}, mapError(err)
	}
	var err error
	if result.OpeningBalance, err = accounting.ParseAmount(openingText); err != nil {
		return accounting.GeneralLedgerPage{}, err
	}
	if result.TotalDebit, err = accounting.ParseAmount(debitText); err != nil {
		return accounting.GeneralLedgerPage{}, err
	}
	if result.TotalCredit, err = accounting.ParseAmount(creditText); err != nil {
		return accounting.GeneralLedgerPage{}, err
	}
	result.ClosingBalance = result.OpeningBalance.Add(result.TotalDebit).Sub(result.TotalCredit)
	result.Total = int(total)

	var (
		cursorDate   any
		cursorNumber int64
		cursorLineNo int
		cursorLineID any
	)
	if filter.Cursor != nil {
		cursorDate = filter.Cursor.Date
		cursorNumber = filter.Cursor.EntryNumber
		cursorLineNo = filter.Cursor.LineNumber
		cursorLineID = filter.Cursor.LineID
	}

	rows, err := repository.tx.Query(ctx, `
		WITH period_lines AS (
			SELECT
				entry.id AS entry_id,
				line.id AS line_id,
				entry.entry_number,
				line.line_no,
				entry.entry_date,
				coalesce(entry.reference, '') AS reference,
				coalesce(nullif(entry.source_type, ''), entry.entry_kind) AS origin,
				entry.description,
				line.description AS memo,
				line.debit_amount::text AS debit,
				line.credit_amount::text AS credit,
				sum(line.debit_amount - line.credit_amount) OVER (
					ORDER BY entry.entry_date, entry.entry_number, line.line_no, line.id
					ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
				)::text AS period_balance
			  FROM accounting.journal_lines AS line
			  JOIN accounting.journal_entries AS entry
				ON entry.org_id = line.org_id
			   AND entry.id = line.journal_entry_id
			 WHERE line.org_id = $1
			   AND line.account_id = $2
			   AND entry.entry_date BETWEEN $3 AND $4
		), filtered AS (
			SELECT *
			  FROM period_lines
			 WHERE (
					$5::text = ''
					OR entry_number::text = $5
					OR reference ILIKE '%' || $5 || '%'
					OR origin ILIKE '%' || $5 || '%'
					OR description ILIKE '%' || $5 || '%'
					OR memo ILIKE '%' || $5 || '%'
			 )
			   AND ($6::text = '' OR lower(origin) = lower($6))
		)
		SELECT
			entry_id,
			line_id,
			entry_number,
			line_no,
			entry_date,
			reference,
			origin,
			description,
			memo,
			debit,
			credit,
			period_balance
		  FROM filtered
		 WHERE $7::date IS NULL
			OR (entry_date, entry_number, line_no, line_id) > (
				$7::date,
				$8::bigint,
				$9::integer,
				$10::uuid
			)
		 ORDER BY entry_date, entry_number, line_no, line_id
		 LIMIT $11
	`,
		repository.orgID,
		filter.AccountID,
		filter.From,
		filter.To,
		filter.Query,
		filter.Origin,
		cursorDate,
		cursorNumber,
		cursorLineNo,
		cursorLineID,
		filter.Limit+1,
	)
	if err != nil {
		return accounting.GeneralLedgerPage{}, mapError(err)
	}
	defer rows.Close()

	for rows.Next() {
		movement, scanErr := scanGeneralLedgerMovement(rows)
		if scanErr != nil {
			return accounting.GeneralLedgerPage{}, scanErr
		}
		// The window contains only the requested period, so carry the exact
		// pre-period opening balance into every visible row after scanning.
		// This deliberately happens before query-filtered rows are returned.
		movement.Balance = result.OpeningBalance.Add(movement.Balance)
		result.Items = append(result.Items, movement)
	}
	if err := rows.Err(); err != nil {
		return accounting.GeneralLedgerPage{}, mapError(err)
	}
	if len(result.Items) > filter.Limit {
		last := result.Items[filter.Limit-1]
		result.Items = result.Items[:filter.Limit]
		result.NextCursor = &accounting.GeneralLedgerCursor{
			Date:        last.Date,
			EntryNumber: last.EntryNumber,
			LineNumber:  last.LineNumber,
			LineID:      last.LineID,
		}
	}
	return result, nil
}

func scanGeneralLedgerMovement(row scanner) (accounting.GeneralLedgerMovement, error) {
	var (
		movement    accounting.GeneralLedgerMovement
		debitText   string
		creditText  string
		balanceText string
	)
	if err := row.Scan(
		&movement.EntryID,
		&movement.LineID,
		&movement.EntryNumber,
		&movement.LineNumber,
		&movement.Date,
		&movement.Reference,
		&movement.Origin,
		&movement.Description,
		&movement.Memo,
		&debitText,
		&creditText,
		&balanceText,
	); err != nil {
		return accounting.GeneralLedgerMovement{}, mapError(err)
	}
	var err error
	if movement.Debit, err = accounting.ParseAmount(debitText); err != nil {
		return accounting.GeneralLedgerMovement{}, fmt.Errorf("parse general ledger debit: %w", err)
	}
	if movement.Credit, err = accounting.ParseAmount(creditText); err != nil {
		return accounting.GeneralLedgerMovement{}, fmt.Errorf("parse general ledger credit: %w", err)
	}
	if movement.Balance, err = accounting.ParseAmount(balanceText); err != nil {
		return accounting.GeneralLedgerMovement{}, fmt.Errorf("parse general ledger balance: %w", err)
	}
	return movement, nil
}
