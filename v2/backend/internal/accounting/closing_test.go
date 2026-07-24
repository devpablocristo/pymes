package accounting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAnnualClosingZerosTemporaryAccountsAndTransfersExactProfit(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	revenueID := uuid.New()
	expenseID := uuid.New()
	cashID := uuid.New()
	resultID := uuid.New()
	trial := BuildTrialBalance(from, to, []ReportLine{
		{EntryDate: to, AccountID: cashID, AccountCode: "1.1", AccountName: "Caja", AccountClass: AccountAsset, NormalBalance: NormalDebit, Debit: MustDecimal("60")},
		{EntryDate: to, AccountID: expenseID, AccountCode: "6.1", AccountName: "Gastos", AccountClass: AccountExpense, NormalBalance: NormalDebit, Debit: MustDecimal("40")},
		{EntryDate: to, AccountID: revenueID, AccountCode: "4.1", AccountName: "Ventas", AccountClass: AccountRevenue, NormalBalance: NormalCredit, Credit: MustDecimal("100")},
	})
	workpaper, err := BuildAnnualClosingWorkpaper(
		trial,
		MustCurrency("ARS"),
		resultID,
		"contador_1",
		"annual-close-2026",
		to,
	)
	if err != nil {
		t.Fatal(err)
	}
	if workpaper.NetIncome.String() != "60" {
		t.Fatalf("net income = %s, want 60", workpaper.NetIncome)
	}
	if len(workpaper.Lines) != 3 {
		t.Fatalf("closing line count = %d, want 3", len(workpaper.Lines))
	}
	entry := workpaper.Draft.ToEntry(EntrySource{
		Type:           "annual_closing",
		ID:             workpaper.Draft.ID,
		Event:          "primary",
		IdempotencyKey: workpaper.Draft.IdempotencyKey,
	}, "contador_1")
	if err := entry.ValidateForPosting(); err != nil {
		t.Fatalf("annual closing draft is not postable: %v", err)
	}
	assertLine(t, entry.Lines, revenueID, "100", "0")
	assertLine(t, entry.Lines, expenseID, "0", "40")
	assertLine(t, entry.Lines, resultID, "0", "60")
}

func TestAnnualClosingTransfersLossToResultAccountDebit(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	revenueID := uuid.New()
	expenseID := uuid.New()
	cashID := uuid.New()
	resultID := uuid.New()
	trial := BuildTrialBalance(from, to, []ReportLine{
		{EntryDate: to, AccountID: revenueID, AccountCode: "4.1", AccountName: "Ventas", AccountClass: AccountRevenue, NormalBalance: NormalCredit, Credit: MustDecimal("30")},
		{EntryDate: to, AccountID: expenseID, AccountCode: "6.1", AccountName: "Gastos", AccountClass: AccountExpense, NormalBalance: NormalDebit, Debit: MustDecimal("50")},
		{EntryDate: to, AccountID: cashID, AccountCode: "1.1", AccountName: "Caja", AccountClass: AccountAsset, NormalBalance: NormalDebit, Credit: MustDecimal("20")},
	})
	workpaper, err := BuildAnnualClosingWorkpaper(
		trial,
		MustCurrency("ARS"),
		resultID,
		"contador_1",
		"annual-close-loss",
		to,
	)
	if err != nil {
		t.Fatal(err)
	}
	if workpaper.NetIncome.String() != "-20" {
		t.Fatalf("net income = %s, want -20", workpaper.NetIncome)
	}
	assertLine(t, workpaper.Draft.Lines, resultID, "20", "0")
}

func TestAnnualClosingRejectsUnbalancedTrial(t *testing.T) {
	t.Parallel()

	trial := TrialBalance{
		From:        dateFixture(),
		AsOf:        dateFixture(),
		TotalDebit:  MustDecimal("1"),
		TotalCredit: Zero,
	}
	_, err := BuildAnnualClosingDraft(
		trial,
		MustCurrency("ARS"),
		uuid.New(),
		"actor",
		"annual-close",
		dateFixture(),
	)
	if !errors.Is(err, ErrUnbalancedEntry) {
		t.Fatalf("unbalanced trial error = %v", err)
	}
}

func TestServiceCreatesAnnualClosingDraftIdempotently(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	resultAccount := Account{
		ID:            uuid.New(),
		Code:          "3.1.03",
		Name:          "Resultado del ejercicio",
		Class:         AccountEquity,
		NormalBalance: NormalCredit,
		Monetary:      NonMonetary,
		Postable:      true,
		Version:       1,
	}
	repository.accounts[resultAccount.ID] = resultAccount
	repository.mappings[RoleCurrentResult] = AccountMapping{
		Role:      RoleCurrentResult,
		AccountID: resultAccount.ID,
		Version:   1,
	}
	revenueID := uuid.New()
	cashID := uuid.New()
	repository.reportLines = []ReportLine{
		{EntryDate: to, AccountID: cashID, AccountCode: "1.1", AccountName: "Caja", AccountClass: AccountAsset, NormalBalance: NormalDebit, Debit: MustDecimal("100")},
		{EntryDate: to, AccountID: revenueID, AccountCode: "4.1", AccountName: "Ventas", AccountClass: AccountRevenue, NormalBalance: NormalCredit, Credit: MustDecimal("100")},
	}
	command := AnnualClosingCommand{
		From:               from,
		To:                 to,
		FunctionalCurrency: MustCurrency("ARS"),
		IdempotencyKey:     "annual-close-2026",
	}
	first, err := service.CreateAnnualClosingDraft(context.Background(), scope, command)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateAnnualClosingDraft(context.Background(), scope, command)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("annual closing replay IDs = %s/%s", first.ID, second.ID)
	}
	if len(repository.drafts) != 1 {
		t.Fatalf("draft count = %d, want 1", len(repository.drafts))
	}
	command.To = to.AddDate(1, 0, 0)
	if _, err := service.CreateAnnualClosingDraft(context.Background(), scope, command); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("idempotency reuse error = %v", err)
	}
}
