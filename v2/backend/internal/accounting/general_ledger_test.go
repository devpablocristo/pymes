package accounting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestListGeneralLedgerAllowsArchivedPostingAccounts(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	accountID := uuid.New()
	archivedAt := dateFixture()
	repository.accounts[accountID] = Account{
		ID:            accountID,
		Code:          "1.1.01",
		Name:          "Caja histórica",
		Class:         AccountAsset,
		NormalBalance: NormalDebit,
		Monetary:      Monetary,
		Postable:      true,
		NodeType:      AccountPosting,
		ArchivedAt:    &archivedAt,
		Version:       1,
	}
	repository.generalLedgerPage = GeneralLedgerPage{
		OpeningBalance: MustDecimal("15"),
		ClosingBalance: MustDecimal("40"),
		TotalDebit:     MustDecimal("25"),
		Items:          []GeneralLedgerMovement{},
	}
	from := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)

	ledger, err := service.ListGeneralLedger(context.Background(), scope, GeneralLedgerFilter{
		AccountID: accountID,
		From:      from,
		To:        to,
	})
	if err != nil {
		t.Fatalf("list archived account ledger: %v", err)
	}
	if ledger.Account.ID != accountID || ledger.ClosingBalance.String() != "40" {
		t.Fatalf("ledger = %#v", ledger)
	}
	if repository.generalLedgerCalls != 1 || repository.lastGeneralLedger.Limit != 50 {
		t.Fatalf("repository filter = %#v, calls = %d", repository.lastGeneralLedger, repository.generalLedgerCalls)
	}
}

func TestListGeneralLedgerRejectsGroupsAndTrashedAccounts(t *testing.T) {
	t.Parallel()

	for name, fixture := range map[string]Account{
		"group": {
			ID: uuid.New(), Code: "1.1", Name: "Disponibilidades",
			Class: AccountAsset, NormalBalance: NormalDebit,
			Monetary: NotApplicable, NodeType: AccountGroup, Version: 1,
		},
		"trashed posting": func() Account {
			trashedAt := dateFixture()
			return Account{
				ID: uuid.New(), Code: "1.1.01", Name: "Caja eliminada",
				Class: AccountAsset, NormalBalance: NormalDebit,
				Monetary: Monetary, Postable: true, NodeType: AccountPosting,
				TrashedAt: &trashedAt, Version: 1,
			}
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repository, service, scope := serviceFixture(t)
			repository.accounts[fixture.ID] = fixture
			_, err := service.ListGeneralLedger(context.Background(), scope, GeneralLedgerFilter{
				AccountID: fixture.ID,
				From:      time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
				To:        time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC),
			})
			want := ErrAccountNotPostable
			if name == "trashed posting" {
				want = ErrNotFound
			}
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
			if repository.generalLedgerCalls != 0 {
				t.Fatalf("group/trashed account queried the ledger %d times", repository.generalLedgerCalls)
			}
		})
	}
}

func TestGeneralLedgerPageReportKeepsDebitAndCreditSidesExplicit(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
	report := GeneralLedgerPageReportTable(GeneralLedgerPage{
		Account:        Account{Code: "2.1.01", Name: "Proveedores"},
		From:           day,
		To:             day,
		OpeningBalance: MustDecimal("-50"),
		ClosingBalance: MustDecimal("-75"),
		Items: []GeneralLedgerMovement{{
			Date: day, EntryNumber: 12, Reference: "FC-12", Origin: "purchase",
			Description: "Compra", Memo: "Mercadería", Credit: MustDecimal("25"),
			Balance: MustDecimal("-75"),
		}},
	})
	if err := report.Validate(); err != nil {
		t.Fatalf("validate general ledger report: %v", err)
	}
	if len(report.Rows) != 3 || report.Rows[1][7].Value != "0" || report.Rows[1][8].Value != "75" {
		t.Fatalf("report rows = %#v", report.Rows)
	}
	if report.Rows[1][4].Value != "Compra · Mercadería" {
		t.Fatalf("report detail = %q", report.Rows[1][4].Value)
	}
}
