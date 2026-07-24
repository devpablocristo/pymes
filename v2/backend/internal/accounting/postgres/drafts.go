package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

func (repository *Repository) CreateDraft(
	ctx context.Context,
	draft accounting.Draft,
) (accounting.Draft, error) {
	if _, err := repository.tx.Exec(ctx, `
		INSERT INTO accounting.drafts (
			org_id,
			id,
			idempotency_key,
			entry_date,
			entry_kind,
			reference,
			currency_code,
			exchange_rate,
			exchange_rate_date,
			exchange_rate_source,
			description,
			source_type,
			source_id,
			created_by,
			updated_by
		)
		VALUES (
			$1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8::numeric,
			$9, NULLIF($10, ''), $11, NULLIF($12, ''),
			NULLIF($13, ''), $14, $15
		)
	`,
		repository.orgID,
		draft.ID,
		draft.IdempotencyKey,
		draft.Date,
		draft.Kind,
		draft.Reference,
		draft.Currency.Code(),
		draft.ExchangeRate.String(),
		nullableDraftRateDate(draft.ExchangeRateDate),
		strings.TrimSpace(draft.ExchangeRateSource),
		draft.Description,
		draft.SourceType,
		draft.SourceID,
		draft.CreatedBy,
		draft.UpdatedBy,
	); err != nil {
		return accounting.Draft{}, mapError(err)
	}
	if err := repository.insertDraftLines(ctx, draft.ID, draft.Lines); err != nil {
		return accounting.Draft{}, err
	}
	if err := repository.validateDraftConstraints(ctx); err != nil {
		return accounting.Draft{}, err
	}
	return repository.GetDraft(ctx, draft.ID, false)
}

func (repository *Repository) GetDraft(
	ctx context.Context,
	id uuid.UUID,
	forUpdate bool,
) (accounting.Draft, error) {
	query := `
		SELECT
			id,
			version,
			idempotency_key,
			entry_date,
			entry_kind,
			coalesce(reference, ''),
			currency_code,
			exchange_rate::text,
			exchange_rate_date,
			coalesce(exchange_rate_source, ''),
			description,
			coalesce(source_type, ''),
			coalesce(source_id, ''),
			created_by,
			updated_by,
			created_at,
			updated_at
		  FROM accounting.drafts
		 WHERE org_id = $1
		   AND id = $2
		   AND status <> 'discarded'
	`
	if forUpdate {
		query += " AND status = 'active' FOR UPDATE"
	}
	var (
		draft        accounting.Draft
		currencyCode string
		rateText     string
		rateDate     *time.Time
	)
	if err := repository.tx.QueryRow(ctx, query, repository.orgID, id).Scan(
		&draft.ID,
		&draft.Version,
		&draft.IdempotencyKey,
		&draft.Date,
		&draft.Kind,
		&draft.Reference,
		&currencyCode,
		&rateText,
		&rateDate,
		&draft.ExchangeRateSource,
		&draft.Description,
		&draft.SourceType,
		&draft.SourceID,
		&draft.CreatedBy,
		&draft.UpdatedBy,
		&draft.CreatedAt,
		&draft.UpdatedAt,
	); err != nil {
		return accounting.Draft{}, mapError(err)
	}
	var err error
	draft.Currency, err = accounting.NewCurrency(currencyCode)
	if err != nil {
		return accounting.Draft{}, err
	}
	draft.ExchangeRate, err = accounting.ParseExchangeRate(rateText)
	if err != nil {
		return accounting.Draft{}, err
	}
	if rateDate != nil {
		draft.ExchangeRateDate = *rateDate
	}
	functionalCurrency, err := repository.functionalCurrency(ctx)
	if err != nil {
		return accounting.Draft{}, err
	}
	draft.FunctionalCurrency = functionalCurrency
	lines, err := repository.listDraftLines(ctx, id)
	if err != nil {
		return accounting.Draft{}, err
	}
	draft.Lines = lines
	draft.IsAdjustment = draft.Kind == accounting.EntryAdjustment ||
		draft.Kind == accounting.EntryInflation ||
		draft.Kind == accounting.EntryRevaluation ||
		draft.Kind == accounting.EntryClosing
	return draft, nil
}

