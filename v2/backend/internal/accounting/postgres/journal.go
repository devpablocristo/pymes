package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

func (repository *Repository) PostEntry(
	ctx context.Context,
	entry accounting.JournalEntry,
) (accounting.JournalEntry, error) {
	var periodID uuid.UUID
	if err := repository.tx.QueryRow(ctx, `
		SELECT id
		  FROM accounting.periods
		 WHERE org_id = $1
		   AND $2::date BETWEEN start_date AND end_date
		 FOR SHARE
	`, repository.orgID, entry.Date).Scan(&periodID); err != nil {
		return accounting.JournalEntry{}, mapError(err)
	}
	if err := repository.lockPostingDependencies(ctx, entry.Lines); err != nil {
		return accounting.JournalEntry{}, err
	}
	var (
		number   int64
		postedAt time.Time
	)
	var draftID any
	if entry.DraftID != nil {
		draftID = *entry.DraftID
	}
	var reversesID any
	var reversalReason any
	var reversedBy any
	if entry.ReversesEntryID != nil {
		reversesID = *entry.ReversesEntryID
		reversalReason = entry.ReversalReason
		reversedBy = entry.CreatedBy
	}
	if err := repository.tx.QueryRow(ctx, `
		INSERT INTO accounting.journal_entries (
			org_id,
			id,
			entry_number,
			entry_date,
			period_id,
			entry_kind,
			reference,
			description,
			functional_currency,
			source_type,
			source_id,
			source_event,
			posting_kind,
			idempotency_key,
			draft_id,
			reverses_entry_id,
			reversal_reason,
			reversed_by,
			created_by
		)
		VALUES (
			$1, $2, 1, $3, $4, $5, NULLIF($6, ''), $7, $8, $9,
			$10, $11, $12, $13, $14, $15, $16, $17, $18
		)
		RETURNING entry_number, posted_at
	`,
		repository.orgID,
		entry.ID,
		entry.Date,
		periodID,
		entry.Kind,
		entry.Reference,
		entry.Description,
		entry.FunctionalCurrency.Code(),
		entry.Source.Type,
		entry.Source.ID.String(),
		entry.Source.Event,
		entry.PostingKind,
		entry.Source.IdempotencyKey,
		draftID,
		reversesID,
		reversalReason,
		reversedBy,
		entry.CreatedBy,
	).Scan(&number, &postedAt); err != nil {
		return accounting.JournalEntry{}, mapError(err)
	}
	for index := range entry.Lines {
		line := &entry.Lines[index]
		if line.ID == uuid.Nil {
			line.ID = uuid.New()
		}
		line.LineNo = index + 1
		currencyAmount := line.TransactionDebit.Add(line.TransactionCredit)
		var rateDate any
		var rateSource any
		if !line.ExchangeRateDate.IsZero() {
			rateDate = line.ExchangeRateDate
			rateSource = line.ExchangeRateSource
		}
		var partyType any
		var partyID any
		if line.PartyID != nil {
			partyType = "party"
			partyID = line.PartyID.String()
		}
		if _, err := repository.tx.Exec(ctx, `
			INSERT INTO accounting.journal_lines (
				org_id,
				id,
				journal_entry_id,
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
				$1, $2, $3, $4, $5, $6, $7::numeric, $8::numeric,
				$9, $10::numeric, $11::numeric, $12, $13, $14, $15
			)
		`,
			repository.orgID,
			line.ID,
			entry.ID,
			line.LineNo,
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
			return accounting.JournalEntry{}, mapError(err)
		}
	}
	if err := repository.validateJournalConstraints(ctx); err != nil {
		return accounting.JournalEntry{}, err
	}
	entry.Number = number
	entry.CreatedAt = postedAt
	return entry, nil
}

