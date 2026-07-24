package postgres

import (
	"context"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

const trialBalanceCTE = `
	WITH RECURSIVE account_paths AS (
		SELECT
			account.org_id,
			account.id,
			account.parent_id,
			account.code,
			account.name,
			account.account_class,
			account.normal_balance,
			account.posting_allowed,
			account.archived_at,
			account.trashed_at,
			ARRAY[account.name]::text[] AS path,
			ARRAY[account.id]::uuid[] AS visited
		  FROM accounting.accounts AS account
		 WHERE account.org_id = $1
		   AND account.parent_id IS NULL

		UNION ALL

		SELECT
			child.org_id,
			child.id,
			child.parent_id,
			child.code,
			child.name,
			child.account_class,
			child.normal_balance,
			child.posting_allowed,
			child.archived_at,
			child.trashed_at,
			parent.path || child.name,
			parent.visited || child.id
		  FROM accounting.accounts AS child
		  JOIN account_paths AS parent
			ON parent.org_id = child.org_id
		   AND parent.id = child.parent_id
		 WHERE child.org_id = $1
		   AND NOT child.id = ANY(parent.visited)
	),
	ledger_amounts AS (
		SELECT
			line.account_id,
			coalesce(
				sum(line.debit_amount - line.credit_amount)
					FILTER (WHERE entry.entry_date < $2),
				0
			) AS opening_balance,
			coalesce(
				sum(line.debit_amount)
					FILTER (WHERE entry.entry_date BETWEEN $2 AND $3),
				0
			) AS period_debit,
			coalesce(
				sum(line.credit_amount)
					FILTER (WHERE entry.entry_date BETWEEN $2 AND $3),
				0
			) AS period_credit
		  FROM accounting.journal_entries AS entry
		  JOIN accounting.journal_lines AS line
			ON line.org_id = entry.org_id
		   AND line.journal_entry_id = entry.id
		 WHERE entry.org_id = $1
		   AND entry.entry_date <= $3
		 GROUP BY line.account_id
	),
	balance_rows AS (
		SELECT
			account.id AS account_id,
			account.code,
			account.name,
			account.account_class,
			account.normal_balance,
			account.path,
			CASE
				WHEN account.trashed_at IS NOT NULL THEN 'trashed'
				WHEN account.archived_at IS NOT NULL THEN 'archived'
				ELSE 'active'
			END AS lifecycle_state,
			coalesce(amount.opening_balance, 0) AS opening_balance,
			coalesce(amount.period_debit, 0) AS period_debit,
			coalesce(amount.period_credit, 0) AS period_credit
		  FROM account_paths AS account
		  LEFT JOIN ledger_amounts AS amount
			ON amount.account_id = account.id
		 WHERE account.posting_allowed
		   AND account.trashed_at IS NULL
	),
	filtered AS (
		SELECT
			account_id,
			code,
			name,
			account_class,
			normal_balance,
			path,
			lifecycle_state,
			opening_balance,
			period_debit,
			period_credit,
			opening_balance + period_debit - period_credit AS closing_balance
		  FROM balance_rows
		 WHERE ($4::text = '' OR account_class = $4)
		   AND (
				$5::text = ''
				OR code ILIKE '%' || $5 || '%'
				OR name ILIKE '%' || $5 || '%'
				OR array_to_string(path, ' > ') ILIKE '%' || $5 || '%'
		   )
		   AND (
				opening_balance <> 0
				OR period_debit <> 0
				OR period_credit <> 0
				OR ($6::boolean AND lifecycle_state = 'active')
		   )
	)
`

func (repository *Repository) ListTrialBalance(
	ctx context.Context,
	filter accounting.TrialBalanceFilter,
) (accounting.TrialBalancePage, error) {
	result := accounting.TrialBalancePage{
		From:  filter.From,
		To:    filter.To,
		Items: make([]accounting.TrialBalanceAccountRow, 0, filter.Limit),
	}

	var (
		total             int64
		openingDebitText  string
		openingCreditText string
		debitText         string
		creditText        string
		closingDebitText  string
		closingCreditText string
	)
	if err := repository.tx.QueryRow(
		ctx,
		trialBalanceCTE+`
			SELECT
				count(*),
				coalesce(sum(greatest(opening_balance, 0)), 0)::text,
				coalesce(sum(greatest(-opening_balance, 0)), 0)::text,
				coalesce(sum(period_debit), 0)::text,
				coalesce(sum(period_credit), 0)::text,
				coalesce(sum(greatest(closing_balance, 0)), 0)::text,
				coalesce(sum(greatest(-closing_balance, 0)), 0)::text
			  FROM filtered
		`,
		repository.orgID,
		filter.From,
		filter.To,
		string(filter.AccountClass),
		filter.Query,
		filter.IncludeZero,
	).Scan(
		&total,
		&openingDebitText,
		&openingCreditText,
		&debitText,
		&creditText,
		&closingDebitText,
		&closingCreditText,
	); err != nil {
		return accounting.TrialBalancePage{}, mapError(err)
	}

	var err error
	if result.Totals.OpeningDebit, err = accounting.ParseAmount(openingDebitText); err != nil {
		return accounting.TrialBalancePage{}, err
	}
	if result.Totals.OpeningCredit, err = accounting.ParseAmount(openingCreditText); err != nil {
		return accounting.TrialBalancePage{}, err
	}
	if result.Totals.Debit, err = accounting.ParseAmount(debitText); err != nil {
		return accounting.TrialBalancePage{}, err
	}
	if result.Totals.Credit, err = accounting.ParseAmount(creditText); err != nil {
		return accounting.TrialBalancePage{}, err
	}
	if result.Totals.ClosingDebit, err = accounting.ParseAmount(closingDebitText); err != nil {
		return accounting.TrialBalancePage{}, err
	}
	if result.Totals.ClosingCredit, err = accounting.ParseAmount(closingCreditText); err != nil {
		return accounting.TrialBalancePage{}, err
	}
	result.Totals.OpeningDifference = result.Totals.OpeningDebit.Sub(
		result.Totals.OpeningCredit,
	)
	result.Totals.MovementDifference = result.Totals.Debit.Sub(
		result.Totals.Credit,
	)
	result.Totals.ClosingDifference = result.Totals.ClosingDebit.Sub(
		result.Totals.ClosingCredit,
	)
	result.Total = int(total)

	var (
		cursorCode      any
		cursorAccountID any
	)
	if filter.Cursor != nil {
		cursorCode = filter.Cursor.Code
		cursorAccountID = filter.Cursor.AccountID
	}
	rows, err := repository.tx.Query(
		ctx,
		trialBalanceCTE+`
			SELECT
				account_id,
				code,
				name,
				account_class,
				normal_balance,
				path,
				lifecycle_state,
				opening_balance::text,
				period_debit::text,
				period_credit::text,
				closing_balance::text
			  FROM filtered
			 WHERE $7::text IS NULL
				OR (
					accounting.account_code_sort_key(code),
					account_id
				) > (
					accounting.account_code_sort_key($7::text),
					$8::uuid
				)
			 ORDER BY accounting.account_code_sort_key(code), account_id
			 LIMIT $9
		`,
		repository.orgID,
		filter.From,
		filter.To,
		string(filter.AccountClass),
		filter.Query,
		filter.IncludeZero,
		cursorCode,
		cursorAccountID,
		filter.Limit+1,
	)
	if err != nil {
		return accounting.TrialBalancePage{}, mapError(err)
	}
	defer rows.Close()

	for rows.Next() {
		row, scanErr := scanTrialBalanceAccountRow(rows)
		if scanErr != nil {
			return accounting.TrialBalancePage{}, scanErr
		}
		result.Items = append(result.Items, row)
	}
	if err := rows.Err(); err != nil {
		return accounting.TrialBalancePage{}, mapError(err)
	}
	if len(result.Items) > filter.Limit {
		last := result.Items[filter.Limit-1]
		result.Items = result.Items[:filter.Limit]
		result.NextCursor = &accounting.TrialBalanceCursor{
			Code:      last.Code,
			AccountID: last.AccountID,
		}
	}
	return result, nil
}

func scanTrialBalanceAccountRow(row scanner) (accounting.TrialBalanceAccountRow, error) {
	var (
		result      accounting.TrialBalanceAccountRow
		openingText string
		debitText   string
		creditText  string
		closingText string
	)
	if err := row.Scan(
		&result.AccountID,
		&result.Code,
		&result.Name,
		&result.Class,
		&result.NormalBalance,
		&result.Path,
		&result.LifecycleState,
		&openingText,
		&debitText,
		&creditText,
		&closingText,
	); err != nil {
		return accounting.TrialBalanceAccountRow{}, mapError(err)
	}
	var err error
	if result.OpeningBalance, err = accounting.ParseAmount(openingText); err != nil {
		return accounting.TrialBalanceAccountRow{}, err
	}
	if result.Debit, err = accounting.ParseAmount(debitText); err != nil {
		return accounting.TrialBalanceAccountRow{}, err
	}
	if result.Credit, err = accounting.ParseAmount(creditText); err != nil {
		return accounting.TrialBalanceAccountRow{}, err
	}
	if result.ClosingBalance, err = accounting.ParseAmount(closingText); err != nil {
		return accounting.TrialBalanceAccountRow{}, err
	}
	return result, nil
}
