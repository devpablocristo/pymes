package accounting

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type ReportLine struct {
	EntryID       uuid.UUID     `json:"entry_id"`
	LineID        uuid.UUID     `json:"line_id"`
	EntryNumber   int64         `json:"entry_number"`
	EntryDate     time.Time     `json:"entry_date"`
	Description   string        `json:"description"`
	SourceType    string        `json:"source_type"`
	SourceID      uuid.UUID     `json:"source_id"`
	AccountID     uuid.UUID     `json:"account_id"`
	AccountCode   string        `json:"account_code"`
	AccountName   string        `json:"account_name"`
	AccountClass  AccountClass  `json:"account_class"`
	NormalBalance NormalBalance `json:"normal_balance"`
	Debit         Decimal       `json:"debit"`
	Credit        Decimal       `json:"credit"`
	PartyID       *uuid.UUID    `json:"party_id,omitempty"`
	LineNo        int           `json:"line_no"`
}

type LedgerLine struct {
	EntryID     uuid.UUID `json:"entry_id"`
	LineID      uuid.UUID `json:"line_id"`
	EntryNumber int64     `json:"entry_number"`
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
	Debit       Decimal   `json:"debit"`
	Credit      Decimal   `json:"credit"`
	Balance     Decimal   `json:"balance"`
}

type GeneralLedger struct {
	Account        Account      `json:"account"`
	From           time.Time    `json:"from"`
	To             time.Time    `json:"to"`
	OpeningBalance Decimal      `json:"opening_balance"`
	ClosingBalance Decimal      `json:"closing_balance"`
	Lines          []LedgerLine `json:"lines"`
}

func BuildGeneralLedger(
	account Account,
	from time.Time,
	to time.Time,
	opening Decimal,
	reportLines []ReportLine,
) GeneralLedger {
	lines := make([]ReportLine, 0, len(reportLines))
	for _, line := range reportLines {
		if line.AccountID == account.ID && !line.EntryDate.Before(from) && !line.EntryDate.After(to) {
			lines = append(lines, line)
		}
	}
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].EntryDate.Equal(lines[j].EntryDate) {
			if lines[i].EntryNumber == lines[j].EntryNumber {
				return lines[i].LineNo < lines[j].LineNo
			}
			return lines[i].EntryNumber < lines[j].EntryNumber
		}
		return lines[i].EntryDate.Before(lines[j].EntryDate)
	})
	running := opening
	result := GeneralLedger{
		Account:        account,
		From:           from,
		To:             to,
		OpeningBalance: opening,
		Lines:          make([]LedgerLine, 0, len(lines)),
	}
	for _, line := range lines {
		running = running.Add(line.Debit).Sub(line.Credit)
		result.Lines = append(result.Lines, LedgerLine{
			EntryID:     line.EntryID,
			LineID:      line.LineID,
			EntryNumber: line.EntryNumber,
			Date:        line.EntryDate,
			Description: line.Description,
			Debit:       line.Debit,
			Credit:      line.Credit,
			Balance:     running,
		})
	}
	result.ClosingBalance = running
	return result
}

type FinancialAccountActivity struct {
	FinancialAccountID uuid.UUID    `json:"financial_account_id"`
	LedgerAccountID    uuid.UUID    `json:"ledger_account_id"`
	From               time.Time    `json:"from"`
	To                 time.Time    `json:"to"`
	OpeningBalance     Decimal      `json:"opening_balance"`
	ClosingBalance     Decimal      `json:"closing_balance"`
	Movements          []LedgerLine `json:"movements"`
}

// BuildFinancialAccountActivity derives the movements and exact balance of a
// cash, bank, card or wallet account from its ledger account. It deliberately
// has no independent balance source: the journal remains authoritative.
func BuildFinancialAccountActivity(
	financialAccountID uuid.UUID,
	ledgerAccount Account,
	from time.Time,
	to time.Time,
	opening Decimal,
	reportLines []ReportLine,
) FinancialAccountActivity {
	ledger := BuildGeneralLedger(ledgerAccount, from, to, opening, reportLines)
	return FinancialAccountActivity{
		FinancialAccountID: financialAccountID,
		LedgerAccountID:    ledgerAccount.ID,
		From:               ledger.From,
		To:                 ledger.To,
		OpeningBalance:     ledger.OpeningBalance,
		ClosingBalance:     ledger.ClosingBalance,
		Movements:          ledger.Lines,
	}
}

