package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func TestTrialBalanceCursorRoundTripsStableNaturalOrderFields(t *testing.T) {
	t.Parallel()

	want := &accounting.TrialBalanceCursor{
		Code:      "1.10.02",
		AccountID: uuid.New(),
	}
	raw := encodeTrialBalanceCursor(want)
	if raw == nil {
		t.Fatal("encoded cursor is nil")
	}
	got, err := decodeTrialBalanceCursor((*api.Cursor)(raw))
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if *got != *want {
		t.Fatalf("cursor = %#v, want %#v", got, want)
	}

	invalid := api.Cursor("not-a-cursor")
	if _, err := decodeTrialBalanceCursor(&invalid); err == nil {
		t.Fatal("invalid cursor was accepted")
	}
}

func TestTrialBalanceBalanceUsesAbsoluteAmountAndExplicitSide(t *testing.T) {
	t.Parallel()

	for name, fixture := range map[string]struct {
		input    accounting.Decimal
		amount   string
		expected api.TrialBalanceSide
	}{
		"debit":  {accounting.MustDecimal("12.5"), "12.5", api.TrialBalanceSideDebit},
		"credit": {accounting.MustDecimal("-12.5"), "12.5", api.TrialBalanceSideCredit},
		"zero":   {accounting.Zero, "0", api.TrialBalanceSideZero},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			balance := apiTrialBalanceBalance(fixture.input)
			if balance.Amount != fixture.amount || balance.Side != fixture.expected {
				t.Fatalf("balance = %#v", balance)
			}
		})
	}
}

func TestTrialBalanceRequestBuildsDomainFilterAndRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	query := " ventas "
	class := api.Income
	includeZero := true
	recorder := httptest.NewRecorder()
	filter, ok := trialBalanceFilterFromRequest(
		recorder,
		from,
		to,
		&query,
		&class,
		&includeZero,
		nil,
		75,
	)
	if !ok {
		t.Fatalf("valid filter rejected: status=%d body=%s", recorder.Code, recorder.Body)
	}
	if filter.AccountClass != accounting.AccountRevenue ||
		filter.Query != "ventas" ||
		!filter.IncludeZero ||
		filter.Limit != 75 {
		t.Fatalf("filter = %#v", filter)
	}

	recorder = httptest.NewRecorder()
	_, ok = trialBalanceFilterFromRequest(
		recorder,
		to,
		from,
		nil,
		nil,
		nil,
		nil,
		50,
	)
	if ok || recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid period result = ok:%t status:%d", ok, recorder.Code)
	}
}

func TestTrialBalanceAPILimitMatchesOpenAPIContract(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	if limit, ok := trialBalanceAPILimit(recorder, nil); !ok || limit != 25 {
		t.Fatalf("default limit = %d, ok = %t", limit, ok)
	}

	valid := api.Limit(100)
	recorder = httptest.NewRecorder()
	if limit, ok := trialBalanceAPILimit(recorder, &valid); !ok || limit != 100 {
		t.Fatalf("valid limit = %d, ok = %t", limit, ok)
	}

	for _, value := range []api.Limit{0, -1, 101, 999} {
		recorder = httptest.NewRecorder()
		if limit, ok := trialBalanceAPILimit(recorder, &value); ok ||
			limit != 0 ||
			recorder.Code != http.StatusBadRequest {
			t.Fatalf(
				"invalid limit %d = %d, ok = %t, status = %d",
				value,
				limit,
				ok,
				recorder.Code,
			)
		}
	}
}

