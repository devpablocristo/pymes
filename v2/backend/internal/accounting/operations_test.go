package accounting

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestReportsReconcileJournalLedgerTrialBalanceAndStatements(t *testing.T) {
	t.Parallel()

	cashID := uuid.New()
	equityID := uuid.New()
	revenueID := uuid.New()
	expenseID := uuid.New()
	date := dateFixture()
	lines := []ReportLine{
		{EntryID: uuid.New(), EntryNumber: 1, EntryDate: date, AccountID: cashID, AccountCode: "1.1", AccountName: "Caja", AccountClass: AccountAsset, NormalBalance: NormalDebit, Debit: MustDecimal("100")},
		{EntryID: uuid.New(), EntryNumber: 1, EntryDate: date, AccountID: equityID, AccountCode: "3.1", AccountName: "Capital", AccountClass: AccountEquity, NormalBalance: NormalCredit, Credit: MustDecimal("100")},
		{EntryID: uuid.New(), EntryNumber: 2, EntryDate: date, AccountID: cashID, AccountCode: "1.1", AccountName: "Caja", AccountClass: AccountAsset, NormalBalance: NormalDebit, Debit: MustDecimal("121")},
		{EntryID: uuid.New(), EntryNumber: 2, EntryDate: date, AccountID: revenueID, AccountCode: "4.1", AccountName: "Ventas", AccountClass: AccountRevenue, NormalBalance: NormalCredit, Credit: MustDecimal("121")},
		{EntryID: uuid.New(), EntryNumber: 3, EntryDate: date, AccountID: expenseID, AccountCode: "6.1", AccountName: "Gastos", AccountClass: AccountExpense, NormalBalance: NormalDebit, Debit: MustDecimal("21")},
		{EntryID: uuid.New(), EntryNumber: 3, EntryDate: date, AccountID: cashID, AccountCode: "1.1", AccountName: "Caja", AccountClass: AccountAsset, NormalBalance: NormalDebit, Credit: MustDecimal("21")},
	}
	trial := BuildTrialBalance(date, date, lines)
	if !trial.TotalDebit.Equal(trial.TotalCredit) || trial.TotalDebit.String() != "242" {
		t.Fatalf("trial totals = %s/%s", trial.TotalDebit, trial.TotalCredit)
	}
	balance := BuildBalanceSheet(trial)
	if !balance.Difference.IsZero() || balance.TotalAssets.String() != "200" {
		t.Fatalf("balance sheet = %+v", balance)
	}
	income := BuildIncomeStatement(trial)
	if income.NetIncome.String() != "100" {
		t.Fatalf("net income = %s, want 100", income.NetIncome)
	}
	ledger := BuildGeneralLedger(
		Account{ID: cashID, Code: "1.1", Name: "Caja"},
		date,
		date,
		Zero,
		lines,
	)
	if ledger.ClosingBalance.String() != "200" {
		t.Fatalf("cash closing = %s, want 200", ledger.ClosingBalance)
	}
	var csvOutput bytes.Buffer
	if err := WriteTrialBalanceCSV(&csvOutput, trial); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(csvOutput.Bytes(), []byte("TOTAL")) {
		t.Fatalf("CSV output = %s", csvOutput.String())
	}
}

func TestTrialBalanceCarriesPermanentOpeningBalancesOnly(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	before := from.AddDate(0, 0, -1)
	assetID := uuid.New()
	equityID := uuid.New()
	revenueID := uuid.New()
	trial := BuildTrialBalanceWithOpening(from, from, []ReportLine{
		{EntryDate: before, AccountID: assetID, AccountCode: "1.1", AccountName: "Caja", AccountClass: AccountAsset, NormalBalance: NormalDebit, Debit: MustDecimal("150"), Credit: MustDecimal("50")},
		{EntryDate: before, AccountID: equityID, AccountCode: "3.1", AccountName: "Patrimonio", AccountClass: AccountEquity, NormalBalance: NormalCredit, Credit: MustDecimal("100")},
		{EntryDate: before, AccountID: revenueID, AccountCode: "4.1", AccountName: "Venta anterior", AccountClass: AccountRevenue, NormalBalance: NormalCredit, Credit: MustDecimal("999")},
	})
	if len(trial.Rows) != 2 {
		t.Fatalf("opening rows = %d, want 2 permanent accounts", len(trial.Rows))
	}
	if !trial.TotalDebit.Equal(MustDecimal("100")) ||
		!trial.TotalCredit.Equal(MustDecimal("100")) {
		t.Fatalf("opening totals = %s/%s", trial.TotalDebit, trial.TotalCredit)
	}
}