func (repository *Repository) FindDraftByIdempotency(
	ctx context.Context,
	idempotencyKey string,
) (accounting.Draft, error) {
	var id uuid.UUID
	var status string
	if err := repository.tx.QueryRow(ctx, `
		SELECT id, status
		  FROM accounting.drafts
		 WHERE org_id = $1
		   AND idempotency_key = $2
	`, repository.orgID, idempotencyKey).Scan(&id, &status); err != nil {
		return accounting.Draft{}, mapError(err)
	}
	if status == "discarded" {
		return accounting.Draft{}, accounting.ErrIdempotencyConflict
	}
	return repository.GetDraft(ctx, id, false)
}

func (repository *Repository) UpdateDraft(
	ctx context.Context,
	draft accounting.Draft,
	expectedVersion int64,
) (accounting.Draft, error) {
	commandTag, err := repository.tx.Exec(ctx, `
		UPDATE accounting.drafts
		   SET entry_date = $3,
		       entry_kind = $4,
		       reference = NULLIF($5, ''),
		       currency_code = $6,
		       exchange_rate = $7::numeric,
		       exchange_rate_date = $8,
		       exchange_rate_source = NULLIF($9, ''),
		       description = $10,
		       source_type = NULLIF($11, ''),
		       source_id = NULLIF($12, ''),
		       updated_by = $13,
		       version = version + 1,
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
		   AND version = $14
		   AND status = 'active'
	`,
		repository.orgID,
		draft.ID,
		draft.Date,
		draft.Kind,
		draft.Reference,
		draft.Currency.Code(),
		draft.ExchangeRate.String(),
		nullableDraftRateDate(draft.ExchangeRateDate),
		strings.TrimSpace(draft.ExchangeRateSource),
		draft.Description,
		draft.SourceType,
		draft.SourceID,
		draft.UpdatedBy,
		expectedVersion,
	)
	if err != nil {
		return accounting.Draft{}, mapError(err)
	}
	if commandTag.RowsAffected() == 0 {
		return accounting.Draft{}, accounting.ErrVersionConflict
	}
	if _, err := repository.tx.Exec(ctx, `
		DELETE FROM accounting.draft_lines
		 WHERE org_id = $1
		   AND draft_id = $2
	`, repository.orgID, draft.ID); err != nil {
		return accounting.Draft{}, mapError(err)
	}
	if err := repository.insertDraftLines(ctx, draft.ID, draft.Lines); err != nil {
		return accounting.Draft{}, err
	}
	if err := repository.validateDraftConstraints(ctx); err != nil {
		return accounting.Draft{}, err
	}
	return repository.GetDraft(ctx, draft.ID, false)
}

