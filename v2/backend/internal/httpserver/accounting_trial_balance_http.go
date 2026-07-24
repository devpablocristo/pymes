package httpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type trialBalanceCursorWire struct {
	Code      string `json:"c"`
	AccountID string `json:"i"`
}

func (h *IAMAPI) getTrialBalance(
	w http.ResponseWriter,
	r *http.Request,
	params api.GetTrialBalanceParams,
) {
	limit, ok := trialBalanceAPILimit(w, params.Limit)
	if !ok {
		return
	}
	filter, ok := trialBalanceFilterFromRequest(
		w,
		params.From.Time,
		params.To.Time,
		params.Query,
		params.AccountClass,
		params.IncludeZero,
		params.Cursor,
		limit,
	)
	if !ok {
		return
	}
	var (
		page     accounting.TrialBalancePage
		currency accounting.Currency
	)
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
			var err error
			currency, err = loadFunctionalCurrency(ctx, tx)
			if err != nil {
				return err
			}
			page, err = service.ListTrialBalance(ctx, scope, filter)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, apiTrialBalance(page, currency))
}

func (h *IAMAPI) exportTrialBalance(
	w http.ResponseWriter,
	r *http.Request,
	params api.ExportTrialBalanceParams,
) {
	filter, ok := trialBalanceFilterFromRequest(
		w,
		params.From.Time,
		params.To.Time,
		params.Query,
		params.AccountClass,
		params.IncludeZero,
		nil,
		200,
	)
	if !ok {
		return
	}
	var (
		body        []byte
		contentType string
		extension   string
	)
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
			currency, err := loadFunctionalCurrency(ctx, tx)
			if err != nil {
				return err
			}
			page, err := collectTrialBalance(ctx, service, scope, filter)
			if err != nil {
				return err
			}
			table := trialBalanceExportTable(page, currency)
			switch params.Format {
			case api.ExportTrialBalanceParamsFormatCsv:
				body, err = exportReportCSV(table)
				contentType, extension = "text/csv; charset=utf-8", "csv"
			case api.ExportTrialBalanceParamsFormatXlsx:
				body, err = accounting.ExportReportXLSX(table)
				contentType, extension = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"
			case api.ExportTrialBalanceParamsFormatPdf:
				body, err = accounting.ExportReportPDF(table)
				contentType, extension = "application/pdf", "pdf"
			default:
				return accounting.ErrInvalidArgument
			}
			return err
		},
	) {
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set(
		"Content-Disposition",
		fmt.Sprintf(
			`attachment; filename="balance-sumas-saldos-%s-%s.%s"`,
			params.From.Time.Format("20060102"),
			params.To.Time.Format("20060102"),
			extension,
		),
	)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func trialBalanceAPILimit(w http.ResponseWriter, raw *api.Limit) (int, bool) {
	if raw == nil {
		return 25, true
	}
	if *raw < 1 || *raw > 100 {
		writeAPIError(
			w,
			http.StatusBadRequest,
			"REQUEST_INVALID",
			"El límite debe estar entre 1 y 100.",
		)
		return 0, false
	}
	return int(*raw), true
}

func trialBalanceExportTable(
	page accounting.TrialBalancePage,
	currency accounting.Currency,
) accounting.ReportTable {
	table := accounting.TrialBalancePageReportTable(page)
	table.Subtitle += " · Moneda " + currency.Code()
	return table
}

func trialBalanceFilterFromRequest(
	w http.ResponseWriter,
	from time.Time,
	to time.Time,
	query *string,
	rawClass *api.AccountingAccountType,
	includeZero *bool,
	rawCursor *api.Cursor,
	limit int,
) (accounting.TrialBalanceFilter, bool) {
	if to.Before(from) {
		writeAPIError(
			w,
			http.StatusBadRequest,
			"REQUEST_INVALID",
			"La fecha hasta no puede ser anterior a desde.",
		)
		return accounting.TrialBalanceFilter{}, false
	}
	var accountClass accounting.AccountClass
	if rawClass != nil {
		var err error
		accountClass, err = domainAccountClass(*rawClass)
		if err != nil {
			writeAPIError(
				w,
				http.StatusBadRequest,
				"REQUEST_INVALID",
				"La clase contable seleccionada no es válida.",
			)
			return accounting.TrialBalanceFilter{}, false
		}
	}
	cursor, err := decodeTrialBalanceCursor(rawCursor)
	if err != nil {
		writeAPIError(
			w,
			http.StatusBadRequest,
			"REQUEST_INVALID",
			"El cursor del balance de sumas y saldos no es válido.",
		)
		return accounting.TrialBalanceFilter{}, false
	}
	filter := accounting.TrialBalanceFilter{
		From:         from,
		To:           to,
		Query:        stringValue(query),
		AccountClass: accountClass,
		IncludeZero:  includeZero != nil && *includeZero,
		Cursor:       cursor,
		Limit:        limit,
	}
	if err := filter.Validate(); err != nil {
		writeAPIError(
			w,
			http.StatusBadRequest,
			"REQUEST_INVALID",
			"Los filtros del balance de sumas y saldos no son válidos.",
		)
		return accounting.TrialBalanceFilter{}, false
	}
	return filter, true
}

func collectTrialBalance(
	ctx context.Context,
	service *accounting.Service,
	scope accounting.Scope,
	filter accounting.TrialBalanceFilter,
) (accounting.TrialBalancePage, error) {
	return collectTrialBalancePages(
		filter,
		func(filter accounting.TrialBalanceFilter) (accounting.TrialBalancePage, error) {
			return service.ListTrialBalance(ctx, scope, filter)
		},
	)
}

func collectTrialBalancePages(
	filter accounting.TrialBalanceFilter,
	load func(accounting.TrialBalanceFilter) (accounting.TrialBalancePage, error),
) (accounting.TrialBalancePage, error) {
	var result accounting.TrialBalancePage
	items := make([]accounting.TrialBalanceAccountRow, 0)
	initialized := false
	for {
		page, err := load(filter)
		if err != nil {
			return accounting.TrialBalancePage{}, err
		}
		if !initialized {
			result = page
			items = make([]accounting.TrialBalanceAccountRow, 0, page.Total)
			initialized = true
		}
		items = append(items, page.Items...)
		if page.NextCursor == nil {
			result.Items = items
			result.NextCursor = nil
			return result, nil
		}
		filter.Cursor = page.NextCursor
	}
}

func apiTrialBalance(
	page accounting.TrialBalancePage,
	currency accounting.Currency,
) api.TrialBalance {
	items := make([]api.TrialBalanceItem, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, api.TrialBalanceItem{
			AccountClass:   apiAccountTypeFromDB(string(row.Class)),
			AccountId:      openapi_types.UUID(row.AccountID),
			ClosingBalance: apiTrialBalanceBalance(row.ClosingBalance),
			Code:           row.Code,
			Credit:         row.Credit.String(),
			Debit:          row.Debit.String(),
			LifecycleState: api.LifecycleState(row.LifecycleState),
			Name:           row.Name,
			NormalBalance:  api.AccountingNormalBalance(row.NormalBalance),
			OpeningBalance: apiTrialBalanceBalance(row.OpeningBalance),
			Path:           append([]string(nil), row.Path...),
		})
	}
	return api.TrialBalance{
		Controls: api.TrialBalanceControls{
			ClosingDifference:  page.Totals.ClosingDifference.String(),
			MovementDifference: page.Totals.MovementDifference.String(),
			OpeningDifference:  page.Totals.OpeningDifference.String(),
		},
		Currency: currency.Code(),
		From:     openapi_types.Date{Time: page.From},
		Items:    items,
		Page: api.PageInfo{
			NextCursor: encodeTrialBalanceCursor(page.NextCursor),
			Total:      page.Total,
		},
		To: openapi_types.Date{Time: page.To},
		Totals: api.TrialBalanceTotals{
			ClosingCredit:  page.Totals.ClosingCredit.String(),
			ClosingDebit:   page.Totals.ClosingDebit.String(),
			MovementCredit: page.Totals.Credit.String(),
			MovementDebit:  page.Totals.Debit.String(),
			OpeningCredit:  page.Totals.OpeningCredit.String(),
			OpeningDebit:   page.Totals.OpeningDebit.String(),
		},
	}
}

