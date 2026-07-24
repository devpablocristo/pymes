package httpserver

import (
	"testing"

	"github.com/devpablocristo/pymes/v2/backend/internal/accounting"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/google/uuid"
)

func TestAccountingAccountDirectoryUsesParentIDsAndNaturalOrder(t *testing.T) {
	t.Parallel()

	rootID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	twoID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	tenID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	details := []accounting.AccountDetail{
		accountDetailFixture(tenID, "1.10", &rootID, true),
		accountDetailFixture(rootID, "1", nil, false),
		accountDetailFixture(twoID, "1.2", &rootID, true),
	}
	directory, err := accountingAccountDirectoryRows(details)
	if err != nil {
		t.Fatalf("accountingAccountDirectoryRows: %v", err)
	}
	included := map[uuid.UUID]struct{}{
		rootID: {},
		twoID:  {},
		tenID:  {},
	}
	ordered, err := preorderAccountingDirectory(directory, included)
	if err != nil {
		t.Fatalf("preorderAccountingDirectory: %v", err)
	}
	got := make([]string, 0, len(ordered))
	for _, item := range ordered {
		got = append(got, item.detail.Account.Code)
	}
	want := []string{"1", "1.2", "1.10"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("codes = %v, want %v", got, want)
		}
	}
	if len(ordered[2].path) != 2 ||
		ordered[2].path[0] != rootID ||
		ordered[2].path[1] != tenID {
		t.Fatalf("path = %v", ordered[2].path)
	}
}

func TestAccountingAccountDirectoryRejectsCycles(t *testing.T) {
	t.Parallel()

	leftID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	rightID := uuid.MustParse("10000000-0000-0000-0000-000000000002")
	_, err := accountingAccountDirectoryRows([]accounting.AccountDetail{
		accountDetailFixture(leftID, "1", &rightID, false),
		accountDetailFixture(rightID, "2", &leftID, false),
	})
	if err != errAccountingAccountHierarchyCycle {
		t.Fatalf("error = %v, want hierarchy cycle", err)
	}
}

func TestAccountCommandRequiresConsistentNodeType(t *testing.T) {
	t.Parallel()

	debit := api.AccountingNormalBalanceDebit
	parentID := uuid.MustParse("20000000-0000-0000-0000-000000000001")
	postable := true
	command, err := accountCommand(api.AccountingAccountInput{
		AccountType:            api.Asset,
		Code:                   "1.1.01",
		MonetaryClassification: api.Monetary,
		Name:                   "Caja",
		NodeType:               api.Posting,
		NormalBalance:          debit,
		ParentId:               &parentID,
		Postable:               &postable,
	})
	if err != nil {
		t.Fatalf("accountCommand posting: %v", err)
	}
	if !command.Postable || command.NodeType != accounting.AccountPosting {
		t.Fatalf("command = %+v", command)
	}

	groupPostable := true
	if _, err := accountCommand(api.AccountingAccountInput{
		AccountType:            api.Asset,
		Code:                   "1.1",
		MonetaryClassification: api.NotApplicable,
		Name:                   "Disponibilidades",
		NodeType:               api.Group,
		NormalBalance:          debit,
		Postable:               &groupPostable,
	}); err == nil {
		t.Fatal("inconsistent group/postable input was accepted")
	}
}

func accountDetailFixture(
	id uuid.UUID,
	code string,
	parentID *uuid.UUID,
	postable bool,
) accounting.AccountDetail {
	monetary := accounting.NotApplicable
	if postable {
		monetary = accounting.Monetary
	}
	account := accounting.Account{
		ID:            id,
		Code:          code,
		Name:          code,
		Class:         accounting.AccountAsset,
		NormalBalance: accounting.NormalDebit,
		Monetary:      monetary,
		ParentID:      parentID,
		Postable:      postable,
		NodeType:      accounting.AccountGroup,
		Version:       1,
	}
	if postable {
		account.NodeType = accounting.AccountPosting
	}
	return accounting.AccountDetail{
		Account:      account,
		Capabilities: accounting.BuildAccountCapabilities(account, accounting.AccountUsage{}),
	}
}