func nullableDraftRateDate(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func (repository *Repository) DiscardDraft(
	ctx context.Context,
	id uuid.UUID,
	expectedVersion int64,
	actor string,
	reason string,
) error {
	commandTag, err := repository.tx.Exec(ctx, `
		UPDATE accounting.drafts
		   SET status = 'discarded',
		       updated_by = $4,
		       discarded_by = $4,
		       discard_reason = NULLIF($5, ''),
		       discarded_at = now(),
		       version = version + 1,
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
		   AND version = $3
		   AND status = 'active'
	`, repository.orgID, id, expectedVersion, actor, strings.TrimSpace(reason))
	if err != nil {
		return mapError(err)
	}
	if commandTag.RowsAffected() == 0 {
		return accounting.ErrVersionConflict
	}
	return repository.validateDraftConstraints(ctx)
}

func (repository *Repository) MarkDraftPosted(
	ctx context.Context,
	id uuid.UUID,
	expectedVersion int64,
	entryID uuid.UUID,
) error {
	commandTag, err := repository.tx.Exec(ctx, `
		UPDATE accounting.drafts
		   SET status = 'posted',
		       posted_entry_id = $4,
		       updated_by = $5,
		       version = version + 1,
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
		   AND version = $3
		   AND status = 'active'
	`, repository.orgID, id, expectedVersion, entryID, repository.actor)
	if err != nil {
		return mapError(err)
	}
	if commandTag.RowsAffected() == 0 {
		return accounting.ErrVersionConflict
	}
	return repository.validateDraftConstraints(ctx)
}

func (repository *Repository) insertDraftLines(
	ctx context.Context,
	draftID uuid.UUID,
	lines []accounting.JournalLine,
) error {
	for index, line := range lines {
		currencyAmount := line.TransactionDebit.Add(line.TransactionCredit)
		var rateDate any
		var rateSource any
		if !line.ExchangeRateDate.IsZero() {
			rateDate = line.ExchangeRateDate
			rateSource = strings.TrimSpace(line.ExchangeRateSource)
		}
		var partyType any
		var partyID any
		if line.PartyID != nil {
			partyType = "party"
			partyID = line.PartyID.String()
		}
		if _, err := repository.tx.Exec(ctx, `
			INSERT INTO accounting.draft_lines (
				org_id,
				draft_id,
				line_no,
				account_id,
				description,
				debit_amount,
				credit_amount,
				currency_code,
				currency_amount,
				exchange_rate,
				exchange_rate_date,
				exchange_rate_source,
				party_type,
				party_id
			)
			VALUES (
				$1, $2, $3, $4, $5, $6::numeric, $7::numeric, $8,
				$9::numeric, $10::numeric, $11, $12, $13, $14
			)
		`,
			repository.orgID,
			draftID,
			index+1,
			line.AccountID,
			line.Memo,
			line.Debit.String(),
			line.Credit.String(),
			line.Currency.Code(),
			currencyAmount.String(),
			line.ExchangeRate.String(),
			rateDate,
			rateSource,
			partyType,
			partyID,
		); err != nil {
			return mapError(err)
		}
	}
	return nil
}

func (repository *Repository) listDraftLines(
	ctx context.Context,
	draftID uuid.UUID,
) ([]accounting.JournalLine, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			line.id,
			line.line_no,
			line.account_id,
			account.code,
			account.name,
			line.description,
			line.debit_amount::text,
			line.credit_amount::text,
			line.currency_code,
			line.currency_amount::text,
			line.exchange_rate::text,
			line.exchange_rate_date,
			coalesce(line.exchange_rate_source, ''),
			line.party_id
		  FROM accounting.draft_lines AS line
		  JOIN accounting.accounts AS account
		    ON account.org_id = line.org_id
		   AND account.id = line.account_id
		 WHERE line.org_id = $1
		   AND line.draft_id = $2
		 ORDER BY line.line_no
	`, repository.orgID, draftID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	lines := make([]accounting.JournalLine, 0)
	for rows.Next() {
		var (
			line           accounting.JournalLine
			debitText      string
			creditText     string
			currencyCode   string
			currencyAmount string
			rateText       string
			rateDate       *time.Time
			partyID        *string
		)
		if err := rows.Scan(
			&line.ID,
			&line.LineNo,
			&line.AccountID,
			&line.AccountCode,
			&line.AccountName,
			&line.Memo,
			&debitText,
			&creditText,
			&currencyCode,
			&currencyAmount,
			&rateText,
			&rateDate,
			&line.ExchangeRateSource,
			&partyID,
		); err != nil {
			return nil, mapError(err)
		}
		if err := hydrateLineAmounts(&line, debitText, creditText, currencyCode, currencyAmount, rateText); err != nil {
			return nil, err
		}
		if rateDate != nil {
			line.ExchangeRateDate = *rateDate
		}
		if partyID != nil {
			parsed, parseErr := uuid.Parse(*partyID)
			if parseErr != nil {
				return nil, fmt.Errorf("accounting postgres: invalid party id: %w", parseErr)
			}
			line.PartyID = &parsed
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return lines, nil
}

func (repository *Repository) functionalCurrency(ctx context.Context) (accounting.Currency, error) {
	var code string
	if err := repository.tx.QueryRow(ctx, `
		SELECT functional_currency
		  FROM accounting.organization_settings
		 WHERE org_id = $1
	`, repository.orgID).Scan(&code); err != nil {
		return accounting.Currency{}, mapError(err)
	}
	currency, err := accounting.NewCurrency(code)
	if err != nil {
		return accounting.Currency{}, err
	}
	return currency, nil
}