func TestFinancialAccountActivityComesOnlyFromItsLedgerMovements(t *testing.T) {
	t.Parallel()

	financialAccountID := uuid.New()
	ledgerAccountID := uuid.New()
	otherAccountID := uuid.New()
	firstEntryID := uuid.New()
	firstLineID := uuid.New()
	secondEntryID := uuid.New()
	secondLineID := uuid.New()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	activity := BuildFinancialAccountActivity(
		financialAccountID,
		Account{ID: ledgerAccountID, Code: "1.1.01", Name: "Caja"},
		from,
		to,
		MustDecimal("10"),
		[]ReportLine{
			{
				EntryID: firstEntryID, LineID: firstLineID,
				EntryNumber: 1, EntryDate: from.AddDate(0, 0, 2),
				AccountID: ledgerAccountID, Debit: MustDecimal("100"),
			},
			{
				EntryID: secondEntryID, LineID: secondLineID,
				EntryNumber: 2, EntryDate: from.AddDate(0, 0, 3),
				AccountID: ledgerAccountID, Credit: MustDecimal("25"),
			},
			{
				EntryID: uuid.New(), LineID: uuid.New(),
				EntryNumber: 3, EntryDate: from.AddDate(0, 0, 4),
				AccountID: otherAccountID, Debit: MustDecimal("999"),
			},
			{
				EntryID: uuid.New(), LineID: uuid.New(),
				EntryNumber: 4, EntryDate: to.AddDate(0, 0, 1),
				AccountID: ledgerAccountID, Debit: MustDecimal("500"),
			},
		},
	)

	if activity.FinancialAccountID != financialAccountID ||
		activity.LedgerAccountID != ledgerAccountID {
		t.Fatalf("financial activity identity = %+v", activity)
	}
	if len(activity.Movements) != 2 {
		t.Fatalf("financial movements = %d, want 2", len(activity.Movements))
	}
	if activity.Movements[0].EntryID != firstEntryID ||
		activity.Movements[0].LineID != firstLineID ||
		activity.Movements[1].EntryID != secondEntryID ||
		activity.Movements[1].LineID != secondLineID {
		t.Fatalf("financial movement drill-down ids = %+v", activity.Movements)
	}
	if activity.OpeningBalance.String() != "10" ||
		activity.ClosingBalance.String() != "85" {
		t.Fatalf(
			"financial activity balances = %s/%s, want 10/85",
			activity.OpeningBalance,
			activity.ClosingBalance,
		)
	}
}

func TestAgingTreatsAnOpenItemWithoutDueDateAsCurrent(t *testing.T) {
	t.Parallel()

	partyID := uuid.New()
	result := BuildAging(
		time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		[]OpenItem{{
			ID:             uuid.New(),
			PartyID:        partyID,
			Kind:           Receivable,
			OpenFunctional: MustDecimal("121"),
		}},
	)
	if len(result) != 1 ||
		result[0].Buckets.Current.String() != "121" ||
		result[0].Buckets.Total.String() != "121" {
		t.Fatalf("aging without due date = %+v", result)
	}
}