type TrialBalanceRow struct {
	AccountID     uuid.UUID     `json:"account_id"`
	Code          string        `json:"code"`
	Name          string        `json:"name"`
	Class         AccountClass  `json:"class"`
	NormalBalance NormalBalance `json:"normal_balance"`
	Debit         Decimal       `json:"debit"`
	Credit        Decimal       `json:"credit"`
	DebitBalance  Decimal       `json:"debit_balance"`
	CreditBalance Decimal       `json:"credit_balance"`
	NetBalance    Decimal       `json:"net_balance"`
}

type TrialBalance struct {
	From          time.Time         `json:"from"`
	AsOf          time.Time         `json:"as_of"`
	Rows          []TrialBalanceRow `json:"rows"`
	TotalDebit    Decimal           `json:"total_debit"`
	TotalCredit   Decimal           `json:"total_credit"`
	TotalDebtor   Decimal           `json:"total_debit_balance"`
	TotalCreditor Decimal           `json:"total_credit_balance"`
}

func BuildTrialBalance(from, asOf time.Time, lines []ReportLine) TrialBalance {
	rows := make(map[uuid.UUID]TrialBalanceRow)
	result := TrialBalance{From: from, AsOf: asOf}
	for _, line := range lines {
		if line.EntryDate.Before(from) || line.EntryDate.After(asOf) {
			continue
		}
		row := rows[line.AccountID]
		row.AccountID = line.AccountID
		row.Code = line.AccountCode
		row.Name = line.AccountName
		row.Class = line.AccountClass
		row.NormalBalance = line.NormalBalance
		row.Debit = row.Debit.Add(line.Debit)
		row.Credit = row.Credit.Add(line.Credit)
		rows[line.AccountID] = row
		result.TotalDebit = result.TotalDebit.Add(line.Debit)
		result.TotalCredit = result.TotalCredit.Add(line.Credit)
	}
	result.Rows = make([]TrialBalanceRow, 0, len(rows))
	for _, row := range rows {
		row.NetBalance = row.Debit.Sub(row.Credit)
		if row.NetBalance.Sign() >= 0 {
			row.DebitBalance = row.NetBalance
			result.TotalDebtor = result.TotalDebtor.Add(row.DebitBalance)
		} else {
			row.CreditBalance = row.NetBalance.Abs()
			result.TotalCreditor = result.TotalCreditor.Add(row.CreditBalance)
		}
		result.Rows = append(result.Rows, row)
	}
	sort.Slice(result.Rows, func(i, j int) bool {
		return result.Rows[i].Code < result.Rows[j].Code
	})
	return result
}

func BuildTrialBalanceWithOpening(
	from,
	asOf time.Time,
	lines []ReportLine,
) TrialBalance {
	current := make([]ReportLine, 0, len(lines))
	opening := make(map[uuid.UUID]ReportLine)
	for _, line := range lines {
		if !line.EntryDate.Before(from) {
			current = append(current, line)
			continue
		}
		if line.AccountClass != AccountAsset &&
			line.AccountClass != AccountLiability &&
			line.AccountClass != AccountEquity {
			continue
		}
		accumulated := opening[line.AccountID]
		accumulated.EntryDate = from
		accumulated.AccountID = line.AccountID
		accumulated.AccountCode = line.AccountCode
		accumulated.AccountName = line.AccountName
		accumulated.AccountClass = line.AccountClass
		accumulated.NormalBalance = line.NormalBalance
		accumulated.Debit = accumulated.Debit.Add(line.Debit)
		accumulated.Credit = accumulated.Credit.Add(line.Credit)
		opening[line.AccountID] = accumulated
	}
	for _, line := range opening {
		net := line.Debit.Sub(line.Credit)
		line.Debit = Zero
		line.Credit = Zero
		if net.Sign() >= 0 {
			line.Debit = net
		} else {
			line.Credit = net.Abs()
		}
		current = append(current, line)
	}
	return BuildTrialBalance(from, asOf, current)
}