func (repository *Repository) GetEntry(
	ctx context.Context,
	id uuid.UUID,
) (accounting.JournalEntry, error) {
	entry, err := repository.scanEntry(repository.tx.QueryRow(ctx, `
			SELECT
				entry.id,
				entry.entry_number,
				entry.entry_date,
				entry.entry_kind,
				entry.posting_kind,
				entry.functional_currency,
				coalesce(entry.reference, ''),
				coalesce(entry.source_type, ''),
				coalesce(entry.source_id, ''),
				entry.source_event,
				entry.idempotency_key,
				entry.description,
				entry.created_by,
				entry.posted_at,
				entry.reverses_entry_id,
				reversed_entry.entry_number,
				coalesce(entry.reversal_reason, ''),
				entry.draft_id,
				direct_reversal.id,
				direct_reversal.entry_number
			  FROM accounting.journal_entries AS entry
			  LEFT JOIN accounting.journal_entries AS reversed_entry
			    ON reversed_entry.org_id = entry.org_id
			   AND reversed_entry.id = entry.reverses_entry_id
			  LEFT JOIN accounting.journal_entries AS direct_reversal
			    ON direct_reversal.org_id = entry.org_id
			   AND direct_reversal.reverses_entry_id = entry.id
			 WHERE entry.org_id = $1
			   AND entry.id = $2
	`, repository.orgID, id))
	if err != nil {
		return accounting.JournalEntry{}, err
	}
	lines, err := repository.listJournalLines(ctx, entry.ID)
	if err != nil {
		return accounting.JournalEntry{}, err
	}
	entry.Lines = lines
	if len(lines) > 0 {
		representative := lines[0]
		for _, line := range lines {
			if line.Currency.Code() != entry.FunctionalCurrency.Code() {
				representative = line
				break
			}
		}
		entry.Currency = representative.Currency
		entry.ExchangeRate = representative.ExchangeRate
		entry.ExchangeRateDate = representative.ExchangeRateDate
		entry.ExchangeRateSource = representative.ExchangeRateSource
	}
	return entry, nil
}

func (repository *Repository) FindEntryBySource(
	ctx context.Context,
	source accounting.EntrySource,
) (accounting.JournalEntry, error) {
	var id uuid.UUID
	if err := repository.tx.QueryRow(ctx, `
		SELECT id
		  FROM accounting.journal_entries
		 WHERE org_id = $1
		   AND source_type = $2
		   AND source_id = $3
		 ORDER BY
			CASE WHEN posting_kind = 'primary' THEN 0 ELSE 1 END,
			entry_number
		 LIMIT 1
	`, repository.orgID, source.Type, source.ID.String()).Scan(&id); err != nil {
		return accounting.JournalEntry{}, mapError(err)
	}
	return repository.GetEntry(ctx, id)
}

func (repository *Repository) FindDirectReversal(
	ctx context.Context,
	id uuid.UUID,
) (accounting.JournalEntry, error) {
	var reversalID uuid.UUID
	if err := repository.tx.QueryRow(ctx, `
		SELECT id
		  FROM accounting.journal_entries
		 WHERE org_id = $1
		   AND reverses_entry_id = $2
	`, repository.orgID, id).Scan(&reversalID); err != nil {
		return accounting.JournalEntry{}, mapError(err)
	}
	return repository.GetEntry(ctx, reversalID)
}

func (repository *Repository) EntryHasOpenItemEffects(
	ctx context.Context,
	id uuid.UUID,
) (bool, error) {
	var linked bool
	if err := repository.tx.QueryRow(ctx, `
		SELECT
			EXISTS (
				SELECT 1
				  FROM accounting.open_items AS item
				 WHERE item.org_id = $1
				   AND item.origin_journal_entry_id = $2
			)
			OR EXISTS (
				SELECT 1
				  FROM accounting.open_item_applications AS application
				 WHERE application.org_id = $1
				   AND application.settlement_journal_entry_id = $2
			)
	`, repository.orgID, id).Scan(&linked); err != nil {
		return false, mapError(err)
	}
	return linked, nil
}