func TestStatementCSVAndOFXNeverUseBinaryMoney(t *testing.T) {
	t.Parallel()

	csvContent := "fecha,descripcion,monto,referencia\n24/07/2026,Transferencia,\"1.234,56\",ABC\n"
	movements, err := ParseStatementCSV(bytes.NewBufferString(csvContent), MustCurrency("ARS"))
	if err != nil {
		t.Fatal(err)
	}
	if len(movements) != 1 || movements[0].Amount.String() != "1234.56" {
		t.Fatalf("CSV movements = %+v", movements)
	}

	ofx := []byte(`<OFX><CURDEF>USD
<STMTTRN><DTPOSTED>20260724120000<TRNAMT>-10.25<FITID>x1<MEMO>Fee</STMTTRN></OFX>`)
	movements, err = ParseStatementOFX(ofx, MustCurrency("ARS"))
	if err != nil {
		t.Fatal(err)
	}
	if movements[0].Amount.String() != "-10.25" || movements[0].Currency.Code() != "USD" {
		t.Fatalf("OFX movement = %+v", movements[0])
	}
}

func TestReconciliationSupportsPartialAndCombinedMatches(t *testing.T) {
	t.Parallel()

	movementID := uuid.New()
	lineOne := uuid.New()
	lineTwo := uuid.New()
	movement := StatementMovement{ID: movementID, Amount: MustDecimal("-100")}
	candidates := map[uuid.UUID]ReconciliationLedgerCandidate{
		lineOne: {JournalLineID: lineOne, Amount: MustDecimal("60")},
		lineTwo: {JournalLineID: lineTwo, Amount: MustDecimal("40")},
	}
	reconciliation := Reconciliation{
		ID:                 uuid.New(),
		FinancialAccountID: uuid.New(),
		PeriodStart:        dateFixture(),
		PeriodEnd:          dateFixture(),
		Matches: []ReconciliationMatch{
			{StatementMovementID: movementID, JournalLineID: lineOne, StatementAmount: MustDecimal("60"), LedgerAmount: MustDecimal("60")},
			{StatementMovementID: movementID, JournalLineID: lineTwo, StatementAmount: MustDecimal("40"), LedgerAmount: MustDecimal("40")},
		},
	}
	if err := reconciliation.Validate(
		map[uuid.UUID]StatementMovement{movementID: movement},
		candidates,
	); err != nil {
		t.Fatal(err)
	}
	reconciliation.Matches[1].StatementAmount = MustDecimal("41")
	reconciliation.Matches[1].LedgerAmount = MustDecimal("41")
	if err := reconciliation.Validate(
		map[uuid.UUID]StatementMovement{movementID: movement},
		candidates,
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("overallocation error = %v", err)
	}
}

func TestInflationProducesReviewableBalancedDraft(t *testing.T) {
	t.Parallel()

	origin := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	closeDate := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	originIndex, err := NewInflationIndex(origin, MustDecimal("100"), "FACPCE", "origin")
	if err != nil {
		t.Fatal(err)
	}
	closingIndex, err := NewInflationIndex(closeDate, MustDecimal("200"), "FACPCE", "closing")
	if err != nil {
		t.Fatal(err)
	}
	assetID := uuid.New()
	recpamID := uuid.New()
	workpaper, err := BuildInflationWorkpaper(
		closeDate,
		MustCurrency("ARS"),
		[]InflationIndex{originIndex, closingIndex},
		[]InflationPosition{{
			AccountID:      assetID,
			AccountCode:    "1.2.01",
			AccountName:    "Mercaderías",
			NormalBalance:  NormalDebit,
			Classification: NonMonetary,
			OriginDate:     origin,
			Balance:        MustDecimal("100"),
		}},
		recpamID,
		"user_1",
		closeDate,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(workpaper.Lines) != 1 || workpaper.Lines[0].Adjustment.String() != "100" {
		t.Fatalf("workpaper = %+v", workpaper)
	}
	entry := workpaper.Draft.ToEntry(EntrySource{
		Type:           "inflation",
		ID:             workpaper.ID,
		Event:          "primary",
		IdempotencyKey: "inflation-post",
	}, "user_1")
	if err := entry.ValidateForPosting(); err != nil {
		t.Fatalf("inflation draft is not postable: %v", err)
	}
}