func TestAPITrialBalancePreservesPathTotalsControlsAndPagination(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	accountID := uuid.New()
	nextID := uuid.New()
	response := apiTrialBalance(accounting.TrialBalancePage{
		From: from,
		To:   to,
		Items: []accounting.TrialBalanceAccountRow{{
			AccountID:      accountID,
			Code:           "4.1",
			Name:           "Ventas",
			Class:          accounting.AccountRevenue,
			NormalBalance:  accounting.NormalCredit,
			Path:           []string{"Ingresos", "Ventas"},
			LifecycleState: accounting.AccountArchived,
			OpeningBalance: accounting.MustDecimal("-25"),
			Debit:          accounting.MustDecimal("10"),
			Credit:         accounting.MustDecimal("85"),
			ClosingBalance: accounting.MustDecimal("-100"),
		}},
		Totals: accounting.TrialBalanceTotals{
			OpeningDebit:       accounting.MustDecimal("20"),
			OpeningCredit:      accounting.MustDecimal("20"),
			Debit:              accounting.MustDecimal("100"),
			Credit:             accounting.MustDecimal("100"),
			ClosingDebit:       accounting.MustDecimal("120"),
			ClosingCredit:      accounting.MustDecimal("120"),
			OpeningDifference:  accounting.Zero,
			MovementDifference: accounting.Zero,
			ClosingDifference:  accounting.Zero,
		},
		Total: 2,
		NextCursor: &accounting.TrialBalanceCursor{
			Code:      "4.2",
			AccountID: nextID,
		},
	}, accounting.MustCurrency("ARS"))

	if response.Currency != "ARS" ||
		response.From.Time != from ||
		response.To.Time != to ||
		response.Page.Total != 2 ||
		response.Page.NextCursor == nil {
		t.Fatalf("response metadata = %#v", response)
	}
	if len(response.Items) != 1 {
		t.Fatalf("item count = %d", len(response.Items))
	}
	item := response.Items[0]
	if item.AccountId != openapi_types.UUID(accountID) ||
		item.AccountClass != api.Income ||
		item.LifecycleState != api.LifecycleStateArchived ||
		item.OpeningBalance.Side != api.TrialBalanceSideCredit ||
		item.OpeningBalance.Amount != "25" ||
		item.ClosingBalance.Side != api.TrialBalanceSideCredit ||
		item.ClosingBalance.Amount != "100" ||
		len(item.Path) != 2 ||
		item.Path[0] != "Ingresos" {
		t.Fatalf("item = %#v", item)
	}
	if response.Totals.MovementDebit != "100" ||
		response.Totals.MovementCredit != "100" ||
		response.Controls.ClosingDifference != "0" {
		t.Fatalf("totals=%#v controls=%#v", response.Totals, response.Controls)
	}
}

func TestCollectTrialBalancePagesReturnsEveryPageAndGlobalTotals(t *testing.T) {
	t.Parallel()

	firstID := uuid.New()
	secondID := uuid.New()
	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	totals := accounting.TrialBalanceTotals{
		Debit:  accounting.MustDecimal("500"),
		Credit: accounting.MustDecimal("500"),
	}
	calls := 0
	page, err := collectTrialBalancePages(
		accounting.TrialBalanceFilter{From: from, To: to, Limit: 200},
		func(filter accounting.TrialBalanceFilter) (accounting.TrialBalancePage, error) {
			calls++
			if filter.Cursor == nil {
				return accounting.TrialBalancePage{
					From:   from,
					To:     to,
					Items:  []accounting.TrialBalanceAccountRow{{AccountID: firstID, Code: "1.1.01"}},
					Totals: totals,
					Total:  2,
					NextCursor: &accounting.TrialBalanceCursor{
						Code:      "1.1.01",
						AccountID: firstID,
					},
				}, nil
			}
			if filter.Cursor.AccountID != firstID {
				t.Fatalf("second page cursor = %#v", filter.Cursor)
			}
			return accounting.TrialBalancePage{
				From:   from,
				To:     to,
				Items:  []accounting.TrialBalanceAccountRow{{AccountID: secondID, Code: "2.1.01"}},
				Totals: totals,
				Total:  2,
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("collect pages: %v", err)
	}
	if calls != 2 || len(page.Items) != 2 || page.NextCursor != nil ||
		page.Total != 2 || page.Totals.Debit.String() != "500" {
		t.Fatalf("page = %#v, calls = %d", page, calls)
	}
}

func TestTrialBalanceExportIdentifiesFunctionalCurrency(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	table := trialBalanceExportTable(
		accounting.TrialBalancePage{From: day, To: day},
		accounting.MustCurrency("ARS"),
	)
	if !strings.Contains(table.Subtitle, "Moneda ARS") {
		t.Fatalf("subtitle = %q", table.Subtitle)
	}
}

func TestTrialBalanceHTTPAllowsAccountingViewAndUsesContractLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		query         string
		expectedLimit int
		expectedItems int
		expectsCursor bool
	}{
		{
			name:          "OpenAPI default",
			query:         "",
			expectedLimit: 26,
			expectedItems: 2,
		},
		{
			name:          "minimum",
			query:         "&limit=1",
			expectedLimit: 2,
			expectedItems: 1,
			expectsCursor: true,
		},
		{
			name:          "maximum",
			query:         "&limit=100",
			expectedLimit: 101,
			expectedItems: 2,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			claims := teamReadClaims("org:member")
			tx := newTrialBalanceHTTPFixtureTx()
			handler := newTeamReadTestHandler(
				t,
				claims,
				teamReadTransactor(
					t,
					claims,
					teamReadActiveMembership("member"),
					tx,
				),
			)
			response := httptest.NewRecorder()
			handler.ServeHTTP(
				response,
				newTeamReadRequest(
					http.MethodGet,
					"/api/v1/accounting/trial-balance?from=2026-07-01&to=2026-07-31"+test.query,
				),
			)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body)
			}
			var body api.TrialBalance
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Currency != "ARS" ||
				body.Page.Total != 2 ||
				len(body.Items) != test.expectedItems ||
				(body.Page.NextCursor != nil) != test.expectsCursor {
				t.Fatalf("response = %#v", body)
			}
			if tx.lastRowLimit != test.expectedLimit {
				t.Fatalf(
					"repository limit = %d, want %d",
					tx.lastRowLimit,
					test.expectedLimit,
				)
			}
			assertTrialBalanceBodyHasNoTenantSelector(t, response.Body.String())
		})
	}
}