func apiTrialBalanceBalance(value accounting.Decimal) api.TrialBalanceBalance {
	side := api.TrialBalanceSideZero
	if value.Sign() > 0 {
		side = api.TrialBalanceSideDebit
	} else if value.Sign() < 0 {
		side = api.TrialBalanceSideCredit
	}
	return api.TrialBalanceBalance{
		Amount: value.Abs().String(),
		Side:   side,
	}
}

func encodeTrialBalanceCursor(cursor *accounting.TrialBalanceCursor) *string {
	if cursor == nil || !cursor.Valid() {
		return nil
	}
	raw, err := json.Marshal(trialBalanceCursorWire{
		Code:      cursor.Code,
		AccountID: cursor.AccountID.String(),
	})
	if err != nil {
		return nil
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return &encoded
}

func decodeTrialBalanceCursor(raw *api.Cursor) (*accounting.TrialBalanceCursor, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(*raw))
	if err != nil {
		return nil, err
	}
	var payload trialBalanceCursorWire
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}
	accountID, err := uuid.Parse(payload.AccountID)
	if err != nil {
		return nil, err
	}
	cursor := &accounting.TrialBalanceCursor{
		Code:      payload.Code,
		AccountID: accountID,
	}
	if !cursor.Valid() {
		return nil, fmt.Errorf("invalid trial balance cursor")
	}
	return cursor, nil
}
