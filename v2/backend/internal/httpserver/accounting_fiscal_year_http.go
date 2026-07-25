package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *IAMAPI) listAccountingFiscalYears(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListAccountingFiscalYearsParams,
) {
	limit := accountingAPILimit(params.Limit)
	filter := accounting.FiscalYearFilter{Limit: limit}
	if params.Query != nil {
		filter.Query = strings.TrimSpace(*params.Query)
	}
	if params.State != nil {
		filter.State = accounting.FiscalYearState(*params.State)
	}
	if params.Cursor != nil {
		filter.After = string(*params.Cursor)
	}

	response := api.AccountingFiscalYearList{
		Items: make([]api.AccountingFiscalYearSummary, 0),
		Page:  api.PageInfo{Total: 0},
	}
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingView,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			_ pgx.Tx,
		) error {
			page, err := service.ListFiscalYears(ctx, scope, filter)
			if err != nil {
				return err
			}
			response.Items = make(
				[]api.AccountingFiscalYearSummary,
				0,
				len(page.Items),
			)
			for _, fiscalYear := range page.Items {
				response.Items = append(
					response.Items,
					apiFiscalYearSummary(fiscalYear),
				)
			}
			response.Page.Total = page.Total
			if page.NextCursor != "" {
				response.Page.NextCursor = &page.NextCursor
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) createAccountingFiscalYear(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateAccountingFiscalYearParams,
) {
	var input api.AccountingFiscalYearInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	idempotencyKey, valid := validateIdempotencyKey(w, params.IdempotencyKey)
	if !valid {
		return
	}

	var response api.AccountingFiscalYearDetail
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			_ pgx.Tx,
		) error {
			detail, err := service.CreateFiscalYear(
				ctx,
				scope,
				accounting.CreateFiscalYearCommand{
					StartYear:      input.StartYear,
					IdempotencyKey: idempotencyKey,
				},
			)
			if err != nil {
				return err
			}
			response = apiFiscalYearDetail(detail)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) getAccountingFiscalYear(
	w http.ResponseWriter,
	r *http.Request,
	fiscalYearID api.FiscalYearID,
) {
	var response api.AccountingFiscalYearDetail
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingView,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			detail, err := service.GetFiscalYear(
				ctx,
				scope,
				uuid.UUID(fiscalYearID),
			)
			if err != nil {
				return err
			}
			response = apiFiscalYearDetail(detail)
			return loadFiscalYearPeriodEvents(ctx, tx, &response)
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func loadFiscalYearPeriodEvents(
	ctx context.Context,
	tx pgx.Tx,
	detail *api.AccountingFiscalYearDetail,
) error {
	periodIndex := make(map[uuid.UUID]int, len(detail.Periods))
	for index := range detail.Periods {
		periodIndex[uuid.UUID(detail.Periods[index].Id)] = index
	}
	rows, err := tx.Query(ctx, `
		SELECT
			event.id,
			event.period_id,
			event.from_status,
			event.to_status,
			event.from_version,
			event.to_version,
			event.actor,
			coalesce(event.reason, ''),
			event.occurred_at
		  FROM accounting.period_events AS event
		  JOIN accounting.periods AS period
		    ON period.org_id = event.org_id
		   AND period.id = event.period_id
		 WHERE period.fiscal_year_id = $1
		 ORDER BY event.occurred_at DESC, event.id DESC
		 LIMIT 240
	`, detail.Id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var event accounting.PeriodAudit
		if err := rows.Scan(
			&event.ID,
			&event.PeriodID,
			&event.FromStatus,
			&event.ToStatus,
			&event.FromVersion,
			&event.ToVersion,
			&event.ActorID,
			&event.Reason,
			&event.OccurredAt,
		); err != nil {
			return err
		}
		index, ok := periodIndex[event.PeriodID]
		if !ok {
			continue
		}
		detail.Periods[index].RecentEvents = append(
			detail.Periods[index].RecentEvents,
			apiPeriodEvent(event),
		)
	}
	return rows.Err()
}

func (h *IAMAPI) createFiscalYearAnnualClosingDraft(
	w http.ResponseWriter,
	r *http.Request,
	fiscalYearID api.FiscalYearID,
	params api.CreateFiscalYearAnnualClosingDraftParams,
) {
	var input api.AnnualClosingDraftInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	idempotencyKey, valid := validateIdempotencyKey(w, params.IdempotencyKey)
	if !valid {
		return
	}

	var response api.AccountingAnnualCloseResult
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			currency, err := loadFunctionalCurrency(ctx, tx)
			if err != nil {
				return err
			}
			result, err := service.PrepareFiscalYearAnnualClose(
				ctx,
				scope,
				accounting.FiscalYearAnnualClosingCommand{
					FiscalYearID:       uuid.UUID(fiscalYearID),
					ExpectedVersion:    input.Version,
					FunctionalCurrency: currency,
					IdempotencyKey:     idempotencyKey,
				},
			)
			if err != nil {
				return err
			}
			response.FiscalYear = apiFiscalYearSummary(result.FiscalYear)
			if result.Draft != nil {
				draft := apiDraft(*result.Draft)
				response.Draft = &draft
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *IAMAPI) reopenAccountingFiscalYear(
	w http.ResponseWriter,
	r *http.Request,
	fiscalYearID api.FiscalYearID,
	params api.ReopenAccountingFiscalYearParams,
) {
	var input api.AccountingFiscalYearReopenInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	idempotencyKey, valid := validateIdempotencyKey(w, params.IdempotencyKey)
	if !valid {
		return
	}

	var response api.AccountingFiscalYearDetail
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			_ pgx.Tx,
		) error {
			if _, err := service.ReopenFiscalYear(
				ctx,
				scope,
				accounting.ReopenFiscalYearCommand{
					FiscalYearID:    uuid.UUID(fiscalYearID),
					ExpectedVersion: input.Version,
					Reason:          input.Reason,
					IdempotencyKey:  idempotencyKey,
				},
			); err != nil {
				return err
			}
			detail, err := service.GetFiscalYear(
				ctx,
				scope,
				uuid.UUID(fiscalYearID),
			)
			if err != nil {
				return err
			}
			response = apiFiscalYearDetail(detail)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) getAccountingPeriod(
	w http.ResponseWriter,
	r *http.Request,
	periodID api.PeriodID,
) {
	var response api.AccountingPeriodDetail
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingView,
		func(
			ctx context.Context,
			service *accounting.Service,
			scope accounting.Scope,
			_ pgx.Tx,
		) error {
			detail, err := service.GetPeriodDetail(
				ctx,
				scope,
				uuid.UUID(periodID),
			)
			if err != nil {
				return err
			}
			response = apiPeriodDetail(detail)
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) listAccountingPeriodEvents(
	w http.ResponseWriter,
	r *http.Request,
	periodID api.PeriodID,
	params api.ListAccountingPeriodEventsParams,
) {
	limit := accountingAPILimit(params.Limit)
	var rawCursor *string
	if params.Cursor != nil {
		value := string(*params.Cursor)
		rawCursor = &value
	}
	cursor, err := decodeKeysetCursor(rawCursor)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid cursor")
		return
	}

	response := api.AccountingPeriodEventList{
		Items: make([]api.AccountingPeriodEvent, 0),
		Page:  api.PageInfo{Total: 0},
	}
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingView,
		func(
			ctx context.Context,
			_ *accounting.Service,
			_ accounting.Scope,
			tx pgx.Tx,
		) error {
			var exists bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1
					  FROM accounting.periods
					 WHERE id = $1
				)
			`, periodID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return accounting.ErrNotFound
			}
			if err := tx.QueryRow(ctx, `
				SELECT count(*)
				  FROM accounting.period_events
				 WHERE period_id = $1
			`, periodID).Scan(&response.Page.Total); err != nil {
				return err
			}

			var cursorTime *time.Time
			var cursorID *uuid.UUID
			if cursor.Sort != "" {
				parsedTime, parseErr := time.Parse(time.RFC3339Nano, cursor.Sort)
				if parseErr != nil {
					return fmt.Errorf("%w: invalid period event cursor", accounting.ErrInvalidArgument)
				}
				parsedID, parseErr := uuid.Parse(cursor.ID)
				if parseErr != nil {
					return fmt.Errorf("%w: invalid period event cursor", accounting.ErrInvalidArgument)
				}
				cursorTime = &parsedTime
				cursorID = &parsedID
			}
			rows, err := tx.Query(ctx, `
				SELECT
					event.id,
					event.period_id,
					event.from_status,
					event.to_status,
					event.from_version,
					event.to_version,
					event.actor,
					coalesce(event.reason, ''),
					event.occurred_at
				  FROM accounting.period_events AS event
				 WHERE event.period_id = $1
				   AND (
						$2::timestamptz IS NULL
						OR (event.occurred_at, event.id)
						   < ($2::timestamptz, $3::uuid)
				   )
				 ORDER BY event.occurred_at DESC, event.id DESC
				 LIMIT $4
			`, periodID, cursorTime, cursorID, limit+1)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var event accounting.PeriodAudit
				if err := rows.Scan(
					&event.ID,
					&event.PeriodID,
					&event.FromStatus,
					&event.ToStatus,
					&event.FromVersion,
					&event.ToVersion,
					&event.ActorID,
					&event.Reason,
					&event.OccurredAt,
				); err != nil {
					return err
				}
				response.Items = append(response.Items, apiPeriodEvent(event))
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(response.Items) > limit {
				response.Items = response.Items[:limit]
				last := response.Items[len(response.Items)-1]
				response.Page.NextCursor = encodeKeysetCursor(
					last.OccurredAt.Format(time.RFC3339Nano),
					uuid.UUID(last.Id).String(),
				)
			}
			return nil
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *IAMAPI) updateAccountingFiscalCalendar(
	w http.ResponseWriter,
	r *http.Request,
	params api.UpdateAccountingFiscalCalendarParams,
) {
	var input api.AccountingFiscalCalendarInput
	if !decodeBusinessBody(w, r, &input) {
		return
	}
	idempotencyKey, valid := validateIdempotencyKey(w, params.IdempotencyKey)
	if !valid {
		return
	}
	if input.FiscalYearStartMonth < 1 || input.FiscalYearStartMonth > 12 ||
		input.Version <= 0 {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Invalid fiscal calendar")
		return
	}

	var response api.AccountingSettings
	if !h.withAccountingService(
		w,
		r,
		productiam.PermissionAccountingManage,
		func(
			ctx context.Context,
			_ *accounting.Service,
			scope accounting.Scope,
			tx pgx.Tx,
		) error {
			if _, err := tx.Exec(ctx, `
				SELECT accounting.replace_empty_fiscal_calendar(
					$1::uuid,
					$2::smallint,
					$3::bigint,
					$4::text,
					$5::text
				)
			`,
				scope.OrganizationID,
				input.FiscalYearStartMonth,
				input.Version,
				scope.ActorID,
				idempotencyKey,
			); err != nil {
				var postgresError *pgconn.PgError
				if errors.As(err, &postgresError) {
					switch {
					case postgresError.Code == "40001":
						return accounting.ErrVersionConflict
					case postgresError.Code == "23505" &&
						strings.Contains(
							postgresError.ConstraintName,
							"idempotency",
						):
						return accounting.ErrIdempotencyConflict
					case postgresError.Code == "55000" &&
						strings.Contains(
							postgresError.ConstraintName,
							"calendar",
						):
						return accounting.ErrFiscalYearNotReady
					case postgresError.Code == "23514":
						return accounting.ErrInvalidArgument
					}
				}
				return err
			}
			return tx.QueryRow(ctx, `
				SELECT
					setting.country_code,
					setting.functional_currency,
					setting.timezone,
					setting.fiscal_year_start_month,
					setting.version
				  FROM accounting.organization_settings AS setting
				 LIMIT 1
			`).Scan(
				&response.CountryCode,
				&response.FunctionalCurrency,
				&response.Timezone,
				&response.FiscalYearStartMonth,
				&response.Version,
			)
		},
	) {
		return
	}
	response.CanChangeFiscalYearStart = true
	writeJSON(w, http.StatusOK, response)
}

func apiFiscalYearSummary(
	fiscalYear accounting.FiscalYearSummary,
) api.AccountingFiscalYearSummary {
	response := api.AccountingFiscalYearSummary{
		AnnualCloseStatus: api.AccountingAnnualCloseStatus(
			fiscalYear.AnnualCloseStatus,
		),
		Code:     fiscalYear.Code,
		EndDate:  openapi_types.Date{Time: fiscalYear.EndDate},
		Id:       openapi_types.UUID(fiscalYear.ID),
		IsLegacy: fiscalYear.IsLegacy,
		PeriodCounts: api.AccountingFiscalYearPeriodCounts{
			Locked:     fiscalYear.PeriodCounts.Locked,
			Open:       fiscalYear.PeriodCounts.Open,
			SoftClosed: fiscalYear.PeriodCounts.SoftClosed,
		},
		StartDate: openapi_types.Date{Time: fiscalYear.StartDate},
		State:     api.AccountingFiscalYearState(fiscalYear.State),
		Version:   fiscalYear.Version,
	}
	if fiscalYear.AnnualCloseDraftID != nil {
		value := openapi_types.UUID(*fiscalYear.AnnualCloseDraftID)
		response.AnnualClosingDraftId = &value
	}
	if fiscalYear.AnnualCloseEntryID != nil {
		value := openapi_types.UUID(*fiscalYear.AnnualCloseEntryID)
		response.AnnualClosingEntryId = &value
	}
	return response
}

func apiFiscalYearDetail(
	detail accounting.FiscalYearDetail,
) api.AccountingFiscalYearDetail {
	summary := apiFiscalYearSummary(detail.FiscalYearSummary)
	response := api.AccountingFiscalYearDetail{
		AnnualCloseStatus:    summary.AnnualCloseStatus,
		AnnualClosingDraftId: summary.AnnualClosingDraftId,
		AnnualClosingEntryId: summary.AnnualClosingEntryId,
		Capabilities: api.AccountingFiscalYearCapabilities{
			Blockers: append(
				[]string(nil),
				detail.Capabilities.BlockingReasons...,
			),
			CanPrepareAnnualClose: detail.Capabilities.CanPrepareAnnualClose,
			CanReopen:             detail.Capabilities.CanReopen,
		},
		Code:         summary.Code,
		EndDate:      summary.EndDate,
		Id:           summary.Id,
		IsLegacy:     summary.IsLegacy,
		PeriodCounts: summary.PeriodCounts,
		Periods:      make([]api.AccountingPeriodDetail, 0, len(detail.Periods)),
		RecentEvents: make(
			[]api.AccountingFiscalYearEvent,
			0,
			len(detail.RecentEvents),
		),
		StartDate: summary.StartDate,
		State:     summary.State,
		Version:   summary.Version,
	}
	for _, period := range detail.Periods {
		response.Periods = append(
			response.Periods,
			apiPeriodDetail(accounting.PeriodDetail{Period: period}),
		)
	}
	for _, event := range detail.RecentEvents {
		mapped := api.AccountingFiscalYearEvent{
			Actor:      event.ActorID,
			EventType:  event.EventType,
			Id:         openapi_types.UUID(event.ID),
			OccurredAt: event.OccurredAt,
			ToVersion:  event.ToVersion,
		}
		if event.FromVersion != nil {
			mapped.FromVersion = event.FromVersion
		}
		if event.FromStatus != "" {
			mapped.FromStatus = &event.FromStatus
		}
		if event.ToStatus != "" {
			mapped.ToStatus = &event.ToStatus
		}
		if event.Reason != "" {
			mapped.Reason = &event.Reason
		}
		response.RecentEvents = append(response.RecentEvents, mapped)
	}
	return response
}

func apiPeriodDetail(detail accounting.PeriodDetail) api.AccountingPeriodDetail {
	period := detail.Period
	fiscalYearID := uuid.Nil
	if period.FiscalYearID != nil {
		fiscalYearID = *period.FiscalYearID
	}
	response := api.AccountingPeriodDetail{
		Capabilities:   apiPeriodCapabilities(period.Capabilities),
		CloseReadiness: apiCloseReadiness(period.Checklist),
		Code:           period.Name,
		EndDate:        openapi_types.Date{Time: period.EndDate},
		FiscalYearId:   openapi_types.UUID(fiscalYearID),
		Id:             openapi_types.UUID(period.ID),
		IsLegacy:       period.IsLegacy,
		RecentEvents: make(
			[]api.AccountingPeriodEvent,
			0,
			len(detail.RecentEvents),
		),
		Sequence:  period.SequenceNo,
		StartDate: openapi_types.Date{Time: period.StartDate},
		State:     api.AccountingPeriodState(period.Status),
		Version:   period.Version,
	}
	for _, event := range detail.RecentEvents {
		response.RecentEvents = append(
			response.RecentEvents,
			apiPeriodEvent(event),
		)
	}
	return response
}

func apiPeriodCapabilities(
	capabilities accounting.PeriodCapabilities,
) api.AccountingPeriodCapabilities {
	response := api.AccountingPeriodCapabilities{
		Blockers:     append([]string(nil), capabilities.Blockers...),
		CanLock:      capabilities.CanLock,
		CanSoftClose: capabilities.CanSoftClose,
	}
	for _, target := range capabilities.Targets {
		switch target {
		case string(accounting.PeriodSoftClosed):
			response.CanReopenToSoftClosed = capabilities.CanReopen
		case string(accounting.PeriodOpen):
			response.CanReopenToOpen = capabilities.CanReopen
		}
	}
	return response
}

func apiCloseReadiness(
	checklist accounting.CloseChecklist,
) api.AccountingCloseReadiness {
	checks := []struct {
		code   string
		label  string
		count  int
		target string
	}{
		{"unposted_documents", "Documentos sin asiento", checklist.UnpostedDocuments, "/fiscal/vouchers"},
		{"fiscal_pending", "Comprobantes fiscales pendientes", checklist.PendingFiscalDocuments, "/fiscal/vouchers"},
		{"posting_errors", "Errores de posteo", checklist.PostingErrors, "/accounting/journal"},
		{"account_mappings", "Mappings contables incompletos", checklist.MissingMappings, "/accounting/accounts"},
		{"exchange_rates", "Cotizaciones faltantes", checklist.MissingExchangeRates, "/accounting/inflation"},
		{"unreconciled_accounts", "Cuentas sin conciliar", checklist.UnclosedReconciliations, "/accounting/reconciliation"},
		{"pending_drafts", "Borradores contables pendientes", checklist.PendingDrafts, "/accounting/journal"},
	}
	response := api.AccountingCloseReadiness{
		BlockingCount: checklist.BlockingCount(),
		Checks:        make([]api.AccountingCloseCheck, 0, len(checks)),
		Status:        "ready",
	}
	if checklist.EvaluatedAt == nil {
		response.Status = "not_evaluated"
	} else {
		value := checklist.EvaluatedAt.UTC()
		response.EvaluatedAt = &value
		if response.BlockingCount > 0 {
			response.Status = "blocked"
		}
	}
	for _, check := range checks {
		status := api.AccountingCloseCheckStatusPassed
		if checklist.EvaluatedAt == nil {
			status = api.AccountingCloseCheckStatusNotEvaluated
		} else if check.count > 0 {
			status = api.AccountingCloseCheckStatusBlocked
		}
		target := check.target
		response.Checks = append(response.Checks, api.AccountingCloseCheck{
			Code:       check.code,
			Count:      check.count,
			Label:      check.label,
			Status:     status,
			TargetPath: &target,
		})
	}
	return response
}

func apiPeriodEvent(event accounting.PeriodAudit) api.AccountingPeriodEvent {
	response := api.AccountingPeriodEvent{
		Actor:      event.ActorID,
		FromState:  api.AccountingPeriodState(event.FromStatus),
		Id:         openapi_types.UUID(event.ID),
		OccurredAt: event.OccurredAt,
		PeriodId:   openapi_types.UUID(event.PeriodID),
		ToState:    api.AccountingPeriodState(event.ToStatus),
		ToVersion:  event.ToVersion,
	}
	if event.FromVersion != nil {
		response.FromVersion = event.FromVersion
	}
	if strings.TrimSpace(event.Reason) != "" {
		reason := event.Reason
		response.Reason = &reason
	}
	return response
}