func TestTrialBalanceHTTPRejectsLimitsOutsideOpenAPIContract(t *testing.T) {
	t.Parallel()

	for _, limit := range []string{"0", "-1", "101", "999"} {
		limit := limit
		t.Run(limit, func(t *testing.T) {
			t.Parallel()

			claims := teamReadClaims("org:member")
			tx := newTrialBalanceHTTPFixtureTx()
			handler := newTeamReadTestHandler(
				t,
				claims,
				teamReadTransactor(
					t,
					claims,
					teamReadActiveMembership("member"),
					tx,
				),
			)
			response := httptest.NewRecorder()
			handler.ServeHTTP(
				response,
				newTeamReadRequest(
					http.MethodGet,
					"/api/v1/accounting/trial-balance?from=2026-07-01&to=2026-07-31&limit="+limit,
				),
			)

			assertAPIError(t, response, http.StatusBadRequest, "REQUEST_INVALID")
			if tx.execCalls != 0 || tx.queryRowCalls != 0 || tx.queryCalls != 0 {
				t.Fatalf(
					"invalid limit reached accounting tx: exec=%d row=%d rows=%d",
					tx.execCalls,
					tx.queryRowCalls,
					tx.queryCalls,
				)
			}
		})
	}
}

func TestTrialBalanceHTTPRequiresAccountingView(t *testing.T) {
	t.Parallel()

	targets := []string{
		"/api/v1/accounting/trial-balance?from=2026-07-01&to=2026-07-31",
		"/api/v1/accounting/trial-balance/export?format=csv&from=2026-07-01&to=2026-07-31",
	}
	for _, target := range targets {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			claims := teamReadClaims("org:member")
			tx := newTrialBalanceHTTPFixtureTx()
			handler := newTeamReadTestHandler(
				t,
				claims,
				teamReadTransactor(
					t,
					claims,
					teamReadActiveMembership("role-without-accounting-view"),
					tx,
				),
			)
			response := httptest.NewRecorder()
			handler.ServeHTTP(
				response,
				newTeamReadRequest(http.MethodGet, target),
			)

			assertAPIError(t, response, http.StatusForbidden, "IAM_FORBIDDEN")
			if tx.execCalls != 0 || tx.queryRowCalls != 0 || tx.queryCalls != 0 {
				t.Fatalf("forbidden request reached accounting tx")
			}
		})
	}
}

