package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

func (repository *Repository) CreateStatementImport(
	ctx context.Context,
	statement accounting.StatementImport,
) (accounting.StatementImport, error) {
	if _, err := repository.tx.Exec(ctx, `
		INSERT INTO accounting.statement_imports (
			org_id,
			id,
			financial_account_id,
			file_name,
			file_format,
			file_sha256,
			imported_by,
			imported_at,
			row_count
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		repository.orgID,
		statement.ID,
		statement.FinancialAccountID,
		statement.FileName,
		statement.Format,
		statement.SHA256,
		statement.ImportedBy,
		statement.ImportedAt,
		len(statement.Movements),
	); err != nil {
		return accounting.StatementImport{}, mapError(err)
	}
	for _, movement := range statement.Movements {
		if _, err := repository.tx.Exec(ctx, `
			INSERT INTO accounting.statement_transactions (
				org_id,
				id,
				financial_account_id,
				statement_import_id,
				external_id,
				fingerprint_sha256,
				booked_at,
				value_date,
				amount,
				currency_code,
				reference,
				description
			)
			VALUES (
				$1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8,
				$9::numeric, $10, NULLIF($11, ''), $12
			)
		`,
			repository.orgID,
			movement.ID,
			statement.FinancialAccountID,
			statement.ID,
			movement.Reference,
			movement.Fingerprint,
			movement.BookedAt,
			nullableDate(movement.ValueAt),
			movement.Amount.String(),
			movement.Currency.Code(),
			movement.Reference,
			movement.Description,
		); err != nil {
			return accounting.StatementImport{}, mapError(err)
		}
	}
	return statement, nil
}

func (repository *Repository) FindStatementImportByHash(
	ctx context.Context,
	accountID uuid.UUID,
	hash string,
) (accounting.StatementImport, error) {
	var statement accounting.StatementImport
	if err := repository.tx.QueryRow(ctx, `
		SELECT
			id,
			financial_account_id,
			file_name,
			file_format,
			file_sha256,
			imported_by,
			imported_at
		  FROM accounting.statement_imports
		 WHERE org_id = $1
		   AND financial_account_id = $2
		   AND file_sha256 = $3
		 ORDER BY imported_at
		 LIMIT 1
	`, repository.orgID, accountID, hash).Scan(
		&statement.ID,
		&statement.FinancialAccountID,
		&statement.FileName,
		&statement.Format,
		&statement.SHA256,
		&statement.ImportedBy,
		&statement.ImportedAt,
	); err != nil {
		return accounting.StatementImport{}, mapError(err)
	}
	movements, err := repository.ListStatementMovements(ctx, statement.ID)
	if err != nil {
		return accounting.StatementImport{}, err
	}
	statement.Movements = movements
	if len(movements) > 0 {
		statement.Currency = movements[0].Currency
	}
	return statement, nil
}

func (repository *Repository) ListStatementMovements(
	ctx context.Context,
	importID uuid.UUID,
) ([]accounting.StatementMovement, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			id,
			statement_import_id,
			booked_at,
			value_date,
			description,
			coalesce(reference, ''),
			amount::text,
			currency_code,
			fingerprint_sha256
		  FROM accounting.statement_transactions
		 WHERE org_id = $1
		   AND statement_import_id = $2
		 ORDER BY booked_at, id
	`, repository.orgID, importID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.StatementMovement, 0)
	for rows.Next() {
		var (
			movement     accounting.StatementMovement
			valueDate    *time.Time
			amountText   string
			currencyCode string
		)
		if err := rows.Scan(
			&movement.ID,
			&movement.ImportID,
			&movement.BookedAt,
			&valueDate,
			&movement.Description,
			&movement.Reference,
			&amountText,
			&currencyCode,
			&movement.Fingerprint,
		); err != nil {
			return nil, mapError(err)
		}
		if valueDate != nil {
			movement.ValueAt = *valueDate
		} else {
			movement.ValueAt = movement.BookedAt
		}
		movement.Amount, err = accounting.ParseAmount(amountText)
		if err != nil {
			return nil, err
		}
		movement.Currency, err = accounting.NewCurrency(currencyCode)
		if err != nil {
			return nil, err
		}
		result = append(result, movement)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func (repository *Repository) CreateReconciliation(
	ctx context.Context,
	reconciliation accounting.Reconciliation,
) (accounting.Reconciliation, error) {
	status := reconciliationStatusToDatabase(reconciliation.Status)
	if status == "" {
		status = "draft"
	}
	if _, err := repository.tx.Exec(ctx, `
		INSERT INTO accounting.reconciliations (
			org_id,
			id,
			financial_account_id,
			start_date,
			end_date,
			opening_balance,
			closing_balance,
			status,
			version,
			created_by
		)
		VALUES (
			$1, $2, $3, $4, $5, $6::numeric, $7::numeric,
			$8, 1, $9
		)
	`,
		repository.orgID,
		reconciliation.ID,
		reconciliation.FinancialAccountID,
		reconciliation.PeriodStart,
		reconciliation.PeriodEnd,
		reconciliation.StatementOpening.String(),
		reconciliation.StatementClosing.String(),
		status,
		repository.actor,
	); err != nil {
		return accounting.Reconciliation{}, mapError(err)
	}
	if err := repository.replaceReconciliationMatches(ctx, reconciliation); err != nil {
		return accounting.Reconciliation{}, err
	}
	return repository.GetReconciliation(ctx, reconciliation.ID, false)
}

func (repository *Repository) GetReconciliation(
	ctx context.Context,
	id uuid.UUID,
	forUpdate bool,
) (accounting.Reconciliation, error) {
	query := `
		SELECT
			id,
			financial_account_id,
			start_date,
			end_date,
			opening_balance::text,
			closing_balance::text,
			status,
			version,
			coalesce(status_changed_by, ''),
			coalesce(transition_reason, '')
		  FROM accounting.reconciliations
		 WHERE org_id = $1
		   AND id = $2
	`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var (
		reconciliation accounting.Reconciliation
		openingText    string
		closingText    string
		status         string
		statusActor    string
		reason         string
	)
	if err := repository.tx.QueryRow(ctx, query, repository.orgID, id).Scan(
		&reconciliation.ID,
		&reconciliation.FinancialAccountID,
		&reconciliation.PeriodStart,
		&reconciliation.PeriodEnd,
		&openingText,
		&closingText,
		&status,
		&reconciliation.Version,
		&statusActor,
		&reason,
	); err != nil {
		return accounting.Reconciliation{}, mapError(err)
	}
	var err error
	reconciliation.StatementOpening, err = accounting.ParseAmount(openingText)
	if err != nil {
		return accounting.Reconciliation{}, err
	}
	reconciliation.StatementClosing, err = accounting.ParseAmount(closingText)
	if err != nil {
		return accounting.Reconciliation{}, err
	}
	reconciliation.Status = reconciliationStatusFromDatabase(status)
	if reconciliation.Status == accounting.ReconciliationClosed {
		reconciliation.ClosedBy = statusActor
	} else if reason != "" {
		reconciliation.ReopenedBy = statusActor
		reconciliation.ReopenedReason = reason
	}
	if err := repository.loadReconciliationLedgerBalances(ctx, &reconciliation); err != nil {
		return accounting.Reconciliation{}, err
	}
	matches, err := repository.listReconciliationMatches(ctx, id)
	if err != nil {
		return accounting.Reconciliation{}, err
	}
	reconciliation.Matches = matches
	return reconciliation, nil
}

func (repository *Repository) ListReconciliations(
	ctx context.Context,
	page accounting.PageRequest,
) (accounting.PageResult[accounting.Reconciliation], error) {
	limit := page.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var after any
	if page.After != "" {
		parsed, err := uuid.Parse(page.After)
		if err != nil {
			return accounting.PageResult[accounting.Reconciliation]{}, fmt.Errorf("%w: invalid reconciliation cursor", accounting.ErrInvalidArgument)
		}
		after = parsed
	}
	rows, err := repository.tx.Query(ctx, `
		SELECT id
		  FROM accounting.reconciliations
		 WHERE org_id = $1
		   AND ($2::uuid IS NULL OR id > $2)
		 ORDER BY id
		 LIMIT $3
	`, repository.orgID, after, limit+1)
	if err != nil {
		return accounting.PageResult[accounting.Reconciliation]{}, mapError(err)
	}
	defer rows.Close()
	ids := make([]uuid.UUID, 0, limit+1)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return accounting.PageResult[accounting.Reconciliation]{}, mapError(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return accounting.PageResult[accounting.Reconciliation]{}, mapError(err)
	}
	hasMore := len(ids) > limit
	if hasMore {
		ids = ids[:limit]
	}
	result := accounting.PageResult[accounting.Reconciliation]{
		Items: make([]accounting.Reconciliation, 0, len(ids)),
	}
	for _, id := range ids {
		reconciliation, getErr := repository.GetReconciliation(ctx, id, false)
		if getErr != nil {
			return accounting.PageResult[accounting.Reconciliation]{}, getErr
		}
		result.Items = append(result.Items, reconciliation)
	}
	if hasMore && len(ids) > 0 {
		result.NextCursor = ids[len(ids)-1].String()
	}
	return result, nil
}

func (repository *Repository) SaveReconciliation(
	ctx context.Context,
	reconciliation accounting.Reconciliation,
	expectedVersion int64,
) (accounting.Reconciliation, error) {
	current, err := repository.GetReconciliation(ctx, reconciliation.ID, true)
	if err != nil {
		return accounting.Reconciliation{}, err
	}
	if current.Version != expectedVersion {
		return accounting.Reconciliation{}, accounting.ErrVersionConflict
	}
	targetStatus := reconciliationStatusToDatabase(reconciliation.Status)
	if current.Status == accounting.ReconciliationClosed &&
		reconciliation.Status == accounting.ReconciliationOpen {
		if _, err := repository.updateReconciliationRow(ctx, reconciliation, expectedVersion, targetStatus); err != nil {
			return accounting.Reconciliation{}, err
		}
		expectedVersion++
		if err := repository.replaceReconciliationMatches(ctx, reconciliation); err != nil {
			return accounting.Reconciliation{}, err
		}
	} else {
		if err := repository.replaceReconciliationMatches(ctx, reconciliation); err != nil {
			return accounting.Reconciliation{}, err
		}
		if _, err := repository.updateReconciliationRow(ctx, reconciliation, expectedVersion, targetStatus); err != nil {
			return accounting.Reconciliation{}, err
		}
	}
	return repository.GetReconciliation(ctx, reconciliation.ID, false)
}

func (repository *Repository) ListReconciliationCandidates(
	ctx context.Context,
	financialAccountID uuid.UUID,
	from time.Time,
	to time.Time,
) ([]accounting.ReconciliationLedgerCandidate, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			movement.journal_line_id,
			movement.entry_date,
			movement.signed_amount::text,
			coalesce(movement.source_id, ''),
			movement.description
		  FROM accounting.financial_account_movements_view AS movement
		 WHERE movement.org_id = $1
		   AND movement.financial_account_id = $2
		   AND movement.entry_date BETWEEN $3 AND $4
		 ORDER BY movement.entry_date, movement.entry_number, movement.journal_line_id
	`, repository.orgID, financialAccountID, from, to)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.ReconciliationLedgerCandidate, 0)
	for rows.Next() {
		var candidate accounting.ReconciliationLedgerCandidate
		var amountText string
		if err := rows.Scan(
			&candidate.JournalLineID,
			&candidate.Date,
			&amountText,
			&candidate.Reference,
			&candidate.Description,
		); err != nil {
			return nil, mapError(err)
		}
		candidate.Amount, err = accounting.ParseAmount(amountText)
		if err != nil {
			return nil, err
		}
		result = append(result, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func (repository *Repository) updateReconciliationRow(
	ctx context.Context,
	reconciliation accounting.Reconciliation,
	expectedVersion int64,
	status string,
) (int64, error) {
	reason := reconciliation.ReopenedReason
	commandTag, err := repository.tx.Exec(ctx, `
		UPDATE accounting.reconciliations
		   SET opening_balance = $3::numeric,
		       closing_balance = $4::numeric,
		       status = $5,
		       version = version + 1,
		       status_changed_by = $6,
		       transition_reason = NULLIF($7, ''),
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
		   AND version = $8
	`,
		repository.orgID,
		reconciliation.ID,
		reconciliation.StatementOpening.String(),
		reconciliation.StatementClosing.String(),
		status,
		repository.actor,
		reason,
		expectedVersion,
	)
	if err != nil {
		return 0, mapError(err)
	}
	if commandTag.RowsAffected() == 0 {
		return 0, accounting.ErrVersionConflict
	}
	return commandTag.RowsAffected(), nil
}

func (repository *Repository) replaceReconciliationMatches(
	ctx context.Context,
	reconciliation accounting.Reconciliation,
) error {
	if _, err := repository.tx.Exec(ctx, `
		DELETE FROM accounting.reconciliation_matches
		 WHERE org_id = $1
		   AND reconciliation_id = $2
	`, repository.orgID, reconciliation.ID); err != nil {
		return mapError(err)
	}
	for _, match := range reconciliation.Matches {
		var entryID uuid.UUID
		if err := repository.tx.QueryRow(ctx, `
			SELECT journal_entry_id
			  FROM accounting.journal_lines
			 WHERE org_id = $1
			   AND id = $2
		`, repository.orgID, match.JournalLineID).Scan(&entryID); err != nil {
			return mapError(err)
		}
		if _, err := repository.tx.Exec(ctx, `
			INSERT INTO accounting.reconciliation_matches (
				org_id,
				id,
				reconciliation_id,
				statement_transaction_id,
				journal_entry_id,
				journal_line_id,
				matched_amount,
				functional_amount,
				match_source,
				matched_by,
				matched_at
			)
			VALUES (
				$1, $2, $3, $4, $5, $6, $7::numeric, $8::numeric,
				'manual', $9, $10
			)
		`,
			repository.orgID,
			match.ID,
			reconciliation.ID,
			match.StatementMovementID,
			entryID,
			match.JournalLineID,
			match.StatementAmount.String(),
			match.LedgerAmount.String(),
			match.CreatedBy,
			match.CreatedAt,
		); err != nil {
			return mapError(err)
		}
	}
	return nil
}

func (repository *Repository) listReconciliationMatches(
	ctx context.Context,
	reconciliationID uuid.UUID,
) ([]accounting.ReconciliationMatch, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			id,
			statement_transaction_id,
			journal_line_id,
			matched_amount::text,
			functional_amount::text,
			matched_by,
			matched_at
		  FROM accounting.reconciliation_matches
		 WHERE org_id = $1
		   AND reconciliation_id = $2
		 ORDER BY matched_at, id
	`, repository.orgID, reconciliationID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.ReconciliationMatch, 0)
	for rows.Next() {
		var match accounting.ReconciliationMatch
		var statementText, ledgerText string
		if err := rows.Scan(
			&match.ID,
			&match.StatementMovementID,
			&match.JournalLineID,
			&statementText,
			&ledgerText,
			&match.CreatedBy,
			&match.CreatedAt,
		); err != nil {
			return nil, mapError(err)
		}
		match.StatementAmount, err = accounting.ParseAmount(statementText)
		if err != nil {
			return nil, err
		}
		match.LedgerAmount, err = accounting.ParseAmount(ledgerText)
		if err != nil {
			return nil, err
		}
		result = append(result, match)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func (repository *Repository) loadReconciliationLedgerBalances(
	ctx context.Context,
	reconciliation *accounting.Reconciliation,
) error {
	var openingText, closingText string
	if err := repository.tx.QueryRow(ctx, `
		SELECT
			coalesce(sum(
				CASE WHEN movement.entry_date < $3
					THEN movement.signed_amount ELSE 0 END
			), 0)::text,
			coalesce(sum(
				CASE WHEN movement.entry_date <= $4
					THEN movement.signed_amount ELSE 0 END
			), 0)::text
		  FROM accounting.financial_account_movements_view AS movement
		 WHERE movement.org_id = $1
		   AND movement.financial_account_id = $2
	`,
		repository.orgID,
		reconciliation.FinancialAccountID,
		reconciliation.PeriodStart,
		reconciliation.PeriodEnd,
	).Scan(&openingText, &closingText); err != nil {
		return mapError(err)
	}
	var err error
	reconciliation.LedgerOpening, err = accounting.ParseAmount(openingText)
	if err != nil {
		return err
	}
	reconciliation.LedgerClosing, err = accounting.ParseAmount(closingText)
	return err
}

func (repository *Repository) UpsertInflationIndices(
	ctx context.Context,
	indices []accounting.InflationIndex,
) error {
	for _, index := range indices {
		if _, err := repository.tx.Exec(ctx, `
			INSERT INTO accounting.inflation_indices (
				org_id,
				series_code,
				period_month,
				index_value,
				source_url,
				source_checksum,
				imported_by
			)
			VALUES ($1, 'FACPCE', $2, $3::numeric, $4, $5, $6)
			ON CONFLICT (
				org_id,
				series_code,
				period_month,
				source_checksum
			) DO NOTHING
		`,
			repository.orgID,
			index.Period,
			index.Value.String(),
			index.Source,
			index.Checksum,
			repository.actor,
		); err != nil {
			return mapError(err)
		}
	}
	return nil
}

func (repository *Repository) ListInflationIndices(
	ctx context.Context,
	from time.Time,
	to time.Time,
) ([]accounting.InflationIndex, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT DISTINCT ON (period_month)
			period_month,
			index_value::text,
			source_url,
			source_checksum
		  FROM accounting.inflation_indices
		 WHERE org_id = $1
		   AND period_month BETWEEN date_trunc('month', $2::date)
		       AND date_trunc('month', $3::date)
		 ORDER BY period_month, imported_at DESC, id DESC
	`, repository.orgID, from, to)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.InflationIndex, 0)
	for rows.Next() {
		var index accounting.InflationIndex
		var valueText string
		if err := rows.Scan(&index.Period, &valueText, &index.Source, &index.Checksum); err != nil {
			return nil, mapError(err)
		}
		index.Value, err = accounting.ParseDecimal(valueText)
		if err != nil {
			return nil, err
		}
		result = append(result, index)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func (repository *Repository) ListInflationPositions(
	ctx context.Context,
	asOf time.Time,
) ([]accounting.InflationPosition, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			account.id,
			account.code,
			account.name,
			account.normal_balance,
			account.monetary_class,
			coalesce(
				min(line.origin_date) FILTER (WHERE entry.id IS NOT NULL),
				min(entry.entry_date),
				$2::date
			),
			coalesce(
				sum(line.debit_amount - line.credit_amount)
					FILTER (WHERE entry.id IS NOT NULL),
				0
			)::text
		  FROM accounting.accounts AS account
		  LEFT JOIN accounting.journal_lines AS line
		    ON line.org_id = account.org_id
		   AND line.account_id = account.id
		  LEFT JOIN accounting.journal_entries AS entry
		    ON entry.org_id = line.org_id
		   AND entry.id = line.journal_entry_id
		   AND entry.entry_date <= $2
		 WHERE account.org_id = $1
		   AND account.monetary_class = 'non_monetary'
		   AND account.posting_allowed
		   AND account.archived_at IS NULL
		   AND account.trashed_at IS NULL
		 GROUP BY
			account.id,
			account.code,
			account.name,
			account.normal_balance,
			account.monetary_class
		 ORDER BY account.code
	`, repository.orgID, asOf)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	result := make([]accounting.InflationPosition, 0)
	for rows.Next() {
		var position accounting.InflationPosition
		var balanceText string
		if err := rows.Scan(
			&position.AccountID,
			&position.AccountCode,
			&position.AccountName,
			&position.NormalBalance,
			&position.Classification,
			&position.OriginDate,
			&balanceText,
		); err != nil {
			return nil, mapError(err)
		}
		position.Balance, err = accounting.ParseAmount(balanceText)
		if err != nil {
			return nil, err
		}
		if position.NormalBalance == accounting.NormalCredit {
			position.Balance = position.Balance.Neg()
		}
		result = append(result, position)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return result, nil
}

func (repository *Repository) CreateInflationWorkpaper(
	ctx context.Context,
	workpaper accounting.InflationWorkpaper,
) (accounting.InflationWorkpaper, error) {
	draft, err := repository.CreateDraft(ctx, workpaper.Draft)
	if err != nil {
		return accounting.InflationWorkpaper{}, err
	}
	workpaper.Draft = draft
	var periodID uuid.UUID
	if err := repository.tx.QueryRow(ctx, `
		SELECT id
		  FROM accounting.periods
		 WHERE org_id = $1
		   AND $2::date BETWEEN start_date AND end_date
	`, repository.orgID, workpaper.ClosingDate).Scan(&periodID); err != nil {
		return accounting.InflationWorkpaper{}, mapError(err)
	}
	encoded, err := json.Marshal(workpaper.Lines)
	if err != nil {
		return accounting.InflationWorkpaper{}, err
	}
	hash := sha256.Sum256(encoded)
	if _, err := repository.tx.Exec(ctx, `
		INSERT INTO accounting.inflation_runs (
			org_id,
			id,
			period_id,
			series_code,
			status,
			generated_draft_id,
			source_checksum,
			workpaper_sha256,
			recpam_amount,
			created_by
		)
		VALUES (
			$1, $2, $3, 'FACPCE', 'draft', $4, $5, $6,
			$7::numeric, $8
		)
	`,
		repository.orgID,
		workpaper.ID,
		periodID,
		draft.ID,
		workpaper.SourceChecksum,
		hex.EncodeToString(hash[:]),
		workpaper.RECPAM.String(),
		workpaper.CreatedBy,
	); err != nil {
		return accounting.InflationWorkpaper{}, mapError(err)
	}
	for index, line := range workpaper.Lines {
		if _, err := repository.tx.Exec(ctx, `
			INSERT INTO accounting.inflation_run_lines (
				org_id,
				inflation_run_id,
				line_no,
				account_id,
				origin_date,
				monetary_class,
				original_amount,
				origin_index,
				closing_index,
				coefficient,
				adjusted_amount,
				adjustment_amount,
				recpam_amount
			)
			VALUES (
				$1, $2, $3, $4, $5, 'non_monetary', $6::numeric,
				$7::numeric, $8::numeric, $9::numeric, $10::numeric,
				$11::numeric, 0
			)
		`,
			repository.orgID,
			workpaper.ID,
			index+1,
			line.AccountID,
			line.OriginDate,
			line.Historical.String(),
			line.OriginIndex.String(),
			line.ClosingIndex.String(),
			line.Coefficient.String(),
			line.Restated.String(),
			line.Adjustment.String(),
		); err != nil {
			return accounting.InflationWorkpaper{}, mapError(err)
		}
	}
	return workpaper, nil
}

func (repository *Repository) GetInflationWorkpaper(
	ctx context.Context,
	id uuid.UUID,
) (accounting.InflationWorkpaper, error) {
	var (
		workpaper  accounting.InflationWorkpaper
		draftID    uuid.UUID
		recpamText string
	)
	if err := repository.tx.QueryRow(ctx, `
		SELECT
			run.id,
			period.end_date,
			run.generated_draft_id,
			run.source_checksum,
			run.recpam_amount::text,
			run.created_by,
			run.created_at
		  FROM accounting.inflation_runs AS run
		  JOIN accounting.periods AS period
		    ON period.org_id = run.org_id
		   AND period.id = run.period_id
		 WHERE run.org_id = $1
		   AND run.id = $2
	`, repository.orgID, id).Scan(
		&workpaper.ID,
		&workpaper.ClosingDate,
		&draftID,
		&workpaper.SourceChecksum,
		&recpamText,
		&workpaper.CreatedBy,
		&workpaper.CreatedAt,
	); err != nil {
		return accounting.InflationWorkpaper{}, mapError(err)
	}
	var err error
	workpaper.RECPAM, err = accounting.ParseAmount(recpamText)
	if err != nil {
		return accounting.InflationWorkpaper{}, err
	}
	workpaper.Draft, err = repository.GetDraft(ctx, draftID, false)
	if err != nil {
		return accounting.InflationWorkpaper{}, err
	}
	workpaper.FunctionalCurrency = workpaper.Draft.FunctionalCurrency
	rows, err := repository.tx.Query(ctx, `
		SELECT
			line.account_id,
			account.code,
			account.name,
			line.origin_date,
			line.origin_index::text,
			line.closing_index::text,
			line.coefficient::text,
			line.original_amount::text,
			line.adjusted_amount::text,
			line.adjustment_amount::text,
			account.normal_balance
		  FROM accounting.inflation_run_lines AS line
		  JOIN accounting.accounts AS account
		    ON account.org_id = line.org_id
		   AND account.id = line.account_id
		 WHERE line.org_id = $1
		   AND line.inflation_run_id = $2
		 ORDER BY line.line_no
	`, repository.orgID, id)
	if err != nil {
		return accounting.InflationWorkpaper{}, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var line accounting.InflationCalculationLine
		var origin, closing, coefficient, historical, restated, adjustment string
		if err := rows.Scan(
			&line.AccountID,
			&line.AccountCode,
			&line.AccountName,
			&line.OriginDate,
			&origin,
			&closing,
			&coefficient,
			&historical,
			&restated,
			&adjustment,
			&line.NormalBalance,
		); err != nil {
			return accounting.InflationWorkpaper{}, mapError(err)
		}
		line.OriginIndex, err = accounting.ParseDecimal(origin)
		if err != nil {
			return accounting.InflationWorkpaper{}, err
		}
		line.ClosingIndex, err = accounting.ParseDecimal(closing)
		if err != nil {
			return accounting.InflationWorkpaper{}, err
		}
		line.Coefficient, err = accounting.ParseDecimal(coefficient)
		if err != nil {
			return accounting.InflationWorkpaper{}, err
		}
		line.Historical, err = accounting.ParseAmount(historical)
		if err != nil {
			return accounting.InflationWorkpaper{}, err
		}
		line.Restated, err = accounting.ParseAmount(restated)
		if err != nil {
			return accounting.InflationWorkpaper{}, err
		}
		line.Adjustment, err = accounting.ParseAmount(adjustment)
		if err != nil {
			return accounting.InflationWorkpaper{}, err
		}
		workpaper.Lines = append(workpaper.Lines, line)
	}
	if err := rows.Err(); err != nil {
		return accounting.InflationWorkpaper{}, mapError(err)
	}
	return workpaper, nil
}

func reconciliationStatusToDatabase(status accounting.ReconciliationStatus) string {
	switch status {
	case accounting.ReconciliationOpen:
		return "draft"
	case accounting.ReconciliationClosed:
		return "closed"
	default:
		return ""
	}
}

func reconciliationStatusFromDatabase(status string) accounting.ReconciliationStatus {
	if status == "closed" {
		return accounting.ReconciliationClosed
	}
	return accounting.ReconciliationOpen
}
