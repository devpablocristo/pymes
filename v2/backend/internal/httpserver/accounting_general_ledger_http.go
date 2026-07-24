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

type generalLedgerCursorWire struct {
	Date        string `json:"d"`
	EntryNumber int64  `json:"n"`
	LineNumber  int    `json:"l"`
	LineID      string `json:"i"`
}

func (h *IAMAPI) getGeneralLedger(
	w http.ResponseWriter,
	r *http.Request,
	params api.GetGeneralLedgerParams,
) {
	filter, ok := generalLedgerFilterFromRequest(
		w,
		params.AccountId,
		params.From.Time,
		params.To.Time,
		params.Query,
		params.Origin,
		params.Cursor,
		accountingAPILimit(params.Limit),
	)
	if !ok {
		return
	}
	var (
		ledger   accounting.GeneralLedgerPage
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
			ledger, err = service.ListGeneralLedger(ctx, scope, filter)
			return err
		},
	) {
		return
	}
	writeJSON(w, http.StatusOK, apiGeneralLedger(ledger, currency))
}

func (h *IAMAPI) exportGeneralLedger(
	w http.ResponseWriter,
	r *http.Request,
	params api.ExportGeneralLedgerParams,
) {
	filter, ok := generalLedgerFilterFromRequest(
		w,
		params.AccountId,
		params.From.Time,
		params.To.Time,
		params.Query,
		params.Origin,
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
			_ pgx.Tx,
		) error {
			ledger, err := collectGeneralLedger(ctx, service, scope, filter)
			if err != nil {
				return err
			}
			table := accounting.GeneralLedgerPageReportTable(ledger)
			switch params.Format {
			case api.ExportGeneralLedgerParamsFormatCsv:
				body, err = exportReportCSV(table)
				contentType, extension = "text/csv; charset=utf-8", "csv"
			case api.ExportGeneralLedgerParamsFormatXlsx:
				body, err = accounting.ExportReportXLSX(table)
				contentType, extension = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"
			case api.ExportGeneralLedgerParamsFormatPdf:
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
			`attachment; filename="libro-mayor-%s-%s.%s"`,
			params.From.Time.Format("20060102"),
			params.To.Time.Format("20060102"),
			extension,
		),
	)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func generalLedgerFilterFromRequest(
	w http.ResponseWriter,
	accountID openapi_types.UUID,
	from time.Time,
	to time.Time,
	query *string,
	origin *string,
	rawCursor *api.Cursor,
	limit int,
) (accounting.GeneralLedgerFilter, bool) {
	if to.Before(from) {
		writeAPIError(
			w,
			http.StatusBadRequest,
			"REQUEST_INVALID",
			"La fecha hasta no puede ser anterior a desde.",
		)
		return accounting.GeneralLedgerFilter{}, false
	}
	cursor, err := decodeGeneralLedgerCursor(rawCursor)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "El cursor del Mayor no es válido.")
		return accounting.GeneralLedgerFilter{}, false
	}
	filter := accounting.GeneralLedgerFilter{
		AccountID: uuid.UUID(accountID),
		From:      from,
		To:        to,
		Query:     stringValue(query),
		Origin:    stringValue(origin),
		Cursor:    cursor,
		Limit:     limit,
	}
	if err := filter.Validate(); err != nil {
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", "Los filtros del Libro Mayor no son válidos.")
		return accounting.GeneralLedgerFilter{}, false
	}
	return filter, true
}

func collectGeneralLedger(
	ctx context.Context,
	service *accounting.Service,
	scope accounting.Scope,
	filter accounting.GeneralLedgerFilter,
) (accounting.GeneralLedgerPage, error) {
	var result accounting.GeneralLedgerPage
	items := make([]accounting.GeneralLedgerMovement, 0)
	for {
		page, err := service.ListGeneralLedger(ctx, scope, filter)
		if err != nil {
			return accounting.GeneralLedgerPage{}, err
		}
		if result.Account.ID == uuid.Nil {
			result = page
			result.Items = make([]accounting.GeneralLedgerMovement, 0, page.Total)
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

func apiGeneralLedger(
	ledger accounting.GeneralLedgerPage,
	currency accounting.Currency,
) api.GeneralLedger {
	items := make([]api.GeneralLedgerLine, 0, len(ledger.Items))
	for _, line := range ledger.Items {
		items = append(items, api.GeneralLedgerLine{
			AccountingDate: openapi_types.Date{Time: line.Date},
			Balance:        apiGeneralLedgerBalance(line.Balance),
			Credit:         line.Credit.String(),
			Debit:          line.Debit.String(),
			Description:    line.Description,
			EntryId:        line.EntryID,
			EntryNumber:    line.EntryNumber,
			LineId:         line.LineID,
			LineNumber:     line.LineNumber,
			Memo:           line.Memo,
			Origin:         line.Origin,
			Reference:      line.Reference,
		})
	}
	return api.GeneralLedger{
		Account: apiAccount(
			ledger.Account,
			api.LifecycleState(ledger.Account.LifecycleState()),
		),
		ClosingBalance: apiGeneralLedgerBalance(ledger.ClosingBalance),
		Currency:       currency.Code(),
		From:           openapi_types.Date{Time: ledger.From},
		Items:          items,
		OpeningBalance: apiGeneralLedgerBalance(ledger.OpeningBalance),
		Page: api.PageInfo{
			NextCursor: encodeGeneralLedgerCursor(ledger.NextCursor),
			Total:      ledger.Total,
		},
		To:          openapi_types.Date{Time: ledger.To},
		TotalCredit: ledger.TotalCredit.String(),
		TotalDebit:  ledger.TotalDebit.String(),
	}
}

func apiGeneralLedgerBalance(value accounting.Decimal) api.GeneralLedgerBalance {
	side := api.GeneralLedgerBalanceSideZero
	amount := value
	if value.Sign() > 0 {
		side = api.GeneralLedgerBalanceSideDebit
	} else if value.Sign() < 0 {
		side = api.GeneralLedgerBalanceSideCredit
		amount = value.Abs()
	}
	return api.GeneralLedgerBalance{Amount: amount.Abs().String(), Side: side}
}

func encodeGeneralLedgerCursor(cursor *accounting.GeneralLedgerCursor) *string {
	if cursor == nil || !cursor.Valid() {
		return nil
	}
	raw, err := json.Marshal(generalLedgerCursorWire{
		Date:        cursor.Date.Format("2006-01-02"),
		EntryNumber: cursor.EntryNumber,
		LineNumber:  cursor.LineNumber,
		LineID:      cursor.LineID.String(),
	})
	if err != nil {
		return nil
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return &encoded
}

func decodeGeneralLedgerCursor(raw *api.Cursor) (*accounting.GeneralLedgerCursor, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(string(*raw))
	if err != nil {
		return nil, err
	}
	var payload generalLedgerCursorWire
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}
	date, err := time.Parse("2006-01-02", payload.Date)
	if err != nil {
		return nil, err
	}
	lineID, err := uuid.Parse(payload.LineID)
	if err != nil {
		return nil, err
	}
	cursor := &accounting.GeneralLedgerCursor{
		Date:        date,
		EntryNumber: payload.EntryNumber,
		LineNumber:  payload.LineNumber,
		LineID:      lineID,
	}
	if !cursor.Valid() {
		return nil, fmt.Errorf("invalid general ledger cursor")
	}
	return cursor, nil
}
