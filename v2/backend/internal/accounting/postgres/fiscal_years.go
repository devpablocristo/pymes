package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

const fiscalYearSummaryCTE = `
	WITH summaries AS (
		SELECT
			fiscal_year.id,
			fiscal_year.idempotency_key,
			fiscal_year.code,
			fiscal_year.start_date,
			fiscal_year.end_date,
			fiscal_year.is_legacy,
			CASE
				WHEN count(period.id) FILTER (
					WHERE period.status = 'open'
				) > 0 THEN 'open'
				WHEN count(period.id) > 0
				AND count(period.id) FILTER (
					WHERE period.status = 'locked'
				) = count(period.id)
				AND fiscal_year.annual_close_status IN (
					'posted',
					'not_required'
				) THEN 'closed'
				ELSE 'closing'
			END AS derived_state,
			fiscal_year.version,
			count(period.id) FILTER (
				WHERE period.status = 'open'
			)::integer AS open_count,
			count(period.id) FILTER (
				WHERE period.status = 'soft_closed'
			)::integer AS soft_closed_count,
			count(period.id) FILTER (
				WHERE period.status = 'locked'
			)::integer AS locked_count,
			fiscal_year.annual_close_status,
			fiscal_year.annual_close_draft_id,
			fiscal_year.annual_close_entry_id,
			fiscal_year.annual_close_reversal_entry_id,
			fiscal_year.created_by,
			fiscal_year.created_at,
			fiscal_year.updated_at
		  FROM accounting.fiscal_years AS fiscal_year
		  LEFT JOIN accounting.periods AS period
		    ON period.org_id = fiscal_year.org_id
		   AND period.fiscal_year_id = fiscal_year.id
		 WHERE fiscal_year.org_id = $1
		 GROUP BY fiscal_year.org_id, fiscal_year.id
	)
`

const fiscalYearSummaryColumns = `
	id,
	idempotency_key,
	code,
	start_date,
	end_date,
	is_legacy,
	derived_state,
	version,
	open_count,
	soft_closed_count,
	locked_count,
	annual_close_status,
	annual_close_draft_id,
	annual_close_entry_id,
	annual_close_reversal_entry_id,
	created_by,
	created_at,
	updated_at
`

func (repository *Repository) FiscalYearStartMonth(
	ctx context.Context,
) (time.Month, error) {
	var month int
	if err := repository.tx.QueryRow(ctx, `
		SELECT fiscal_year_start_month
		  FROM accounting.organization_settings
		 WHERE org_id = $1
	`, repository.orgID).Scan(&month); err != nil {
		return 0, mapError(err)
	}
	if month < 1 || month > 12 {
		return 0, accounting.ErrInvalidArgument
	}
	return time.Month(month), nil
}

func (repository *Repository) ListFiscalYears(
	ctx context.Context,
	filter accounting.FiscalYearFilter,
) (accounting.FiscalYearPage, error) {
	afterDate, afterID, err := decodeFiscalYearCursor(filter.After)
	if err != nil {
		return accounting.FiscalYearPage{}, err
	}
	const searchPredicate = `
		(
			$2 = ''
			OR code ILIKE '%' || $2 || '%'
			OR start_date::text ILIKE '%' || $2 || '%'
			OR end_date::text ILIKE '%' || $2 || '%'
		)
	`
	var total int
	if err := repository.tx.QueryRow(
		ctx,
		fiscalYearSummaryCTE+`
			SELECT count(*)::integer
			  FROM summaries
			 WHERE `+searchPredicate+`
			   AND ($3 = '' OR derived_state = $3)
		`,
		repository.orgID,
		filter.Query,
		filter.State,
	).Scan(&total); err != nil {
		return accounting.FiscalYearPage{}, mapError(err)
	}
	rows, err := repository.tx.Query(ctx, fiscalYearSummaryCTE+`
		SELECT `+fiscalYearSummaryColumns+`
		  FROM summaries
		 WHERE `+searchPredicate+`
		   AND ($3 = '' OR derived_state = $3)
		   AND (
				$4::date IS NULL
				OR (start_date, id) < ($4::date, $5::uuid)
		   )
		 ORDER BY start_date DESC, id DESC
		 LIMIT $6
	`,
		repository.orgID,
		filter.Query,
		filter.State,
		afterDate,
		afterID,
		filter.Limit+1,
	)
	if err != nil {
		return accounting.FiscalYearPage{}, mapError(err)
	}
	defer rows.Close()
	page := accounting.FiscalYearPage{
		Items: make([]accounting.FiscalYearSummary, 0, filter.Limit),
		Total: total,
	}
	for rows.Next() {
		year, scanErr := scanFiscalYearSummary(rows, nil)
		if scanErr != nil {
			return accounting.FiscalYearPage{}, scanErr
		}
		page.Items = append(page.Items, year)
	}
	if err := rows.Err(); err != nil {
		return accounting.FiscalYearPage{}, mapError(err)
	}
	if len(page.Items) > filter.Limit {
		lastVisible := page.Items[filter.Limit-1]
		page.NextCursor = encodeFiscalYearCursor(lastVisible)
		page.Items = page.Items[:filter.Limit]
	}
	return page, nil
}