func TestTrialBalanceDedicatedExportDownloadsCompleteCSV(t *testing.T) {
	t.Parallel()

	claims := teamReadClaims("org:member")
	tx := newTrialBalanceHTTPFixtureTx()
	handler := newTeamReadTestHandler(
		t,
		claims,
		teamReadTransactor(
			t,
			claims,
			teamReadActiveMembership("member"),
			tx,
		),
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		newTeamReadRequest(
			http.MethodGet,
			"/api/v1/accounting/trial-balance/export?format=csv&from=2026-07-01&to=2026-07-31",
		),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	if response.Header().Get("Content-Type") != "text/csv; charset=utf-8" {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
	if disposition := response.Header().Get("Content-Disposition"); !strings.Contains(
		disposition,
		`filename="balance-sumas-saldos-20260701-20260731.csv"`,
	) {
		t.Fatalf("content disposition = %q", disposition)
	}
	body := response.Body.String()
	for _, fragment := range []string{
		"Balance de sumas y saldos",
		"Moneda ARS",
		"Caja",
		"Capital",
		"TOTAL",
		"DIFERENCIA DE CONTROL",
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("CSV is missing %q: %s", fragment, body)
		}
	}
	if tx.queryCalls != 1 || tx.lastRowLimit != 201 {
		t.Fatalf(
			"export queries = %d, repository limit = %d",
			tx.queryCalls,
			tx.lastRowLimit,
		)
	}
}

func TestTrialBalanceLegacyHTTPRemainsCompatible(t *testing.T) {
	t.Parallel()

	t.Run("JSON", func(t *testing.T) {
		t.Parallel()

		claims := teamReadClaims("org:member")
		tx := newTrialBalanceHTTPFixtureTx()
		handler := newTeamReadTestHandler(
			t,
			claims,
			teamReadTransactor(
				t,
				claims,
				teamReadActiveMembership("member"),
				tx,
			),
		)
		response := httptest.NewRecorder()
		handler.ServeHTTP(
			response,
			newTeamReadRequest(
				http.MethodGet,
				"/api/v1/accounting/reports/trial-balance?from=2026-07-01&to=2026-07-31",
			),
		)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body)
		}
		var body api.AccountingReport
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode legacy response: %v", err)
		}
		if body.Report != "trial-balance" ||
			body.Currency != "ARS" ||
			len(body.Rows) != 2 ||
			body.TotalDebit != "100" ||
			body.TotalCredit != "100" {
			t.Fatalf("legacy response = %#v", body)
		}
		assertTrialBalanceBodyHasNoTenantSelector(t, response.Body.String())
	})

	t.Run("CSV export", func(t *testing.T) {
		t.Parallel()

		claims := teamReadClaims("org:member")
		tx := newTrialBalanceHTTPFixtureTx()
		handler := newTeamReadTestHandler(
			t,
			claims,
			teamReadTransactor(
				t,
				claims,
				teamReadActiveMembership("member"),
				tx,
			),
		)
		response := httptest.NewRecorder()
		handler.ServeHTTP(
			response,
			newTeamReadRequest(
				http.MethodGet,
				"/api/v1/accounting/reports/trial-balance/export?format=csv&from=2026-07-01&to=2026-07-31",
			),
		)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body)
		}
		if response.Header().Get("Content-Type") != "text/csv; charset=utf-8" {
			t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
		}
		if disposition := response.Header().Get("Content-Disposition"); !strings.Contains(
			disposition,
			`filename="trial-balance-20260701-20260731.csv"`,
		) {
			t.Fatalf("content disposition = %q", disposition)
		}
		for _, fragment := range []string{
			"Balance de sumas y saldos",
			"Moneda ARS",
			"Caja",
			"Capital",
			"DIFERENCIA DE CONTROL",
		} {
			if !strings.Contains(response.Body.String(), fragment) {
				t.Fatalf("legacy CSV is missing %q", fragment)
			}
		}
	})
}