type StatementRow struct {
	AccountID uuid.UUID    `json:"account_id"`
	Code      string       `json:"code"`
	Name      string       `json:"name"`
	Class     AccountClass `json:"class"`
	Amount    Decimal      `json:"amount"`
}

type BalanceSheet struct {
	AsOf                 time.Time      `json:"as_of"`
	Assets               []StatementRow `json:"assets"`
	Liabilities          []StatementRow `json:"liabilities"`
	Equity               []StatementRow `json:"equity"`
	TotalAssets          Decimal        `json:"total_assets"`
	TotalLiabilities     Decimal        `json:"total_liabilities"`
	TotalEquity          Decimal        `json:"total_equity"`
	CurrentResult        Decimal        `json:"current_result"`
	LiabilitiesAndEquity Decimal        `json:"liabilities_and_equity"`
	Difference           Decimal        `json:"difference"`
}

func BuildBalanceSheet(trial TrialBalance) BalanceSheet {
	statement := BalanceSheet{AsOf: trial.AsOf}
	for _, row := range trial.Rows {
		switch row.Class {
		case AccountAsset:
			amount := row.Debit.Sub(row.Credit)
			statement.Assets = append(statement.Assets, statementRow(row, amount))
			statement.TotalAssets = statement.TotalAssets.Add(amount)
		case AccountLiability:
			amount := row.Credit.Sub(row.Debit)
			statement.Liabilities = append(statement.Liabilities, statementRow(row, amount))
			statement.TotalLiabilities = statement.TotalLiabilities.Add(amount)
		case AccountEquity:
			amount := row.Credit.Sub(row.Debit)
			statement.Equity = append(statement.Equity, statementRow(row, amount))
			statement.TotalEquity = statement.TotalEquity.Add(amount)
		case AccountRevenue:
			statement.CurrentResult = statement.CurrentResult.Add(row.Credit.Sub(row.Debit))
		case AccountCost, AccountExpense:
			statement.CurrentResult = statement.CurrentResult.Sub(row.Debit.Sub(row.Credit))
		}
	}
	statement.LiabilitiesAndEquity = statement.TotalLiabilities.
		Add(statement.TotalEquity).
		Add(statement.CurrentResult)
	statement.Difference = statement.TotalAssets.Sub(statement.LiabilitiesAndEquity)
	return statement
}

type IncomeStatement struct {
	From          time.Time      `json:"from"`
	To            time.Time      `json:"to"`
	Revenue       []StatementRow `json:"revenue"`
	Costs         []StatementRow `json:"costs"`
	Expenses      []StatementRow `json:"expenses"`
	TotalRevenue  Decimal        `json:"total_revenue"`
	TotalCosts    Decimal        `json:"total_costs"`
	GrossProfit   Decimal        `json:"gross_profit"`
	TotalExpenses Decimal        `json:"total_expenses"`
	NetIncome     Decimal        `json:"net_income"`
}

func BuildIncomeStatement(trial TrialBalance) IncomeStatement {
	statement := IncomeStatement{From: trial.From, To: trial.AsOf}
	for _, row := range trial.Rows {
		switch row.Class {
		case AccountRevenue:
			amount := row.Credit.Sub(row.Debit)
			statement.Revenue = append(statement.Revenue, statementRow(row, amount))
			statement.TotalRevenue = statement.TotalRevenue.Add(amount)
		case AccountCost:
			amount := row.Debit.Sub(row.Credit)
			statement.Costs = append(statement.Costs, statementRow(row, amount))
			statement.TotalCosts = statement.TotalCosts.Add(amount)
		case AccountExpense:
			amount := row.Debit.Sub(row.Credit)
			statement.Expenses = append(statement.Expenses, statementRow(row, amount))
			statement.TotalExpenses = statement.TotalExpenses.Add(amount)
		}
	}
	statement.GrossProfit = statement.TotalRevenue.Sub(statement.TotalCosts)
	statement.NetIncome = statement.GrossProfit.Sub(statement.TotalExpenses)
	return statement
}

