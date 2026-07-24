package accounting

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTrialBalanceFilterRejectsInvalidPeriodsClassAndCursor(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	tests := map[string]TrialBalanceFilter{
		"missing from": {To: to},
		"reverse period": {
			From: to,
			To:   from,
		},
		"unknown class": {
			From: from, To: to, AccountClass: AccountClass("memo"),
		},
		"blank cursor code": {
			From: from, To: to,
			Cursor: &TrialBalanceCursor{AccountID: uuid.New()},
		},
		"nil cursor account": {
			From: from, To: to,
			Cursor: &TrialBalanceCursor{Code: "1.1.01"},
		},
		"oversized query": {
			From: from, To: to, Query: strings.Repeat("a", 161),
		},
	}
	for name, filter := range tests {
		filter := filter
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := filter.Validate(); !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("error = %v, want invalid argument", err)
			}
		})
	}
}

func TestListTrialBalanceNormalizesAndForwardsDedicatedQuery(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	from := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	accountID := uuid.New()
	repository.trialBalancePage = TrialBalancePage{
		From: from,
		To:   to,
		Items: []TrialBalanceAccountRow{{
			AccountID:      accountID,
			Code:           "1.1.01",
			Name:           "Caja",
			Class:          AccountAsset,
			NormalBalance:  NormalDebit,
			Path:           []string{"Activo", "Disponibilidades", "Caja"},
			LifecycleState: AccountActive,
			OpeningBalance: MustDecimal("100"),
			Debit:          MustDecimal("25"),
			Credit:         MustDecimal("10"),
			ClosingBalance: MustDecimal("115"),
		}},
		Total: 1,
	}

	page, err := service.ListTrialBalance(
		context.Background(),
		scope,
		TrialBalanceFilter{
			From:         from,
			To:           to,
			Query:        "  caja  ",
			AccountClass: AccountAsset,
			IncludeZero:  true,
			Limit:        500,
		},
	)
	if err != nil {
		t.Fatalf("list trial balance: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 ||
		page.Items[0].AccountID != accountID {
		t.Fatalf("page = %#v", page)
	}
	if repository.trialBalanceCalls != 1 {
		t.Fatalf("repository calls = %d", repository.trialBalanceCalls)
	}
	if repository.lastTrialBalance.Query != "caja" ||
		repository.lastTrialBalance.Limit != 200 ||
		repository.lastTrialBalance.AccountClass != AccountAsset ||
		!repository.lastTrialBalance.IncludeZero {
		t.Fatalf("repository filter = %#v", repository.lastTrialBalance)
	}
}

func TestTrialBalancePageReportSplitsOpeningAndClosingSides(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	report := TrialBalancePageReportTable(TrialBalancePage{
		From: day,
		To:   day,
		Items: []TrialBalanceAccountRow{{
			Code:           "2.1.01",
			Name:           "Proveedores",
			Class:          AccountLiability,
			NormalBalance:  NormalCredit,
			Path:           []string{"Pasivo", "Deudas comerciales", "Proveedores"},
			LifecycleState: AccountArchived,
			OpeningBalance: MustDecimal("-100.25"),
			Debit:          MustDecimal("20"),
			Credit:         MustDecimal("5"),
			ClosingBalance: MustDecimal("-85.25"),
		}},
		Totals: TrialBalanceTotals{
			OpeningCredit: MustDecimal("100.25"),
			Debit:         MustDecimal("20"),
			Credit:        MustDecimal("5"),
			ClosingCredit: MustDecimal("85.25"),
		},
	})
	if err := report.Validate(); err != nil {
		t.Fatalf("validate report: %v", err)
	}
	if report.Title != "Balance de sumas y saldos" || len(report.Rows) != 3 {
		t.Fatalf("report = %#v", report)
	}
	row := report.Rows[0]
	if row[6].Value != "0" || row[7].Value != "100.25" ||
		row[10].Value != "0" || row[11].Value != "85.25" {
		t.Fatalf("balance sides = %#v", row)
	}
	if row[4].Value != "Pasivo > Deudas comerciales > Proveedores" ||
		row[5].Value != "archived" {
		t.Fatalf("account context = %#v", row)
	}
	if report.Rows[2][1].Value != "DIFERENCIA DE CONTROL" {
		t.Fatalf("control row = %#v", report.Rows[2])
	}
}