func (repository *Repository) TouchesClosedReconciliation(
	ctx context.Context,
	entryDate time.Time,
	accountIDs []uuid.UUID,
) (bool, error) {
	if len(accountIDs) == 0 {
		return false, nil
	}
	rows, err := repository.tx.Query(ctx, `
		SELECT reconciliation.status
		  FROM accounting.reconciliations AS reconciliation
		  JOIN accounting.financial_accounts AS financial_account
		    ON financial_account.org_id = reconciliation.org_id
		   AND financial_account.id = reconciliation.financial_account_id
		 WHERE reconciliation.org_id = $1
		   AND $2::date BETWEEN reconciliation.start_date
		                    AND reconciliation.end_date
		   AND financial_account.ledger_account_id = ANY($3::uuid[])
	`, repository.orgID, entryDate, accountIDs)
	if err != nil {
		return false, mapError(err)
	}
	defer rows.Close()
	closed := false
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return false, mapError(err)
		}
		closed = closed || status == "closed"
	}
	if err := rows.Err(); err != nil {
		return false, mapError(err)
	}
	return closed, nil
}

func (repository *Repository) ListJournal(
	ctx context.Context,
	filter accounting.JournalFilter,
) (accounting.PageResult[accounting.JournalEntry], error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := repository.tx.Query(ctx, `
		SELECT
			entry.id,
			entry.entry_number,
			entry.entry_date,
			entry.entry_kind,
			entry.posting_kind,
			entry.functional_currency,
			coalesce(entry.reference, ''),
			coalesce(entry.source_type, ''),
			coalesce(entry.source_id, ''),
			entry.source_event,
			entry.idempotency_key,
			entry.description,
			entry.created_by,
			entry.posted_at,
			entry.reverses_entry_id,
			reversed_entry.entry_number,
			coalesce(entry.reversal_reason, ''),
			entry.draft_id,
			direct_reversal.id,
			direct_reversal.entry_number,
			totals.debit_total::text,
			totals.credit_total::text,
			totals.currency_code,
			totals.exchange_rate::text,
			totals.exchange_rate_date,
			totals.exchange_rate_source
		  FROM accounting.journal_entries AS entry
		  LEFT JOIN accounting.journal_entries AS direct_reversal
		    ON direct_reversal.org_id = entry.org_id
		   AND direct_reversal.reverses_entry_id = entry.id
		  LEFT JOIN accounting.journal_entries AS reversed_entry
		    ON reversed_entry.org_id = entry.org_id
		   AND reversed_entry.id = entry.reverses_entry_id
		  JOIN LATERAL (
			SELECT
				sum(line.debit_amount) AS debit_total,
				sum(line.credit_amount) AS credit_total,
				(array_agg(
					line.currency_code
					ORDER BY
						(line.currency_code = entry.functional_currency),
						line.line_no
				))[1]
					AS currency_code,
				(array_agg(
					line.exchange_rate
					ORDER BY
						(line.currency_code = entry.functional_currency),
						line.line_no
				))[1]
					AS exchange_rate,
				(array_agg(
					line.exchange_rate_date
					ORDER BY
						(line.currency_code = entry.functional_currency),
						line.line_no
				))[1]
					AS exchange_rate_date,
				coalesce(
					(array_agg(
						line.exchange_rate_source
						ORDER BY
							(line.currency_code = entry.functional_currency),
							line.line_no
					))[1],
					''
				) AS exchange_rate_source
			  FROM accounting.journal_lines AS line
			 WHERE line.org_id = entry.org_id
			   AND line.journal_entry_id = entry.id
		  ) AS totals ON true
		 WHERE entry.org_id = $1
		   AND ($2::date IS NULL OR entry.entry_date >= $2)
		   AND ($3::date IS NULL OR entry.entry_date <= $3)
		   AND ($4 = '' OR entry.source_type = $4)
		   AND ($5::uuid IS NULL OR entry.source_id = $5::text)
		   AND ($6::bigint = 0 OR entry.entry_number < $6)
		   AND (
				$7::uuid IS NULL
				OR EXISTS (
					SELECT 1
					  FROM accounting.journal_lines AS line
					 WHERE line.org_id = entry.org_id
					   AND line.journal_entry_id = entry.id
					   AND line.account_id = $7
				)
		   )
		   AND (
				$8::text = ''
				OR EXISTS (
					SELECT 1
					  FROM accounting.journal_lines AS line
					 WHERE line.org_id = entry.org_id
					   AND line.journal_entry_id = entry.id
					   AND line.party_id = $8
				)
		   )
		   AND (
				$9::text = ''
				OR entry.description ILIKE '%' || $9 || '%'
				OR lower(entry.reference) LIKE lower($9) || '%'
				OR coalesce(entry.source_type, '') ILIKE '%' || $9 || '%'
				OR coalesce(entry.source_id, '') ILIKE '%' || $9 || '%'
				OR entry.entry_number::text = $9
				OR EXISTS (
					SELECT 1
					  FROM accounting.journal_lines AS search_line
					  JOIN accounting.accounts AS search_account
					    ON search_account.org_id = search_line.org_id
					   AND search_account.id = search_line.account_id
					 WHERE search_line.org_id = entry.org_id
					   AND search_line.journal_entry_id = entry.id
					   AND (
							search_line.description ILIKE '%' || $9 || '%'
							OR search_account.code ILIKE '%' || $9 || '%'
							OR search_account.name ILIKE '%' || $9 || '%'
					   )
				)
		   )
		   AND (
				$10::text = ''
				OR ($10 = 'reversal' AND entry.reverses_entry_id IS NOT NULL)
				OR ($10 = 'reversed' AND direct_reversal.id IS NOT NULL)
				OR (
					$10 = 'regular'
					AND entry.reverses_entry_id IS NULL
					AND direct_reversal.id IS NULL
				)
		   )
		 ORDER BY entry.entry_number DESC
		 LIMIT $11
	`,
		repository.orgID,
		filter.From,
		filter.To,
		filter.SourceType,
		filter.SourceID,
		filter.AfterNumber,
		filter.AccountID,
		partyFilter(filter.PartyID),
		filter.Query,
		filter.ReversalState,
		limit+1,
	)
	if err != nil {
		return accounting.PageResult[accounting.JournalEntry]{}, mapError(err)
	}
	defer rows.Close()
	entries := make([]accounting.JournalEntry, 0, limit+1)
	for rows.Next() {
		entry, scanErr := scanJournalSummary(rows)
		if scanErr != nil {
			return accounting.PageResult[accounting.JournalEntry]{}, scanErr
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return accounting.PageResult[accounting.JournalEntry]{}, mapError(err)
	}
	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}
	result := accounting.PageResult[accounting.JournalEntry]{
		Items: entries,
	}
	if filter.IncludeLines && len(result.Items) > 0 {
		entryIDs := make([]uuid.UUID, 0, len(result.Items))
		for _, entry := range result.Items {
			entryIDs = append(entryIDs, entry.ID)
		}
		linesByEntry, getErr := repository.listJournalLinesByEntryIDs(ctx, entryIDs)
		if getErr != nil {
			return accounting.PageResult[accounting.JournalEntry]{}, getErr
		}
		for index := range result.Items {
			result.Items[index].Lines = linesByEntry[result.Items[index].ID]
		}
	}
	if hasMore && len(result.Items) > 0 {
		result.NextCursor = fmt.Sprintf("%d", result.Items[len(result.Items)-1].Number)
	}
	return result, nil
}