type AgingBucket struct {
	Current    Decimal `json:"current"`
	Days1To30  Decimal `json:"days_1_to_30"`
	Days31To60 Decimal `json:"days_31_to_60"`
	Days61To90 Decimal `json:"days_61_to_90"`
	Over90     Decimal `json:"over_90"`
	Total      Decimal `json:"total"`
}

type PartyAging struct {
	PartyID uuid.UUID    `json:"party_id"`
	Kind    OpenItemKind `json:"kind"`
	Buckets AgingBucket  `json:"buckets"`
}

func BuildAging(asOf time.Time, items []OpenItem) []PartyAging {
	byParty := make(map[uuid.UUID]PartyAging)
	for _, item := range items {
		if item.OpenFunctional.IsZero() {
			continue
		}
		aging := byParty[item.PartyID]
		aging.PartyID = item.PartyID
		aging.Kind = item.Kind
		days := 0
		if !item.DueDate.IsZero() {
			days = int(asOf.Sub(item.DueDate).Hours() / 24)
		}
		switch {
		case days <= 0:
			aging.Buckets.Current = aging.Buckets.Current.Add(item.OpenFunctional)
		case days <= 30:
			aging.Buckets.Days1To30 = aging.Buckets.Days1To30.Add(item.OpenFunctional)
		case days <= 60:
			aging.Buckets.Days31To60 = aging.Buckets.Days31To60.Add(item.OpenFunctional)
		case days <= 90:
			aging.Buckets.Days61To90 = aging.Buckets.Days61To90.Add(item.OpenFunctional)
		default:
			aging.Buckets.Over90 = aging.Buckets.Over90.Add(item.OpenFunctional)
		}
		aging.Buckets.Total = aging.Buckets.Total.Add(item.OpenFunctional)
		byParty[item.PartyID] = aging
	}
	result := make([]PartyAging, 0, len(byParty))
	for _, aging := range byParty {
		result = append(result, aging)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].PartyID.String() < result[j].PartyID.String()
	})
	return result
}

func WriteTrialBalanceCSV(writer io.Writer, trial TrialBalance) error {
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()
	if err := csvWriter.Write([]string{
		"account_id",
		"code",
		"name",
		"class",
		"debit",
		"credit",
		"debit_balance",
		"credit_balance",
	}); err != nil {
		return fmt.Errorf("accounting: write trial balance CSV header: %w", err)
	}
	for _, row := range trial.Rows {
		if err := csvWriter.Write([]string{
			row.AccountID.String(),
			row.Code,
			row.Name,
			string(row.Class),
			row.Debit.String(),
			row.Credit.String(),
			row.DebitBalance.String(),
			row.CreditBalance.String(),
		}); err != nil {
			return fmt.Errorf("accounting: write trial balance CSV row: %w", err)
		}
	}
	if err := csvWriter.Write([]string{
		"",
		"",
		"TOTAL",
		"",
		trial.TotalDebit.String(),
		trial.TotalCredit.String(),
		trial.TotalDebtor.String(),
		trial.TotalCreditor.String(),
	}); err != nil {
		return fmt.Errorf("accounting: write trial balance CSV total: %w", err)
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("accounting: flush trial balance CSV: %w", err)
	}
	return nil
}

func statementRow(row TrialBalanceRow, amount Decimal) StatementRow {
	return StatementRow{
		AccountID: row.AccountID,
		Code:      row.Code,
		Name:      row.Name,
		Class:     row.Class,
		Amount:    amount,
	}
}

func encodeReportCursor(number int64) string {
	if number <= 0 {
		return ""
	}
	return strconv.FormatInt(number, 10)
}
