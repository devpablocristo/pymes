package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

func (repository *Repository) ListCurrencyRevaluationPositions(
	ctx context.Context,
	asOf time.Time,
	functionalCurrency accounting.Currency,
) ([]accounting.CurrencyRevaluationPosition, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			account.id,
			account.code,
			account.name,
			account.normal_balance,
			line.currency_code,
			sum(
				CASE
					WHEN line.debit_amount > 0 THEN line.currency_amount
					ELSE -line.currency_amount
				END
			)::text,
			sum(line.debit_amount - line.credit_amount)::text
		  FROM accounting.journal_lines AS line
		  JOIN accounting.journal_entries AS entry
		    ON entry.org_id = line.org_id
		   AND entry.id = line.journal_entry_id
		  JOIN accounting.accounts AS account
		    ON account.org_id = line.org_id
		   AND account.id = line.account_id
		 WHERE line.org_id = $1
		   AND entry.entry_date <= $2
		   AND line.currency_code <> $3
		 GROUP BY
			account.id,
			account.code,
			account.name,
			account.normal_balance,
			line.currency_code
		HAVING
			sum(
				CASE
					WHEN line.debit_amount > 0 THEN line.currency_amount
					ELSE -line.currency_amount
				END
			) <> 0
			OR sum(line.debit_amount - line.credit_amount) <> 0
		 ORDER BY account.code, line.currency_code, account.id
	`, repository.orgID, asOf, functionalCurrency.Code())
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.CurrencyRevaluationPosition, 0)
	for rows.Next() {
		var (
			position     accounting.CurrencyRevaluationPosition
			currencyCode string
			currencyText string
			carryingText string
		)
		if err := rows.Scan(
			&position.AccountID,
			&position.AccountCode,
			&position.AccountName,
			&position.NormalBalance,
			&currencyCode,
			&currencyText,
			&carryingText,
		); err != nil {
			return nil, mapError(err)
		}
		position.Currency, err = accounting.NewCurrency(currencyCode)
		if err != nil {
			return nil, err
		}
		position.CurrencyAmount, err = accounting.ParseAmount(currencyText)
		if err != nil {
			return nil, err
		}
		position.CarryingAmount, err = accounting.ParseAmount(carryingText)
		if err != nil {
			return nil, err
		}
		result = append(result, position)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func (repository *Repository) CreateCurrencyRevaluationWorkpaper(
	ctx context.Context,
	workpaper accounting.CurrencyRevaluationWorkpaper,
) (accounting.CurrencyRevaluationWorkpaper, error) {
	var periodID uuid.UUID
	if err := repository.tx.QueryRow(ctx, `
		SELECT id
		  FROM accounting.periods
		 WHERE org_id = $1
		   AND $2::date BETWEEN start_date AND end_date
		 FOR UPDATE
	`, repository.orgID, workpaper.ClosingDate).Scan(&periodID); err != nil {
		return accounting.CurrencyRevaluationWorkpaper{}, mapError(err)
	}
	var existingID uuid.UUID
	err := repository.tx.QueryRow(ctx, `
		SELECT id
		  FROM accounting.currency_revaluation_runs
		 WHERE org_id = $1
		   AND period_id = $2
		   AND revaluation_date = $3
		   AND source_checksum = $4
	`, repository.orgID, periodID, workpaper.ClosingDate, workpaper.SourceChecksum).Scan(&existingID)
	if err == nil {
		return repository.GetCurrencyRevaluationWorkpaper(ctx, existingID)
	}
	if mapped := mapError(err); mapped != accounting.ErrNotFound {
		return accounting.CurrencyRevaluationWorkpaper{}, mapped
	}
	if err := repository.recordClosingExchangeRates(ctx, workpaper.Rates); err != nil {
		return accounting.CurrencyRevaluationWorkpaper{}, err
	}
	draft, err := repository.CreateDraft(ctx, workpaper.Draft)
	if err != nil {
		return accounting.CurrencyRevaluationWorkpaper{}, err
	}
	workpaper.Draft = draft
	if _, err := repository.tx.Exec(ctx, `
		INSERT INTO accounting.currency_revaluation_runs (
			org_id,
			id,
			period_id,
			revaluation_date,
			status,
			generated_draft_id,
			source_checksum,
			created_by
		)
		VALUES ($1, $2, $3, $4, 'draft', $5, $6, $7)
	`,
		repository.orgID,
		workpaper.ID,
		periodID,
		workpaper.ClosingDate,
		draft.ID,
		workpaper.SourceChecksum,
		workpaper.CreatedBy,
	); err != nil {
		return accounting.CurrencyRevaluationWorkpaper{}, mapError(err)
	}
	for index, line := range workpaper.Lines {
		if _, err := repository.tx.Exec(ctx, `
			INSERT INTO accounting.currency_revaluation_lines (
				org_id,
				revaluation_run_id,
				line_no,
				account_id,
				currency_code,
				currency_amount,
				carrying_amount,
				closing_rate,
				revalued_amount,
				exchange_difference_amount
			)
			VALUES (
				$1, $2, $3, $4, $5, $6::numeric, $7::numeric,
				$8::numeric, $9::numeric, $10::numeric
			)
		`,
			repository.orgID,
			workpaper.ID,
			index+1,
			line.AccountID,
			line.Currency.Code(),
			line.CurrencyAmount.String(),
			line.CarryingAmount.String(),
			line.ClosingRate.String(),
			line.RevaluedAmount.String(),
			line.ExchangeDifference.String(),
		); err != nil {
			return accounting.CurrencyRevaluationWorkpaper{}, mapError(err)
		}
	}
	return repository.GetCurrencyRevaluationWorkpaper(ctx, workpaper.ID)
}

func (repository *Repository) GetCurrencyRevaluationWorkpaper(
	ctx context.Context,
	id uuid.UUID,
) (accounting.CurrencyRevaluationWorkpaper, error) {
	var (
		workpaper accounting.CurrencyRevaluationWorkpaper
		draftID   uuid.UUID
	)
	if err := repository.tx.QueryRow(ctx, `
		SELECT
			id,
			revaluation_date,
			generated_draft_id,
			source_checksum,
			created_by,
			created_at
		  FROM accounting.currency_revaluation_runs
		 WHERE org_id = $1
		   AND id = $2
	`, repository.orgID, id).Scan(
		&workpaper.ID,
		&workpaper.ClosingDate,
		&draftID,
		&workpaper.SourceChecksum,
		&workpaper.CreatedBy,
		&workpaper.CreatedAt,
	); err != nil {
		return accounting.CurrencyRevaluationWorkpaper{}, mapError(err)
	}
	var err error
	workpaper.Draft, err = repository.GetDraft(ctx, draftID, false)
	if err != nil {
		return accounting.CurrencyRevaluationWorkpaper{}, err
	}
	workpaper.FunctionalCurrency = workpaper.Draft.FunctionalCurrency
	rows, err := repository.tx.Query(ctx, `
		SELECT
			line.account_id,
			account.code,
			account.name,
			account.normal_balance,
			line.currency_code,
			line.currency_amount::text,
			line.carrying_amount::text,
			line.closing_rate::text,
			line.revalued_amount::text,
			line.exchange_difference_amount::text
		  FROM accounting.currency_revaluation_lines AS line
		  JOIN accounting.accounts AS account
		    ON account.org_id = line.org_id
		   AND account.id = line.account_id
		 WHERE line.org_id = $1
		   AND line.revaluation_run_id = $2
		 ORDER BY line.line_no
	`, repository.orgID, id)
	if err != nil {
		return accounting.CurrencyRevaluationWorkpaper{}, mapError(err)
	}
	defer rows.Close()
	desiredRates := make(map[string]accounting.Decimal)
	for rows.Next() {
		var (
			line                                    accounting.CurrencyRevaluationLine
			currencyCode                            string
			currencyAmount, carryingAmount          string
			closingRate, revaluedAmount, difference string
		)
		if err := rows.Scan(
			&line.AccountID,
			&line.AccountCode,
			&line.AccountName,
			&line.NormalBalance,
			&currencyCode,
			&currencyAmount,
			&carryingAmount,
			&closingRate,
			&revaluedAmount,
			&difference,
		); err != nil {
			return accounting.CurrencyRevaluationWorkpaper{}, mapError(err)
		}
		line.Currency, err = accounting.NewCurrency(currencyCode)
		if err != nil {
			return accounting.CurrencyRevaluationWorkpaper{}, err
		}
		line.CurrencyAmount, err = accounting.ParseAmount(currencyAmount)
		if err != nil {
			return accounting.CurrencyRevaluationWorkpaper{}, err
		}
		line.CarryingAmount, err = accounting.ParseAmount(carryingAmount)
		if err != nil {
			return accounting.CurrencyRevaluationWorkpaper{}, err
		}
		line.ClosingRate, err = accounting.ParseExchangeRate(closingRate)
		if err != nil {
			return accounting.CurrencyRevaluationWorkpaper{}, err
		}
		line.RevaluedAmount, err = accounting.ParseAmount(revaluedAmount)
		if err != nil {
			return accounting.CurrencyRevaluationWorkpaper{}, err
		}
		line.ExchangeDifference, err = accounting.ParseAmount(difference)
		if err != nil {
			return accounting.CurrencyRevaluationWorkpaper{}, err
		}
		if line.ExchangeDifference.Sign() > 0 {
			workpaper.TotalGain = workpaper.TotalGain.Add(line.ExchangeDifference)
		} else {
			workpaper.TotalLoss = workpaper.TotalLoss.Add(line.ExchangeDifference.Abs())
		}
		desiredRates[line.Currency.Code()] = line.ClosingRate
		workpaper.Lines = append(workpaper.Lines, line)
	}
	if err := rows.Err(); err != nil {
		return accounting.CurrencyRevaluationWorkpaper{}, mapError(err)
	}
	rows.Close()
	workpaper.NetResult = workpaper.TotalGain.Sub(workpaper.TotalLoss)
	workpaper.Rates, err = repository.listStoredClosingRates(
		ctx,
		workpaper.ClosingDate,
		workpaper.FunctionalCurrency,
		desiredRates,
	)
	if err != nil {
		return accounting.CurrencyRevaluationWorkpaper{}, err
	}
	return workpaper, nil
}

func (repository *Repository) recordClosingExchangeRates(
	ctx context.Context,
	rates []accounting.ClosingExchangeRate,
) error {
	for _, rate := range rates {
		if _, err := repository.tx.Exec(ctx, `
			INSERT INTO accounting.exchange_rates (
				org_id,
				rate_date,
				currency_code,
				functional_currency,
				rate,
				source,
				source_reference,
				source_checksum
			)
			VALUES (
				$1, $2, $3, $4, $5::numeric, $6, NULLIF($7, ''), $8
			)
			ON CONFLICT (
				org_id,
				rate_date,
				currency_code,
				functional_currency,
				source
			) DO NOTHING
		`,
			repository.orgID,
			rate.Date,
			rate.Currency.Code(),
			rate.FunctionalCurrency.Code(),
			rate.Rate.String(),
			rate.Source,
			rate.SourceReference,
			rate.SourceChecksum,
		); err != nil {
			return mapError(err)
		}
		var storedRate, storedReference, storedChecksum string
		if err := repository.tx.QueryRow(ctx, `
			SELECT
				rate::text,
				coalesce(source_reference, ''),
				coalesce(source_checksum, '')
			  FROM accounting.exchange_rates
			 WHERE org_id = $1
			   AND rate_date = $2
			   AND currency_code = $3
			   AND functional_currency = $4
			   AND source = $5
		`,
			repository.orgID,
			rate.Date,
			rate.Currency.Code(),
			rate.FunctionalCurrency.Code(),
			rate.Source,
		).Scan(&storedRate, &storedReference, &storedChecksum); err != nil {
			return mapError(err)
		}
		parsed, err := accounting.ParseExchangeRate(storedRate)
		if err != nil {
			return err
		}
		if !parsed.Equal(rate.Rate) ||
			storedReference != rate.SourceReference ||
			storedChecksum != rate.SourceChecksum {
			return fmt.Errorf(
				"%w: exchange rate source identity already contains different data",
				accounting.ErrConflict,
			)
		}
	}
	return nil
}

func (repository *Repository) listStoredClosingRates(
	ctx context.Context,
	closingDate time.Time,
	functionalCurrency accounting.Currency,
	desired map[string]accounting.Decimal,
) ([]accounting.ClosingExchangeRate, error) {
	if len(desired) == 0 {
		return nil, nil
	}
	codes := make([]string, 0, len(desired))
	for code := range desired {
		codes = append(codes, code)
	}
	rows, err := repository.tx.Query(ctx, `
		SELECT
			rate_date,
			currency_code,
			functional_currency,
			rate::text,
			source,
			coalesce(source_reference, ''),
			coalesce(source_checksum, '')
		  FROM accounting.exchange_rates
		 WHERE org_id = $1
		   AND currency_code = ANY($2::text[])
		   AND functional_currency = $3
		   AND rate_date <= $4
		 ORDER BY currency_code, rate_date DESC, source
	`, repository.orgID, codes, functionalCurrency.Code(), closingDate)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	found := make(map[string]struct{}, len(desired))
	result := make([]accounting.ClosingExchangeRate, 0, len(desired))
	for rows.Next() {
		var (
			rate                         accounting.ClosingExchangeRate
			currencyCode, functionalCode string
			rateText                     string
		)
		if err := rows.Scan(
			&rate.Date,
			&currencyCode,
			&functionalCode,
			&rateText,
			&rate.Source,
			&rate.SourceReference,
			&rate.SourceChecksum,
		); err != nil {
			return nil, mapError(err)
		}
		if _, ok := found[currencyCode]; ok {
			continue
		}
		rate.Rate, err = accounting.ParseExchangeRate(rateText)
		if err != nil {
			return nil, err
		}
		if !rate.Rate.Equal(desired[currencyCode]) {
			continue
		}
		rate.Currency, err = accounting.NewCurrency(currencyCode)
		if err != nil {
			return nil, err
		}
		rate.FunctionalCurrency, err = accounting.NewCurrency(functionalCode)
		if err != nil {
			return nil, err
		}
		found[currencyCode] = struct{}{}
		result = append(result, rate)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	if len(found) != len(desired) {
		return nil, fmt.Errorf("%w: persisted closing exchange rate", accounting.ErrNotFound)
	}
	return result, nil
}