func scanJournalSummary(row scanner) (accounting.JournalEntry, error) {
	var (
		entry          accounting.JournalEntry
		functionalCode string
		currencyCode   string
		sourceID       string
		debitText      string
		creditText     string
		rateText       string
		rateDate       *time.Time
	)
	if err := row.Scan(
		&entry.ID,
		&entry.Number,
		&entry.Date,
		&entry.Kind,
		&entry.PostingKind,
		&functionalCode,
		&entry.Reference,
		&entry.Source.Type,
		&sourceID,
		&entry.Source.Event,
		&entry.Source.IdempotencyKey,
		&entry.Description,
		&entry.CreatedBy,
		&entry.CreatedAt,
		&entry.ReversesEntryID,
		&entry.ReversesEntryNumber,
		&entry.ReversalReason,
		&entry.DraftID,
		&entry.ReversedByEntryID,
		&entry.ReversedByEntryNumber,
		&debitText,
		&creditText,
		&currencyCode,
		&rateText,
		&rateDate,
		&entry.ExchangeRateSource,
	); err != nil {
		return accounting.JournalEntry{}, mapError(err)
	}
	var err error
	entry.FunctionalCurrency, err = accounting.NewCurrency(functionalCode)
	if err != nil {
		return accounting.JournalEntry{}, err
	}
	entry.Currency, err = accounting.NewCurrency(currencyCode)
	if err != nil {
		return accounting.JournalEntry{}, err
	}
	entry.DebitTotal, err = accounting.ParseAmount(debitText)
	if err != nil {
		return accounting.JournalEntry{}, err
	}
	entry.CreditTotal, err = accounting.ParseAmount(creditText)
	if err != nil {
		return accounting.JournalEntry{}, err
	}
	entry.ExchangeRate, err = accounting.ParseExchangeRate(rateText)
	if err != nil {
		return accounting.JournalEntry{}, err
	}
	if rateDate != nil {
		entry.ExchangeRateDate = *rateDate
	}
	if sourceID != "" {
		entry.Source.ID, err = uuid.Parse(sourceID)
		if err != nil {
			return accounting.JournalEntry{}, fmt.Errorf(
				"accounting postgres: invalid source id: %w",
				err,
			)
		}
	}
	entry.IsAdjustment = entry.Kind == accounting.EntryAdjustment ||
		entry.Kind == accounting.EntryClosing ||
		entry.Kind == accounting.EntryInflation ||
		entry.Kind == accounting.EntryRevaluation ||
		entry.Kind == accounting.EntryReversal
	return entry, nil
}