func (repository *Repository) GetFiscalYear(
	ctx context.Context,
	id uuid.UUID,
	forUpdate bool,
) (accounting.FiscalYearSummary, error) {
	if forUpdate {
		if _, err := repository.tx.Exec(ctx, `
			SELECT id
			  FROM accounting.fiscal_years
			 WHERE org_id = $1
			   AND id = $2
			 FOR UPDATE
		`, repository.orgID, id); err != nil {
			return accounting.FiscalYearSummary{}, mapError(err)
		}
	}
	year, err := scanFiscalYearSummary(
		repository.tx.QueryRow(
			ctx,
			fiscalYearSummaryCTE+`
				SELECT `+fiscalYearSummaryColumns+`
				  FROM summaries
				 WHERE id = $2
			`,
			repository.orgID,
			id,
		),
		nil,
	)
	if err != nil {
		return accounting.FiscalYearSummary{}, err
	}
	return year, nil
}

func (repository *Repository) CreateFiscalYear(
	ctx context.Context,
	year accounting.FiscalYearSummary,
	periods []accounting.Period,
) (accounting.FiscalYearSummary, error) {
	var persistedID uuid.UUID
	if err := repository.tx.QueryRow(ctx, `
		INSERT INTO accounting.fiscal_years (
			org_id,
			id,
			idempotency_key,
			code,
			start_date,
			end_date,
			is_legacy,
			created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (org_id, idempotency_key) DO UPDATE
		   SET idempotency_key = excluded.idempotency_key
		RETURNING id
	`,
		repository.orgID,
		year.ID,
		year.IdempotencyKey,
		year.Code,
		year.StartDate,
		year.EndDate,
		year.IsLegacy,
		year.CreatedBy,
	).Scan(&persistedID); err != nil {
		return accounting.FiscalYearSummary{}, mapError(err)
	}
	if persistedID != year.ID {
		existing, err := repository.GetFiscalYear(ctx, persistedID, false)
		if err != nil {
			return accounting.FiscalYearSummary{}, err
		}
		if !sameDate(existing.StartDate, year.StartDate) ||
			!sameDate(existing.EndDate, year.EndDate) {
			return accounting.FiscalYearSummary{}, accounting.ErrIdempotencyConflict
		}
		return existing, nil
	}

	batch := &pgx.Batch{}
	for _, period := range periods {
		batch.Queue(`
			INSERT INTO accounting.periods (
				org_id,
				id,
				code,
				start_date,
				end_date,
				status,
				fiscal_year_id,
				period_no,
				is_legacy
			)
			VALUES ($1, $2, $3, $4, $5, 'open', $6, $7, $8)
		`,
			repository.orgID,
			period.ID,
			period.Name,
			period.StartDate,
			period.EndDate,
			year.ID,
			period.SequenceNo,
			period.IsLegacy,
		)
	}
	results := repository.tx.SendBatch(ctx, batch)
	for range periods {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return accounting.FiscalYearSummary{}, mapError(err)
		}
	}
	if err := results.Close(); err != nil {
		return accounting.FiscalYearSummary{}, mapError(err)
	}
	return repository.GetFiscalYear(ctx, year.ID, false)
}

