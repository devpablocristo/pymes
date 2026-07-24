package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func TestGeneralLedgerCursorRoundTripsAllStableSortFields(t *testing.T) {
	t.Parallel()

	want := &accounting.GeneralLedgerCursor{
		Date:        time.Date(2026, time.July, 24, 0, 0, 0, 0, time.UTC),
		EntryNumber: 42,
		LineNumber:  3,
		LineID:      uuid.New(),
	}
	raw := encodeGeneralLedgerCursor(want)
	if raw == nil {
		t.Fatal("encoded cursor is nil")
	}
	got, err := decodeGeneralLedgerCursor((*api.Cursor)(raw))
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if *got != *want {
		t.Fatalf("cursor = %#v, want %#v", got, want)
	}
	invalid := api.Cursor("not-a-cursor")
	if _, err := decodeGeneralLedgerCursor(&invalid); err == nil {
		t.Fatal("invalid cursor was accepted")
	}
}

func TestGeneralLedgerBalanceUsesAbsoluteAmountAndSide(t *testing.T) {
	t.Parallel()

	for name, fixture := range map[string]struct {
		input    accounting.Decimal
		amount   string
		expected api.GeneralLedgerBalanceSide
	}{
		"debit":  {accounting.MustDecimal("12.5"), "12.5", api.GeneralLedgerBalanceSideDebit},
		"credit": {accounting.MustDecimal("-12.5"), "12.5", api.GeneralLedgerBalanceSideCredit},
		"zero":   {accounting.Zero, "0", api.GeneralLedgerBalanceSideZero},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			balance := apiGeneralLedgerBalance(fixture.input)
			if balance.Amount != fixture.amount || balance.Side != fixture.expected {
				t.Fatalf("balance = %#v", balance)
			}
		})
	}
}

func TestGeneralLedgerRequestRejectsInvalidPeriodBeforeService(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	_, ok := generalLedgerFilterFromRequest(
		recorder,
		openapi_types.UUID(uuid.New()),
		time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		nil,
		nil,
		nil,
		50,
	)
	if ok || recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid period result = ok:%t status:%d", ok, recorder.Code)
	}
}
