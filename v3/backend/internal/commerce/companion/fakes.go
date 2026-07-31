package companion

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
)

// FakeFiscal and FakeAccounting are deterministic contract fakes for local
// development and use-case tests; they are not production adapters.
type FakeFiscal struct {
	mu               sync.Mutex
	byVoucher        map[string]domain.FiscalResult
	LoseAfterPersist bool
}

func NewFakeFiscal() *FakeFiscal { return &FakeFiscal{byVoucher: make(map[string]domain.FiscalResult)} }
func (f *FakeFiscal) Authorize(_ context.Context, request domain.FiscalRequest) (domain.FiscalResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fiscalKey(request)
	if prior, ok := f.byVoucher[key]; ok {
		return prior, nil
	}
	result := domain.FiscalResult{RequestID: request.RequestID, OrganizationID: request.OrganizationID, Status: "authorized", CAE: fmt.Sprintf("CAE-%d", request.Voucher.VoucherNumber), SnapshotDigest: request.SnapshotDigest, CorrelationID: request.CorrelationID}
	f.byVoucher[key] = result
	if f.LoseAfterPersist {
		f.LoseAfterPersist = false
		return domain.FiscalResult{RequestID: request.RequestID, Status: "uncertain"}, nil
	}
	return result, nil
}
func (f *FakeFiscal) Consult(_ context.Context, request domain.FiscalRequest) (domain.FiscalResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result, ok := f.byVoucher[fiscalKey(request)]
	if !ok {
		return domain.FiscalResult{RequestID: request.RequestID, Status: "not_found"}, nil
	}
	return result, nil
}
func fiscalKey(r domain.FiscalRequest) string {
	return fmt.Sprintf("%s/%d/%s/%d", r.OrganizationID, r.Voucher.PointOfSale, r.Voucher.DocumentType, r.Voucher.VoucherNumber)
}

type FakeAccounting struct {
	mu               sync.Mutex
	byCommand        map[string]domain.AccountingEvent
	LoseAfterPersist bool
	sequence         int
}

func NewFakeAccounting() *FakeAccounting {
	return &FakeAccounting{byCommand: make(map[string]domain.AccountingEvent)}
}
func (f *FakeAccounting) Post(_ context.Context, command domain.PostingCommand) (domain.AccountingEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := command.OrganizationID + ":" + command.CommandID
	if prior, ok := f.byCommand[key]; ok {
		prior.Status = "duplicate"
		return prior, nil
	}
	if !balanced(command.Lines) {
		return domain.AccountingEvent{}, errors.New("UNBALANCED_POSTING")
	}
	f.sequence++
	result := domain.AccountingEvent{CommandID: command.CommandID, OrganizationID: command.OrganizationID, Status: "posted", JournalEntryID: fmt.Sprintf("je-%d", f.sequence)}
	for index, line := range command.Lines {
		if line.OpenItem {
			result.OpenItemIDs = append(result.OpenItemIDs, fmt.Sprintf("oi-%d-%d", f.sequence, index))
		}
	}
	f.byCommand[key] = result
	if f.LoseAfterPersist {
		f.LoseAfterPersist = false
		return domain.AccountingEvent{}, errors.New("response lost")
	}
	return result, nil
}

func (f *FakeAccounting) Reverse(_ context.Context, command domain.ReversalCommand) (domain.AccountingEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := command.OrganizationID + ":reversal:" + command.CommandID
	if prior, ok := f.byCommand[key]; ok {
		prior.Status = "duplicate"
		return prior, nil
	}
	f.sequence++
	result := domain.AccountingEvent{CommandID: command.CommandID, OrganizationID: command.OrganizationID, Status: "reversed", JournalEntryID: fmt.Sprintf("je-%d", f.sequence)}
	f.byCommand[key] = result
	return result, nil
}

func (f *FakeAccounting) ApplyOpenItem(_ context.Context, command domain.AccountingApplicationCommand) (domain.AccountingEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := command.OrganizationID + ":application:" + command.CommandID
	if prior, ok := f.byCommand[key]; ok {
		prior.Status = "duplicate"
		return prior, nil
	}
	if !command.Amount.Valid() || command.Amount.Amount == "0" || command.DebitOpenItemID == "" || command.CreditOpenItemID == "" {
		return domain.AccountingEvent{}, errors.New("VALIDATION_ERROR")
	}
	f.sequence++
	result := domain.AccountingEvent{CommandID: command.CommandID, OrganizationID: command.OrganizationID, Status: "applied", ApplicationID: fmt.Sprintf("app-%d", f.sequence)}
	f.byCommand[key] = result
	return result, nil
}

func (f *FakeAccounting) ReverseOpenItemApplication(_ context.Context, command domain.AccountingApplicationReversalCommand) (domain.AccountingEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := command.OrganizationID + ":application-reversal:" + command.CommandID
	if prior, ok := f.byCommand[key]; ok {
		prior.Status = "duplicate"
		return prior, nil
	}
	f.sequence++
	result := domain.AccountingEvent{CommandID: command.CommandID, OrganizationID: command.OrganizationID, Status: "reversed", ApplicationID: command.ApplicationID}
	f.byCommand[key] = result
	return result, nil
}
func balanced(lines []domain.PostingLine) bool {
	if len(lines) < 2 {
		return false
	}
	balances := map[string]*big.Rat{}
	for _, line := range lines {
		if line.AccountCode == "" || !line.Debit.Valid() || !line.Credit.Valid() || line.Debit.Currency != line.Credit.Currency {
			return false
		}
		debit, ok := new(big.Rat).SetString(line.Debit.Amount)
		if !ok {
			return false
		}
		credit, ok := new(big.Rat).SetString(line.Credit.Amount)
		if !ok {
			return false
		}
		if balances[line.Debit.Currency] == nil {
			balances[line.Debit.Currency] = new(big.Rat)
		}
		balances[line.Debit.Currency].Add(balances[line.Debit.Currency], debit)
		balances[line.Debit.Currency].Sub(balances[line.Debit.Currency], credit)
	}
	for _, balance := range balances {
		if balance.Sign() != 0 {
			return false
		}
	}
	return true
}