func assertTrialBalanceBodyHasNoTenantSelector(t *testing.T, body string) {
	t.Helper()

	for _, forbidden := range []string{
		`"org_id"`,
		`"organization_id"`,
		`"tenant_id"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposes %s: %s", forbidden, body)
		}
	}
}

type trialBalanceHTTPRowFunc func(...any) error

func (row trialBalanceHTTPRowFunc) Scan(destinations ...any) error {
	return row(destinations...)
}

type trialBalanceHTTPFixtureRow struct {
	accountID      uuid.UUID
	code           string
	name           string
	class          accounting.AccountClass
	normalBalance  accounting.NormalBalance
	path           []string
	lifecycleState accounting.AccountLifecycleState
	opening        string
	debit          string
	credit         string
	closing        string
}

type trialBalanceHTTPRows struct {
	values []trialBalanceHTTPFixtureRow
	index  int
	closed bool
}

func (rows *trialBalanceHTTPRows) Close() {
	rows.closed = true
}

func (rows *trialBalanceHTTPRows) Err() error {
	return nil
}

func (rows *trialBalanceHTTPRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("SELECT")
}

func (rows *trialBalanceHTTPRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (rows *trialBalanceHTTPRows) Next() bool {
	if rows.closed || rows.index >= len(rows.values) {
		return false
	}
	rows.index++
	return true
}

func (rows *trialBalanceHTTPRows) Scan(destinations ...any) error {
	if rows.index == 0 || rows.index > len(rows.values) {
		return fmt.Errorf("trial balance HTTP row is not positioned")
	}
	if len(destinations) != 11 {
		return fmt.Errorf(
			"trial balance HTTP row destinations = %d",
			len(destinations),
		)
	}
	value := rows.values[rows.index-1]
	*destinations[0].(*uuid.UUID) = value.accountID
	*destinations[1].(*string) = value.code
	*destinations[2].(*string) = value.name
	*destinations[3].(*accounting.AccountClass) = value.class
	*destinations[4].(*accounting.NormalBalance) = value.normalBalance
	*destinations[5].(*[]string) = append([]string(nil), value.path...)
	*destinations[6].(*accounting.AccountLifecycleState) = value.lifecycleState
	*destinations[7].(*string) = value.opening
	*destinations[8].(*string) = value.debit
	*destinations[9].(*string) = value.credit
	*destinations[10].(*string) = value.closing
	return nil
}

func (rows *trialBalanceHTTPRows) Values() ([]any, error) {
	return nil, nil
}

func (rows *trialBalanceHTTPRows) RawValues() [][]byte {
	return nil
}

func (rows *trialBalanceHTTPRows) Conn() *pgx.Conn {
	return nil
}

type trialBalanceHTTPFixtureTx struct {
	pgx.Tx
	rows          []trialBalanceHTTPFixtureRow
	execCalls     int
	queryRowCalls int
	queryCalls    int
	lastRowLimit  int
}

func newTrialBalanceHTTPFixtureTx() *trialBalanceHTTPFixtureTx {
	return &trialBalanceHTTPFixtureTx{
		rows: []trialBalanceHTTPFixtureRow{
			{
				accountID:      uuid.MustParse("11111111-1111-4111-8111-111111111111"),
				code:           "1.1.01",
				name:           "Caja",
				class:          accounting.AccountAsset,
				normalBalance:  accounting.NormalDebit,
				path:           []string{"Activo", "Disponibilidades", "Caja"},
				lifecycleState: accounting.AccountActive,
				opening:        "50",
				debit:          "100",
				credit:         "0",
				closing:        "150",
			},
			{
				accountID:      uuid.MustParse("33333333-3333-4333-8333-333333333333"),
				code:           "3.1",
				name:           "Capital",
				class:          accounting.AccountEquity,
				normalBalance:  accounting.NormalCredit,
				path:           []string{"Patrimonio neto", "Capital"},
				lifecycleState: accounting.AccountActive,
				opening:        "-50",
				debit:          "0",
				credit:         "100",
				closing:        "-150",
			},
		},
	}
}

func (tx *trialBalanceHTTPFixtureTx) Exec(
	_ context.Context,
	query string,
	_ ...any,
) (pgconn.CommandTag, error) {
	if !strings.Contains(query, "set_config('app.org_id'") {
		return pgconn.CommandTag{}, fmt.Errorf(
			"unexpected trial balance Exec: %s",
			query,
		)
	}
	tx.execCalls++
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func (tx *trialBalanceHTTPFixtureTx) QueryRow(
	_ context.Context,
	query string,
	_ ...any,
) pgx.Row {
	tx.queryRowCalls++
	switch {
	case strings.Contains(query, "functional_currency"):
		return trialBalanceHTTPRowFunc(func(destinations ...any) error {
			if len(destinations) != 1 {
				return fmt.Errorf(
					"currency destinations = %d",
					len(destinations),
				)
			}
			*destinations[0].(*string) = "ARS"
			return nil
		})
	case strings.Contains(query, "count(*)"):
		return trialBalanceHTTPRowFunc(func(destinations ...any) error {
			if len(destinations) != 7 {
				return fmt.Errorf(
					"trial balance summary destinations = %d",
					len(destinations),
				)
			}
			*destinations[0].(*int64) = int64(len(tx.rows))
			*destinations[1].(*string) = "50"
			*destinations[2].(*string) = "50"
			*destinations[3].(*string) = "100"
			*destinations[4].(*string) = "100"
			*destinations[5].(*string) = "150"
			*destinations[6].(*string) = "150"
			return nil
		})
	default:
		return trialBalanceHTTPRowFunc(func(...any) error {
			return fmt.Errorf("unexpected trial balance QueryRow: %s", query)
		})
	}
}

func (tx *trialBalanceHTTPFixtureTx) Query(
	_ context.Context,
	query string,
	args ...any,
) (pgx.Rows, error) {
	if !strings.Contains(query, "accounting.account_code_sort_key(code)") {
		return nil, fmt.Errorf("unexpected trial balance Query: %s", query)
	}
	tx.queryCalls++
	if len(args) != 9 {
		return nil, fmt.Errorf("trial balance query args = %d", len(args))
	}
	limit, ok := args[8].(int)
	if !ok {
		return nil, fmt.Errorf("trial balance limit type = %T", args[8])
	}
	tx.lastRowLimit = limit
	return &trialBalanceHTTPRows{
		values: append([]trialBalanceHTTPFixtureRow(nil), tx.rows...),
	}, nil
}