func (repository *Repository) ListFiscalYearPeriods(
	ctx context.Context,
	fiscalYearID uuid.UUID,
	forUpdate bool,
) ([]accounting.Period, error) {
	query := `
		SELECT
			` + periodColumns + `,
			coalesce((
				SELECT (check_result.details->>'count')::integer
				  FROM accounting.period_close_checks AS check_result
				 WHERE check_result.org_id = period.org_id
				   AND check_result.period_id = period.id
				   AND check_result.check_key = 'unposted_documents'
			), 0),
			coalesce((
				SELECT (check_result.details->>'count')::integer
				  FROM accounting.period_close_checks AS check_result
				 WHERE check_result.org_id = period.org_id
				   AND check_result.period_id = period.id
				   AND check_result.check_key = 'fiscal_pending'
			), 0),
			coalesce((
				SELECT (check_result.details->>'count')::integer
				  FROM accounting.period_close_checks AS check_result
				 WHERE check_result.org_id = period.org_id
				   AND check_result.period_id = period.id
				   AND check_result.check_key = 'posting_errors'
			), 0),
			coalesce((
				SELECT (check_result.details->>'count')::integer
				  FROM accounting.period_close_checks AS check_result
				 WHERE check_result.org_id = period.org_id
				   AND check_result.period_id = period.id
				   AND check_result.check_key = 'account_mappings'
			), 0),
			coalesce((
				SELECT (check_result.details->>'count')::integer
				  FROM accounting.period_close_checks AS check_result
				 WHERE check_result.org_id = period.org_id
				   AND check_result.period_id = period.id
				   AND check_result.check_key = 'exchange_rates'
			), 0),
			coalesce((
				SELECT (check_result.details->>'count')::integer
				  FROM accounting.period_close_checks AS check_result
				 WHERE check_result.org_id = period.org_id
				   AND check_result.period_id = period.id
				   AND check_result.check_key = 'unreconciled_accounts'
			), 0),
			coalesce((
				SELECT (check_result.details->>'count')::integer
				  FROM accounting.period_close_checks AS check_result
				 WHERE check_result.org_id = period.org_id
				   AND check_result.period_id = period.id
				   AND check_result.check_key = 'pending_drafts'
			), 0),
			(
				SELECT max(check_result.checked_at)
				  FROM accounting.period_close_checks AS check_result
				 WHERE check_result.org_id = period.org_id
				   AND check_result.period_id = period.id
			)
		  FROM accounting.periods AS period
		 WHERE period.org_id = $1
		   AND period.fiscal_year_id = $2
		 ORDER BY period_no NULLS LAST, start_date, id
	`
	if forUpdate {
		query += " FOR UPDATE"
	}
	rows, err := repository.tx.Query(
		ctx,
		query,
		repository.orgID,
		fiscalYearID,
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	periods := make([]accounting.Period, 0, 12)
	for rows.Next() {
		period, scanErr := scanPeriodWithChecklist(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		periods = append(periods, period)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return periods, nil
}

func scanPeriodWithChecklist(row scanner) (accounting.Period, error) {
	var period accounting.Period
	if err := row.Scan(
		&period.ID,
		&period.Name,
		&period.StartDate,
		&period.EndDate,
		&period.Status,
		&period.Version,
		&period.FiscalYearID,
		&period.SequenceNo,
		&period.IsLegacy,
		&period.StatusChangedBy,
		&period.TransitionReason,
		&period.Checklist.UnpostedDocuments,
		&period.Checklist.PendingFiscalDocuments,
		&period.Checklist.PostingErrors,
		&period.Checklist.MissingMappings,
		&period.Checklist.MissingExchangeRates,
		&period.Checklist.UnclosedReconciliations,
		&period.Checklist.PendingDrafts,
		&period.Checklist.EvaluatedAt,
	); err != nil {
		return accounting.Period{}, mapError(err)
	}
	return period, nil
}

func (repository *Repository) ListFiscalYearEvents(
	ctx context.Context,
	fiscalYearID uuid.UUID,
	limit int,
) ([]accounting.FiscalYearEvent, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			id,
			fiscal_year_id,
			event_type,
			coalesce(from_status, ''),
			to_status,
			from_version,
			to_version,
			actor,
			coalesce(reason, ''),
			metadata,
			occurred_at
		  FROM accounting.fiscal_year_events
		 WHERE org_id = $1
		   AND fiscal_year_id = $2
		 ORDER BY occurred_at DESC, id DESC
		 LIMIT $3
	`, repository.orgID, fiscalYearID, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	events := make([]accounting.FiscalYearEvent, 0)
	for rows.Next() {
		var (
			event       accounting.FiscalYearEvent
			metadataRaw []byte
		)
		if err := rows.Scan(
			&event.ID,
			&event.FiscalYearID,
			&event.EventType,
			&event.FromStatus,
			&event.ToStatus,
			&event.FromVersion,
			&event.ToVersion,
			&event.ActorID,
			&event.Reason,
			&metadataRaw,
			&event.OccurredAt,
		); err != nil {
			return nil, mapError(err)
		}
		if len(metadataRaw) != 0 {
			if err := json.Unmarshal(metadataRaw, &event.Metadata); err != nil {
				return nil, fmt.Errorf(
					"accounting postgres: decode fiscal year event: %w",
					err,
				)
			}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return events, nil
}

func (repository *Repository) ListPeriodEvents(
	ctx context.Context,
	periodID uuid.UUID,
	limit int,
) ([]accounting.PeriodAudit, error) {
	rows, err := repository.tx.Query(ctx, `
		SELECT
			id,
			period_id,
			from_status,
			to_status,
			from_version,
			to_version,
			coalesce(reason, ''),
			actor,
			occurred_at
		  FROM accounting.period_events
		 WHERE org_id = $1
		   AND period_id = $2
		 ORDER BY occurred_at DESC, id DESC
		 LIMIT $3
	`, repository.orgID, periodID, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	events := make([]accounting.PeriodAudit, 0)
	for rows.Next() {
		var event accounting.PeriodAudit
		if err := rows.Scan(
			&event.ID,
			&event.PeriodID,
			&event.FromStatus,
			&event.ToStatus,
			&event.FromVersion,
			&event.ToVersion,
			&event.Reason,
			&event.ActorID,
			&event.OccurredAt,
		); err != nil {
			return nil, mapError(err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return events, nil
}

func (repository *Repository) UpdateFiscalYearAnnualClose(
	ctx context.Context,
	update accounting.FiscalYearAnnualCloseUpdate,
) (accounting.FiscalYearSummary, error) {
	if _, err := repository.tx.Exec(ctx, `
		SELECT set_config(
			'app.accounting_idempotency_key',
			$1,
			true
		)
	`, strings.TrimSpace(update.IdempotencyKey)); err != nil {
		return accounting.FiscalYearSummary{}, mapError(err)
	}
	commandTag, err := repository.tx.Exec(ctx, `
		UPDATE accounting.fiscal_years
		   SET annual_close_status = $3,
		       annual_close_draft_id = CASE
					WHEN $3 IN ('not_ready', 'ready', 'not_required')
					THEN NULL
					ELSE coalesce($4, annual_close_draft_id)
		       END,
		       annual_close_entry_id = CASE
					WHEN $3 IN ('not_ready', 'ready', 'draft', 'not_required')
					THEN NULL
					ELSE coalesce($5, annual_close_entry_id)
		       END,
		       annual_close_reversal_entry_id = CASE
					WHEN $3 = 'reversed'
					THEN coalesce($6, annual_close_reversal_entry_id)
					ELSE NULL
		       END,
		       version = version + 1,
		       annual_close_changed_by = $7,
		       transition_reason = NULLIF($8, ''),
		       updated_at = now()
		 WHERE org_id = $1
		   AND id = $2
		   AND version = $9
	`,
		repository.orgID,
		update.FiscalYearID,
		update.Status,
		update.DraftID,
		update.EntryID,
		update.ReversalEntryID,
		repository.actor,
		strings.TrimSpace(update.Reason),
		update.ExpectedVersion,
	)
	if err != nil {
		return accounting.FiscalYearSummary{}, mapError(err)
	}
	if commandTag.RowsAffected() != 1 {
		return accounting.FiscalYearSummary{}, accounting.ErrVersionConflict
	}
	return repository.GetFiscalYear(ctx, update.FiscalYearID, false)
}

func (repository *Repository) FiscalYearTransitionWasApplied(
	ctx context.Context,
	fiscalYearID uuid.UUID,
	status accounting.AnnualCloseStatus,
	idempotencyKey string,
) (bool, error) {
	var applied bool
	if err := repository.tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			  FROM accounting.fiscal_year_events AS event
			 WHERE event.org_id = $1
			   AND event.fiscal_year_id = $2
			   AND event.event_type = 'annual_close_transition'
			   AND event.to_status = $3
			   AND event.metadata->>'idempotency_key' = $4
		)
	`,
		repository.orgID,
		fiscalYearID,
		status,
		strings.TrimSpace(idempotencyKey),
	).Scan(&applied); err != nil {
		return false, mapError(err)
	}
	return applied, nil
}

func (repository *Repository) LatestClosedFiscalYear(
	ctx context.Context,
	forUpdate bool,
) (accounting.FiscalYearSummary, error) {
	query := `
		SELECT fiscal_year.id
		  FROM accounting.fiscal_years AS fiscal_year
		 WHERE fiscal_year.org_id = $1
		   AND fiscal_year.annual_close_status IN ('posted', 'not_required')
		   AND EXISTS (
				SELECT 1
				  FROM accounting.periods AS period
				 WHERE period.org_id = fiscal_year.org_id
				   AND period.fiscal_year_id = fiscal_year.id
		   )
		   AND NOT EXISTS (
				SELECT 1
				  FROM accounting.periods AS period
				 WHERE period.org_id = fiscal_year.org_id
				   AND period.fiscal_year_id = fiscal_year.id
				   AND period.status <> 'locked'
		   )
		 ORDER BY fiscal_year.start_date DESC, fiscal_year.id DESC
		 LIMIT 1
	`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var id uuid.UUID
	if err := repository.tx.QueryRow(
		ctx,
		query,
		repository.orgID,
	).Scan(&id); err != nil {
		return accounting.FiscalYearSummary{}, mapError(err)
	}
	return repository.GetFiscalYear(ctx, id, false)
}

func scanFiscalYearSummary(
	row scanner,
	total *int,
) (accounting.FiscalYearSummary, error) {
	var year accounting.FiscalYearSummary
	destinations := []any{
		&year.ID,
		&year.IdempotencyKey,
		&year.Code,
		&year.StartDate,
		&year.EndDate,
		&year.IsLegacy,
		&year.State,
		&year.Version,
		&year.PeriodCounts.Open,
		&year.PeriodCounts.SoftClosed,
		&year.PeriodCounts.Locked,
		&year.AnnualCloseStatus,
		&year.AnnualCloseDraftID,
		&year.AnnualCloseEntryID,
		&year.AnnualCloseReversalEntryID,
		&year.CreatedBy,
		&year.CreatedAt,
		&year.UpdatedAt,
	}
	if total != nil {
		destinations = append(destinations, total)
	}
	if err := row.Scan(destinations...); err != nil {
		return accounting.FiscalYearSummary{}, mapError(err)
	}
	year.Capabilities = accounting.DeriveFiscalYearCapabilities(
		year.State,
		year.PeriodCounts,
		year.AnnualCloseStatus,
	)
	return year, nil
}

func encodeFiscalYearCursor(year accounting.FiscalYearSummary) string {
	raw := year.StartDate.Format("2006-01-02") + "|" + year.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeFiscalYearCursor(value string) (*time.Time, *uuid.UUID, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, nil, accounting.ErrInvalidArgument
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 2 {
		return nil, nil, accounting.ErrInvalidArgument
	}
	date, err := time.Parse("2006-01-02", parts[0])
	if err != nil {
		return nil, nil, accounting.ErrInvalidArgument
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, nil, accounting.ErrInvalidArgument
	}
	return &date, &id, nil
}

func sameDate(left, right time.Time) bool {
	return left.Format("2006-01-02") == right.Format("2006-01-02")
}