func (repository *Repository) ReportLines(
	ctx context.Context,
	from time.Time,
	to time.Time,
) ([]accounting.ReportLine, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			entry.id,
			entry.entry_number,
			entry.entry_date,
			entry.description,
			coalesce(entry.source_type, ''),
			coalesce(entry.source_id, ''),
			line.id,
			line.account_id,
			account.code,
			account.name,
			account.account_class,
			account.normal_balance,
			line.debit_amount::text,
			line.credit_amount::text,
			line.party_id,
			line.line_no
		  FROM accounting.journal_entries AS entry
		  JOIN accounting.journal_lines AS line
		    ON line.org_id = entry.org_id
		   AND line.journal_entry_id = entry.id
		  JOIN accounting.accounts AS account
		    ON account.org_id = line.org_id
		   AND account.id = line.account_id
		 WHERE entry.org_id = $1
		   AND entry.entry_date BETWEEN $2 AND $3
		 ORDER BY entry.entry_date, entry.entry_number, line.line_no
	`, repository.orgID, from, to)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.ReportLine, 0)
	for rows.Next() {
		var (
			line       accounting.ReportLine
			sourceID   string
			debitText  string
			creditText string
			partyID    *string
		)
		if err := rows.Scan(
			&line.EntryID,
			&line.EntryNumber,
			&line.EntryDate,
			&line.Description,
			&line.SourceType,
			&sourceID,
			&line.LineID,
			&line.AccountID,
			&line.AccountCode,
			&line.AccountName,
			&line.AccountClass,
			&line.NormalBalance,
			&debitText,
			&creditText,
			&partyID,
			&line.LineNo,
		); err != nil {
			return nil, mapError(err)
		}
		line.Debit, err = accounting.ParseAmount(debitText)
		if err != nil {
			return nil, err
		}
		line.Credit, err = accounting.ParseAmount(creditText)
		if err != nil {
			return nil, err
		}
		if sourceID != "" {
			line.SourceID, _ = uuid.Parse(sourceID)
		}
		if partyID != nil {
			parsed, parseErr := uuid.Parse(*partyID)
			if parseErr == nil {
				line.PartyID = &parsed
			}
		}
		result = append(result, line)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func (repository *Repository) AccountOpeningBalance(
	ctx context.Context,
	accountID uuid.UUID,
	before time.Time,
) (accounting.Decimal, error) {
	var value string
	if err := repository.tx.QueryRow(ctx, `
		SELECT coalesce(sum(line.debit_amount - line.credit_amount), 0)::text
		  FROM accounting.journal_lines AS line
		  JOIN accounting.journal_entries AS entry
		    ON entry.org_id = line.org_id
		   AND entry.id = line.journal_entry_id
		 WHERE line.org_id = $1
		   AND line.account_id = $2
		   AND entry.entry_date < $3
	`, repository.orgID, accountID, before).Scan(&value); err != nil {
		return accounting.Zero, mapError(err)
	}
	return accounting.ParseAmount(value)
}

func (repository *Repository) ListOpenItems(
	ctx context.Context,
	filter accounting.OpenItemFilter,
) ([]accounting.OpenItem, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			item.id,
			item.item_type,
			item.party_id,
			item.account_id,
			item.origin_journal_entry_id,
			item.origin_journal_line_id,
			item.document_type,
			item.document_id,
			item.issued_at,
			item.due_date,
			item.currency_code,
			item.original_currency_amount::text,
			item.original_functional_amount::text,
			balance.remaining_currency_amount::text,
			balance.remaining_functional_amount::text
		  FROM accounting.open_items AS item
		  JOIN accounting.open_item_balances_as_of(
				coalesce($4::date, 'infinity'::date)
		  ) AS balance
		    ON balance.org_id = item.org_id
		   AND balance.open_item_id = item.id
		 WHERE item.org_id = $1
		   AND ($2 = '' OR item.item_type = $2)
		   AND ($3 = '' OR item.party_id = $3)
		   AND ($4::date IS NULL OR item.issued_at <= $4)
		   AND ($5 = '' OR item.currency_code = $5)
		   AND (
				$6::boolean IS NULL
				OR NOT $6
				OR (item.due_date < $4 AND balance.remaining_currency_amount > 0)
		   )
		 ORDER BY item.due_date NULLS LAST, item.id
	`,
		repository.orgID,
		filter.Kind,
		partyFilter(filter.PartyID),
		nullableDate(filter.AsOf),
		currencyFilter(filter.Currency),
		filter.Overdue,
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.OpenItem, 0)
	for rows.Next() {
		item, scanErr := scanOpenItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func (repository *Repository) CreateOpenItem(
	ctx context.Context,
	item accounting.OpenItem,
) (accounting.OpenItem, error) {
	var dueDate any
	if !item.DueDate.IsZero() {
		dueDate = item.DueDate
	}
	if _, err := repository.tx.Exec(ctx, `
		INSERT INTO accounting.open_items (
			org_id,
			id,
			item_type,
			party_type,
			party_id,
			account_id,
			origin_journal_entry_id,
			origin_journal_line_id,
			document_type,
			document_id,
			currency_code,
			original_currency_amount,
			original_functional_amount,
			issued_at,
			due_date
		)
		VALUES (
			$1, $2, $3, 'party', $4, $5, $6, $7, $8, $9,
			$10, $11::numeric, $12::numeric, $13, $14
		)
	`,
		repository.orgID,
		item.ID,
		item.Kind,
		item.PartyID.String(),
		item.AccountID,
		item.EntryID,
		item.OriginLineID,
		item.SourceType,
		item.SourceID.String(),
		item.Currency.Code(),
		item.OriginalAmount.String(),
		item.FunctionalAmount.String(),
		item.IssueDate,
		dueDate,
	); err != nil {
		return accounting.OpenItem{}, mapError(err)
	}
	item.OpenAmount = item.OriginalAmount
	item.OpenFunctional = item.FunctionalAmount
	return item, nil
}

func (repository *Repository) ApplyOpenItem(
	ctx context.Context,
	application accounting.OpenItemApplication,
) (accounting.OpenItem, error) {
	if _, err := repository.tx.Exec(ctx, `
		INSERT INTO accounting.open_item_applications (
			org_id,
			id,
			open_item_id,
			settlement_journal_entry_id,
			settlement_journal_line_id,
			currency_amount,
			functional_amount,
			exchange_difference_amount,
			applied_by,
			applied_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6::numeric, $7::numeric,
			$8::numeric, $9, $10
		)
	`,
		repository.orgID,
		application.ID,
		application.OpenItemID,
		application.SettlementEntryID,
		application.SettlementLineID,
		application.Amount.String(),
		application.FunctionalAmount.String(),
		application.ExchangeDifference.String(),
		application.CreatedBy,
		application.AppliedAt,
	); err != nil {
		return accounting.OpenItem{}, mapError(err)
	}
	items, err := repository.ListOpenItems(ctx, accounting.OpenItemFilter{})
	if err != nil {
		return accounting.OpenItem{}, err
	}
	for _, item := range items {
		if item.ID == application.OpenItemID {
			return item, nil
		}
	}
	return accounting.OpenItem{}, accounting.ErrNotFound
}

func (repository *Repository) scanEntry(row scanner) (accounting.JournalEntry, error) {
	var (
		entry          accounting.JournalEntry
		functionalCode string
		sourceID       string
	)
	if err := row.Scan(
		&entry.ID,
		&entry.Number,
		&entry.Date,
		&entry.Kind,
		&entry.PostingKind,
		&functionalCode,
		&entry.Reference,
		&entry.Source.Type,
		&sourceID,
		&entry.Source.Event,
		&entry.Source.IdempotencyKey,
		&entry.Description,
		&entry.CreatedBy,
		&entry.CreatedAt,
		&entry.ReversesEntryID,
		&entry.ReversesEntryNumber,
		&entry.ReversalReason,
		&entry.DraftID,
		&entry.ReversedByEntryID,
		&entry.ReversedByEntryNumber,
	); err != nil {
		return accounting.JournalEntry{}, mapError(err)
	}
	functional, err := accounting.NewCurrency(functionalCode)
	if err != nil {
		return accounting.JournalEntry{}, err
	}
	entry.FunctionalCurrency = functional
	if sourceID != "" {
		entry.Source.ID, err = uuid.Parse(sourceID)
		if err != nil {
			return accounting.JournalEntry{}, fmt.Errorf("accounting postgres: invalid source id: %w", err)
		}
	}
	entry.IsAdjustment = entry.Kind == accounting.EntryAdjustment ||
		entry.Kind == accounting.EntryClosing ||
		entry.Kind == accounting.EntryInflation ||
		entry.Kind == accounting.EntryRevaluation ||
		entry.Kind == accounting.EntryReversal
	return entry, nil
}

func (repository *Repository) listJournalLines(
	ctx context.Context,
	entryID uuid.UUID,
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
		  FROM accounting.journal_lines AS line
		  JOIN accounting.accounts AS account
		    ON account.org_id = line.org_id
		   AND account.id = line.account_id
		 WHERE line.org_id = $1
		   AND line.journal_entry_id = $2
		 ORDER BY line.line_no
	`, repository.orgID, entryID)
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
			if parseErr == nil {
				line.PartyID = &parsed
			}
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return lines, nil
}

func (repository *Repository) listJournalLinesByEntryIDs(
	ctx context.Context,
	entryIDs []uuid.UUID,
) (map[uuid.UUID][]accounting.JournalLine, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			line.journal_entry_id,
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
		  FROM accounting.journal_lines AS line
		  JOIN accounting.accounts AS account
		    ON account.org_id = line.org_id
		   AND account.id = line.account_id
		 WHERE line.org_id = $1
		   AND line.journal_entry_id = ANY($2::uuid[])
		 ORDER BY line.journal_entry_id, line.line_no
	`, repository.orgID, entryIDs)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	linesByEntry := make(map[uuid.UUID][]accounting.JournalLine, len(entryIDs))
	for rows.Next() {
		var (
			entryID        uuid.UUID
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
			&entryID,
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
		if err := hydrateLineAmounts(
			&line,
			debitText,
			creditText,
			currencyCode,
			currencyAmount,
			rateText,
		); err != nil {
			return nil, err
		}
		if rateDate != nil {
			line.ExchangeRateDate = *rateDate
		}
		if partyID != nil {
			parsed, parseErr := uuid.Parse(*partyID)
			if parseErr == nil {
				line.PartyID = &parsed
			}
		}
		linesByEntry[entryID] = append(linesByEntry[entryID], line)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return linesByEntry, nil
}

func hydrateLineAmounts(
	line *accounting.JournalLine,
	debitText string,
	creditText string,
	currencyCode string,
	currencyAmount string,
	rateText string,
) error {
	var err error
	line.Debit, err = accounting.ParseAmount(debitText)
	if err != nil {
		return err
	}
	line.Credit, err = accounting.ParseAmount(creditText)
	if err != nil {
		return err
	}
	transaction, err := accounting.ParseAmount(currencyAmount)
	if err != nil {
		return err
	}
	if !line.Debit.IsZero() {
		line.TransactionDebit = transaction
	} else {
		line.TransactionCredit = transaction
	}
	line.Currency, err = accounting.NewCurrency(currencyCode)
	if err != nil {
		return err
	}
	line.ExchangeRate, err = accounting.ParseExchangeRate(rateText)
	return err
}

func scanOpenItem(row scanner) (accounting.OpenItem, error) {
	var (
		item               accounting.OpenItem
		partyID            string
		documentID         string
		currencyCode       string
		originalText       string
		functionalText     string
		openText           string
		openFunctionalText string
		dueDate            *time.Time
	)
	if err := row.Scan(
		&item.ID,
		&item.Kind,
		&partyID,
		&item.AccountID,
		&item.EntryID,
		&item.OriginLineID,
		&item.SourceType,
		&documentID,
		&item.IssueDate,
		&dueDate,
		&currencyCode,
		&originalText,
		&functionalText,
		&openText,
		&openFunctionalText,
	); err != nil {
		return accounting.OpenItem{}, mapError(err)
	}
	var err error
	item.PartyID, err = uuid.Parse(partyID)
	if err != nil {
		return accounting.OpenItem{}, err
	}
	item.SourceID, err = uuid.Parse(documentID)
	if err != nil {
		return accounting.OpenItem{}, err
	}
	item.Currency, err = accounting.NewCurrency(currencyCode)
	if err != nil {
		return accounting.OpenItem{}, err
	}
	item.OriginalAmount, err = accounting.ParseAmount(originalText)
	if err != nil {
		return accounting.OpenItem{}, err
	}
	item.FunctionalAmount, err = accounting.ParseAmount(functionalText)
	if err != nil {
		return accounting.OpenItem{}, err
	}
	item.OpenAmount, err = accounting.ParseAmount(openText)
	if err != nil {
		return accounting.OpenItem{}, err
	}
	item.OpenFunctional, err = accounting.ParseAmount(openFunctionalText)
	if dueDate != nil {
		item.DueDate = *dueDate
	}
	return item, err
}

func partyFilter(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func currencyFilter(currency *accounting.Currency) string {
	if currency == nil {
		return ""
	}
	return currency.Code()
}

func nullableDate(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
