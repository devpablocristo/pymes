package postgres

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

const periodColumns = `
	id,
	code,
	start_date,
	end_date,
	status,
	version,
	coalesce(status_changed_by, ''),
	coalesce(transition_reason, '')
`

func (repository *Repository) ListPeriods(
	ctx context.Context,
) ([]accounting.Period, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT `+periodColumns+`
		  FROM accounting.periods
		 WHERE org_id = $1
		 ORDER BY start_date DESC, id
	`, repository.orgID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.Period, 0)
	for rows.Next() {
		period, scanErr := scanPeriod(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, period)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func (repository *Repository) GetPeriod(
	ctx context.Context,
	id uuid.UUID,
	forUpdate bool,
) (accounting.Period, error) {
	query := `
		SELECT ` + periodColumns + `
		  FROM accounting.periods
		 WHERE org_id = $1
		   AND id = $2
	`
	if forUpdate {
		query += " FOR UPDATE"
	}
	period, err := scanPeriod(repository.tx.QueryRow(ctx, query, repository.orgID, id))
	if err != nil {
		return accounting.Period{}, err
	}
	return period, nil
}

func (repository *Repository) FindPeriodForDate(
	ctx context.Context,
	date time.Time,
	forUpdate bool,
) (accounting.Period, error) {
	query := `
		SELECT ` + periodColumns + `
		  FROM accounting.periods
		 WHERE org_id = $1
		   AND $2::date BETWEEN start_date AND end_date
	`
	if forUpdate {
		query += " FOR UPDATE"
	}
	period, err := scanPeriod(repository.tx.QueryRow(ctx, query, repository.orgID, date))
	if err != nil {
		return accounting.Period{}, err
	}
	return period, nil
}

func (repository *Repository) CreatePeriod(
	ctx context.Context,
	period accounting.Period,
) (accounting.Period, error) {
	created, err := scanPeriod(repository.tx.QueryRow(ctx, `
		INSERT INTO accounting.periods (
			org_id,
			id,
			code,
			start_date,
			end_date,
			status
		)
		VALUES ($1, $2, $3, $4, $5, 'open')
		RETURNING `+periodColumns,
		repository.orgID,
		period.ID,
		period.Name,
		period.StartDate,
		period.EndDate,
	))
	if err != nil {
		return accounting.Period{}, mapError(err)
	}
	return created, nil
}

func (repository *Repository) UpdatePeriod(
	ctx context.Context,
	period accounting.Period,
	expectedVersion int64,
) (accounting.Period, error) {
	updated, err := scanPeriod(repository.tx.QueryRow(ctx, `
		UPDATE accounting.periods
		   SET code = $3,
		       start_date = $4,
		       end_date = $5,
		       status = $6,
		       version = version + 1,
		       status_changed_by = NULLIF($7, ''),
		       transition_reason = NULLIF($8, ''),
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
		   AND version = $9
		RETURNING `+periodColumns,
		repository.orgID,
		period.ID,
		period.Name,
		period.StartDate,
		period.EndDate,
		period.Status,
		period.StatusChangedBy,
		period.TransitionReason,
		expectedVersion,
	))
	if err != nil {
		return accounting.Period{}, optimisticError(err)
	}
	return updated, nil
}

func (repository *Repository) CloseChecklist(
	ctx context.Context,
	periodID uuid.UUID,
) (accounting.CloseChecklist, error) {
	if _, err := repository.tx.Exec(ctx, `
		WITH target AS (
			SELECT start_date, end_date
			  FROM accounting.periods
			 WHERE org_id = $1
			   AND id = $2
		),
		results(check_key, item_count) AS (
			SELECT
				'unposted_documents',
				(
					SELECT count(*)
					  FROM fiscal.vouchers AS voucher, target
					 WHERE voucher.org_id = $1
					   AND voucher.status = 'authorized'
					   AND voucher.issue_date BETWEEN target.start_date AND target.end_date
					   AND NOT EXISTS (
							SELECT 1
							  FROM fiscal.voucher_accounting_links AS link
							 WHERE link.org_id = voucher.org_id
							   AND link.voucher_id = voucher.id
					   )
				) + (
					SELECT count(*)
					  FROM fiscal.purchase_vouchers AS purchase, target
					 WHERE purchase.org_id = $1
					   AND purchase.issue_date BETWEEN target.start_date AND target.end_date
					   AND NOT EXISTS (
							SELECT 1
							  FROM fiscal.purchase_voucher_accounting_links AS link
							 WHERE link.org_id = purchase.org_id
							   AND link.purchase_voucher_id = purchase.id
					   )
				)
			UNION ALL
			SELECT
				'fiscal_pending',
				(
					SELECT count(*)
					  FROM fiscal.vouchers AS voucher, target
					 WHERE voucher.org_id = $1
					   AND voucher.status IN ('queued', 'processing', 'uncertain')
					   AND voucher.issue_date BETWEEN target.start_date AND target.end_date
				)
			UNION ALL
			SELECT
				'posting_errors',
				(
					SELECT count(*)
					  FROM fiscal.accounting_posting_intents AS intent
					  JOIN fiscal.vouchers AS voucher
					    ON voucher.org_id = intent.org_id
					   AND voucher.id = intent.voucher_id
					  CROSS JOIN target
					 WHERE intent.org_id = $1
					   AND intent.status = 'failed'
					   AND voucher.issue_date BETWEEN target.start_date AND target.end_date
				)
			UNION ALL
			SELECT
				'account_mappings',
				CASE
					WHEN NOT EXISTS (
						SELECT 1
						  FROM accounting.organization_settings
						 WHERE org_id = $1
						   AND chart_template_code IS NOT NULL
					)
					THEN 1
					ELSE (
						SELECT count(*)
						  FROM accounting.organization_settings AS setting
						  JOIN accounting.chart_template_mappings AS template_mapping
						    ON template_mapping.template_code = setting.chart_template_code
						   AND template_mapping.template_version = setting.chart_template_version
						  LEFT JOIN accounting.account_mappings AS mapping
						    ON mapping.org_id = setting.org_id
						   AND mapping.mapping_key = template_mapping.mapping_key
						  LEFT JOIN accounting.accounts AS account
						    ON account.org_id = mapping.org_id
						   AND account.id = mapping.account_id
						 WHERE setting.org_id = $1
						   AND (
								mapping.account_id IS NULL
								OR account.archived_at IS NOT NULL
								OR account.trashed_at IS NOT NULL
						   )
					)
				END
			UNION ALL
			SELECT
				'exchange_rates',
				(
					SELECT count(*)
					  FROM (
							SELECT DISTINCT line.currency_code
							  FROM accounting.journal_entries AS entry
							  JOIN accounting.journal_lines AS line
							    ON line.org_id = entry.org_id
							   AND line.journal_entry_id = entry.id
							  JOIN accounting.accounts AS account
							    ON account.org_id = line.org_id
							   AND account.id = line.account_id
							  CROSS JOIN target
							 WHERE entry.org_id = $1
							   AND entry.entry_date BETWEEN target.start_date AND target.end_date
							   AND line.currency_code <> entry.functional_currency
							   AND account.monetary_class = 'monetary'
							   AND NOT EXISTS (
									SELECT 1
									  FROM accounting.exchange_rates AS rate
									 WHERE rate.org_id = entry.org_id
									   AND rate.currency_code = line.currency_code
									   AND rate.functional_currency = entry.functional_currency
									   AND rate.rate_date <= target.end_date
							   )
					  ) AS missing_currency
				)
			UNION ALL
			SELECT
				'unreconciled_accounts',
				(
					SELECT count(*)
					  FROM accounting.financial_accounts AS financial_account, target
					 WHERE financial_account.org_id = $1
					   AND financial_account.archived_at IS NULL
					   AND EXISTS (
							SELECT 1
							  FROM accounting.statement_transactions AS statement
							 WHERE statement.org_id = financial_account.org_id
							   AND statement.financial_account_id = financial_account.id
							   AND statement.booked_at BETWEEN target.start_date AND target.end_date
					   )
					   AND NOT EXISTS (
							SELECT 1
							  FROM accounting.reconciliations AS reconciliation
							 WHERE reconciliation.org_id = financial_account.org_id
							   AND reconciliation.financial_account_id = financial_account.id
							   AND reconciliation.status = 'closed'
							   AND reconciliation.start_date <= target.start_date
							   AND reconciliation.end_date >= target.end_date
					   )
				)
		)
		INSERT INTO accounting.period_close_checks (
			org_id,
			period_id,
			check_key,
			status,
			details,
			checked_by,
			checked_at
		)
		SELECT
			$1,
			$2,
			check_key,
			CASE WHEN item_count = 0 THEN 'passed' ELSE 'blocked' END,
			jsonb_build_object('count', item_count),
			coalesce(nullif(current_setting('app.user_id', true), ''), 'system'),
			now()
		  FROM results
		ON CONFLICT (org_id, period_id, check_key) DO UPDATE
		   SET status = excluded.status,
		       details = excluded.details,
		       checked_by = excluded.checked_by,
		       checked_at = excluded.checked_at
	`, repository.orgID, periodID); err != nil {
		return accounting.CloseChecklist{}, mapError(err)
	}

	rows, err := repository.tx.Query(ctx, `
		WITH expected(check_key) AS (
			VALUES
				('unposted_documents'),
				('fiscal_pending'),
				('posting_errors'),
				('account_mappings'),
				('exchange_rates'),
				('unreconciled_accounts')
		)
		SELECT
			expected.check_key,
			coalesce(check_result.status, 'missing'),
			coalesce(check_result.details->>'count', '1')
		  FROM expected
		  LEFT JOIN accounting.period_close_checks AS check_result
		    ON check_result.org_id = $1
		   AND check_result.period_id = $2
		   AND check_result.check_key = expected.check_key
	`, repository.orgID, periodID)
	if err != nil {
		return accounting.CloseChecklist{}, mapError(err)
	}
	defer rows.Close()
	var checklist accounting.CloseChecklist
	for rows.Next() {
		var key, status, countText string
		if err := rows.Scan(&key, &status, &countText); err != nil {
			return accounting.CloseChecklist{}, mapError(err)
		}
		count, parseErr := strconv.Atoi(countText)
		if parseErr != nil || count < 1 {
			count = 1
		}
		if status == "passed" || status == "warning" {
			count = 0
		}
		switch key {
		case "unposted_documents":
			checklist.UnpostedDocuments = count
		case "fiscal_pending":
			checklist.PendingFiscalDocuments = count
		case "posting_errors":
			checklist.PostingErrors = count
		case "account_mappings":
			checklist.MissingMappings = count
		case "exchange_rates":
			checklist.MissingExchangeRates = count
		case "unreconciled_accounts":
			checklist.UnclosedReconciliations = count
		}
	}
	if err := rows.Err(); err != nil {
		return accounting.CloseChecklist{}, mapError(err)
	}
	return checklist, nil
}

func scanPeriod(row scanner) (accounting.Period, error) {
	var period accounting.Period
	if err := row.Scan(
		&period.ID,
		&period.Name,
		&period.StartDate,
		&period.EndDate,
		&period.Status,
		&period.Version,
		&period.StatusChangedBy,
		&period.TransitionReason,
	); err != nil {
		return accounting.Period{}, mapError(err)
	}
	if period.Status == accounting.PeriodOpen && period.TransitionReason != "" {
		period.ReopenedReason = period.TransitionReason
		period.ReopenedBy = period.StatusChangedBy
	}
	return period, nil
}
