package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
)

func TestFiscalYearCursorRoundTripKeepsStablePosition(t *testing.T) {
	t.Parallel()

	year := accounting.FiscalYearSummary{
		ID:        uuid.New(),
		StartDate: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
	}
	encoded := encodeFiscalYearCursor(year)
	date, id, err := decodeFiscalYearCursor(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if date == nil || id == nil ||
		!date.Equal(year.StartDate) || *id != year.ID {
		t.Fatalf("decoded cursor = %v/%v", date, id)
	}
}

func TestFiscalYearCursorRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	if _, _, err := decodeFiscalYearCursor("not-a-cursor"); !errors.Is(
		err,
		accounting.ErrInvalidArgument,
	) {
		t.Fatalf("cursor error = %v", err)
	}
}
