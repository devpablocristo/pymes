package accounting

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AccountClass string

const (
	AccountAsset     AccountClass = "asset"
	AccountLiability AccountClass = "liability"
	AccountEquity    AccountClass = "equity"
	AccountRevenue   AccountClass = "revenue"
	AccountCost      AccountClass = "cost"
	AccountExpense   AccountClass = "expense"
)

func (c AccountClass) Valid() bool {
	switch c {
	case AccountAsset, AccountLiability, AccountEquity, AccountRevenue, AccountCost, AccountExpense:
		return true
	default:
		return false
	}
}

type NormalBalance string

const (
	NormalDebit  NormalBalance = "debit"
	NormalCredit NormalBalance = "credit"
)

func (n NormalBalance) Valid() bool {
	return n == NormalDebit || n == NormalCredit
}

func DefaultNormalBalance(class AccountClass) NormalBalance {
	switch class {
	case AccountAsset, AccountCost, AccountExpense:
		return NormalDebit
	case AccountLiability, AccountEquity, AccountRevenue:
		return NormalCredit
	default:
		return ""
	}
}

type MonetaryClassification string

const (
	Monetary      MonetaryClassification = "monetary"
	NonMonetary   MonetaryClassification = "non_monetary"
	NotApplicable MonetaryClassification = "not_applicable"
)

func (c MonetaryClassification) Valid() bool {
	return c == Monetary || c == NonMonetary || c == NotApplicable
}

type AccountNodeType string

const (
	AccountGroup   AccountNodeType = "group"
	AccountPosting AccountNodeType = "posting"
)

func (n AccountNodeType) Valid() bool {
	return n == AccountGroup || n == AccountPosting
}

type AccountLifecycleState string

const (
	AccountActive   AccountLifecycleState = "active"
	AccountArchived AccountLifecycleState = "archived"
	AccountTrashed  AccountLifecycleState = "trashed"
)

type Currency struct {
	code       string
	minorUnits int
}

func NewCurrency(code string) (Currency, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 3 {
		return Currency{}, fmt.Errorf("%w: currency must be a three-letter ISO code", ErrInvalidArgument)
	}
	for _, char := range code {
		if char < 'A' || char > 'Z' {
			return Currency{}, fmt.Errorf("%w: invalid currency code", ErrInvalidArgument)
		}
	}
	minorUnits := 2
	switch code {
	case "JPY", "CLP", "PYG", "UYI":
		minorUnits = 0
	case "BHD", "IQD", "JOD", "KWD", "OMR", "TND":
		minorUnits = 3
	}
	return Currency{code: code, minorUnits: minorUnits}, nil
}

func MustCurrency(code string) Currency {
	currency, err := NewCurrency(code)
	if err != nil {
		panic(err)
	}
	return currency
}

func (c Currency) Code() string {
	if c.code == "" {
		return "ARS"
	}
	return c.code
}

func (c Currency) MinorUnits() int {
	if c.code == "" {
		return 2
	}
	return c.minorUnits
}

func (c Currency) Round(amount Decimal) Decimal {
	return amount.Round(c.MinorUnits())
}

func (c Currency) MarshalJSON() ([]byte, error) {
	return []byte(`"` + c.Code() + `"`), nil
}

func (c *Currency) UnmarshalJSON(data []byte) error {
	if len(data) != 5 || data[0] != '"' || data[4] != '"' {
		return fmt.Errorf("%w: invalid currency JSON value", ErrInvalidArgument)
	}
	parsed, err := NewCurrency(string(data[1:4]))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

type Scope struct {
	OrganizationID      uuid.UUID
	ActorID             string
	CanPostAdjustments  bool
	CanReopenPeriods    bool
	CanManageAccounting bool
}

func (s Scope) Validate() error {
	if s.OrganizationID == uuid.Nil {
		return fmt.Errorf("%w: organization is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(s.ActorID) == "" {
		return fmt.Errorf("%w: actor is required", ErrInvalidArgument)
	}
	return nil
}

type Account struct {
	ID                    uuid.UUID              `json:"id"`
	Code                  string                 `json:"code"`
	Name                  string                 `json:"name"`
	Class                 AccountClass           `json:"class"`
	NormalBalance         NormalBalance          `json:"normal_balance"`
	Monetary              MonetaryClassification `json:"monetary_classification"`
	ParentID              *uuid.UUID             `json:"parent_id,omitempty"`
	Postable              bool                   `json:"postable"`
	NodeType              AccountNodeType        `json:"node_type"`
	SystemKey             string                 `json:"-"`
	SystemManaged         bool                   `json:"system_managed"`
	ArchivedAt            *time.Time             `json:"archived_at,omitempty"`
	TrashedAt             *time.Time             `json:"trashed_at,omitempty"`
	Version               int64                  `json:"version"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
	OpeningBalanceDate    *time.Time             `json:"opening_balance_date,omitempty"`
	InflationOriginPolicy string                 `json:"inflation_origin_policy,omitempty"`
}

func (a Account) LifecycleState() AccountLifecycleState {
	if a.TrashedAt != nil {
		return AccountTrashed
	}
	if a.ArchivedAt != nil {
		return AccountArchived
	}
	return AccountActive
}

func (a Account) EffectiveNodeType() AccountNodeType {
	if a.Postable {
		return AccountPosting
	}
	return AccountGroup
}

func (a Account) Validate() error {
	if strings.TrimSpace(a.Code) == "" || len(a.Code) > 32 {
		return fmt.Errorf("%w: account code is required and must not exceed 32 characters", ErrInvalidArgument)
	}
	if strings.TrimSpace(a.Name) == "" || len(a.Name) > 160 {
		return fmt.Errorf("%w: account name is required and must not exceed 160 characters", ErrInvalidArgument)
	}
	if !a.Class.Valid() {
		return fmt.Errorf("%w: invalid account class", ErrInvalidArgument)
	}
	if !a.NormalBalance.Valid() {
		return fmt.Errorf("%w: invalid normal balance", ErrInvalidArgument)
	}
	if !a.Monetary.Valid() {
		return fmt.Errorf("%w: invalid monetary classification", ErrInvalidArgument)
	}
	if a.NodeType != "" && a.NodeType != a.EffectiveNodeType() {
		return fmt.Errorf("%w: node type and postable must agree", ErrInvalidArgument)
	}
	if a.Postable && a.ParentID == nil {
		return fmt.Errorf("%w: posting accounts require a parent group", ErrInvalidAccountParent)
	}
	if a.Postable && a.Monetary == NotApplicable {
		return fmt.Errorf("%w: posting accounts require a monetary classification", ErrInvalidArgument)
	}
	if !a.Postable && a.Monetary != NotApplicable {
		return fmt.Errorf("%w: groups use not_applicable classification", ErrInvalidArgument)
	}
	if a.ParentID != nil && *a.ParentID == a.ID && a.ID != uuid.Nil {
		return fmt.Errorf("%w: account cannot be its own parent", ErrInvalidArgument)
	}
	return nil
}

type AccountUsage struct {
	JournalLines            int64 `json:"journal_lines"`
	DraftLines              int64 `json:"draft_lines"`
	Mappings                int64 `json:"mappings"`
	Children                int64 `json:"children"`
	ActiveChildren          int64 `json:"active_children"`
	FinancialAccounts       int64 `json:"financial_accounts"`
	ActiveFinancialAccounts int64 `json:"active_financial_accounts"`
	OpenItems               int64 `json:"open_items"`
	InflationLines          int64 `json:"inflation_lines"`
	RevaluationLines        int64 `json:"revaluation_lines"`
	InactiveAncestors       int64 `json:"inactive_ancestors"`
}

func (u AccountUsage) HasDependencies() bool {
	return u.JournalLines+u.DraftLines+u.Mappings+u.Children+
		u.FinancialAccounts+u.OpenItems+u.InflationLines+
		u.RevaluationLines > 0
}

func (u AccountUsage) StructureLocked() bool {
	return u.HasDependencies()
}

type AccountCapabilities struct {
	CanEditName      bool     `json:"can_edit_name"`
	CanEditStructure bool     `json:"can_edit_structure"`
	CanArchive       bool     `json:"can_archive"`
	CanTrash         bool     `json:"can_trash"`
	CanRestore       bool     `json:"can_restore"`
	CanDuplicate     bool     `json:"can_duplicate"`
	EditBlockers     []string `json:"edit_blockers"`
	ArchiveBlockers  []string `json:"archive_blockers"`
	TrashBlockers    []string `json:"trash_blockers"`
	RestoreBlockers  []string `json:"restore_blockers"`
}

const (
	BlockerAccountStructureLocked     = "ACCOUNTING_ACCOUNT_STRUCTURE_LOCKED"
	BlockerAccountParentInvalid       = "ACCOUNTING_ACCOUNT_PARENT_INVALID"
	BlockerAccountHierarchyCycle      = "ACCOUNTING_ACCOUNT_HIERARCHY_CYCLE"
	BlockerMappingIncompatible        = "ACCOUNTING_MAPPING_INCOMPATIBLE"
	BlockerAccountMapped              = "ACCOUNTING_ACCOUNT_MAPPED"
	BlockerAccountFinancialDependency = "ACCOUNTING_ACCOUNT_FINANCIAL_DEPENDENCY"
	BlockerAccountHasActiveChildren   = "ACCOUNTING_ACCOUNT_HAS_ACTIVE_CHILDREN"
	BlockerAccountParentInactive      = "ACCOUNTING_ACCOUNT_PARENT_INACTIVE"
	BlockerAccountProtected           = "ACCOUNTING_ACCOUNT_PROTECTED"
)

func BuildAccountCapabilities(
	account Account,
	usage AccountUsage,
) AccountCapabilities {
	capabilities := AccountCapabilities{
		CanEditName: account.LifecycleState() == AccountActive,
		CanEditStructure: account.LifecycleState() == AccountActive &&
			!usage.StructureLocked(),
		CanArchive:      account.LifecycleState() == AccountActive,
		CanTrash:        account.LifecycleState() == AccountActive,
		CanRestore:      account.LifecycleState() != AccountActive,
		CanDuplicate:    !account.SystemManaged,
		EditBlockers:    make([]string, 0),
		ArchiveBlockers: make([]string, 0),
		TrashBlockers:   make([]string, 0),
		RestoreBlockers: make([]string, 0),
	}
	if account.SystemManaged {
		capabilities.CanEditName = false
		capabilities.CanEditStructure = false
		capabilities.CanArchive = false
		capabilities.CanTrash = false
		capabilities.EditBlockers = append(
			capabilities.EditBlockers,
			BlockerAccountProtected,
		)
		capabilities.ArchiveBlockers = append(
			capabilities.ArchiveBlockers,
			BlockerAccountProtected,
		)
		capabilities.TrashBlockers = append(
			capabilities.TrashBlockers,
			BlockerAccountProtected,
		)
	}
	if usage.StructureLocked() {
		capabilities.EditBlockers = append(
			capabilities.EditBlockers,
			BlockerAccountStructureLocked,
		)
	}
	if usage.Mappings > 0 {
		capabilities.CanArchive = false
		capabilities.ArchiveBlockers = append(
			capabilities.ArchiveBlockers,
			BlockerAccountMapped,
		)
	}
	if usage.ActiveFinancialAccounts > 0 {
		capabilities.CanArchive = false
		capabilities.ArchiveBlockers = append(
			capabilities.ArchiveBlockers,
			BlockerAccountFinancialDependency,
		)
	}
	if usage.ActiveChildren > 0 {
		capabilities.CanArchive = false
		capabilities.ArchiveBlockers = append(
			capabilities.ArchiveBlockers,
			BlockerAccountHasActiveChildren,
		)
	}
	if usage.HasDependencies() {
		capabilities.CanTrash = false
		capabilities.TrashBlockers = append(
			capabilities.TrashBlockers,
			BlockerAccountStructureLocked,
		)
	}
	if usage.InactiveAncestors > 0 {
		capabilities.CanRestore = false
		capabilities.RestoreBlockers = append(
			capabilities.RestoreBlockers,
			BlockerAccountParentInactive,
		)
	}
	return capabilities
}

type AccountDetail struct {
	Account      Account             `json:"account"`
	Usage        AccountUsage        `json:"usage"`
	Capabilities AccountCapabilities `json:"capabilities"`
	MappingRoles []string            `json:"mapping_roles"`
}

type AccountMapping struct {
	Role        string    `json:"role"`
	AccountID   uuid.UUID `json:"account_id"`
	AccountCode string    `json:"account_code,omitempty"`
	AccountName string    `json:"account_name,omitempty"`
	Version     int64     `json:"version"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by"`
	Reason      string    `json:"-"`
}

type AccountMappingDefinition struct {
	Role                      string                   `json:"role"`
	LabelES                   string                   `json:"label_es"`
	LabelEN                   string                   `json:"label_en"`
	DescriptionES             string                   `json:"description_es"`
	DescriptionEN             string                   `json:"description_en"`
	Required                  bool                     `json:"required"`
	CompatibleAccountClasses  []AccountClass           `json:"compatible_account_classes"`
	CompatibleNormalBalances  []NormalBalance          `json:"compatible_normal_balances"`
	CompatibleMonetaryClasses []MonetaryClassification `json:"compatible_monetary_classes"`
	CanonicalRole             string                   `json:"canonical_role,omitempty"`
	Alias                     bool                     `json:"is_alias"`
	DisplayOrder              int                      `json:"display_order"`
}

func (m AccountMapping) Validate() error {
	role := strings.TrimSpace(m.Role)
	if role == "" || len(role) > 80 {
		return fmt.Errorf("%w: mapping role is required", ErrInvalidArgument)
	}
	for _, char := range role {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			return fmt.Errorf("%w: mapping role must use lowercase snake_case", ErrInvalidArgument)
		}
	}
	if m.AccountID == uuid.Nil {
		return fmt.Errorf("%w: mapped account is required", ErrInvalidArgument)
	}
	return nil
}

type PeriodStatus string

const (
	PeriodOpen       PeriodStatus = "open"
	PeriodSoftClosed PeriodStatus = "soft_closed"
	PeriodLocked     PeriodStatus = "locked"
)

func (s PeriodStatus) Valid() bool {
	return s == PeriodOpen || s == PeriodSoftClosed || s == PeriodLocked
}

type Period struct {
	ID               uuid.UUID          `json:"id"`
	Name             string             `json:"name"`
	StartDate        time.Time          `json:"start_date"`
	EndDate          time.Time          `json:"end_date"`
	Status           PeriodStatus       `json:"status"`
	Version          int64              `json:"version"`
	FiscalYearID     *uuid.UUID         `json:"fiscal_year_id,omitempty"`
	SequenceNo       int                `json:"sequence_no,omitempty"`
	IsLegacy         bool               `json:"is_legacy,omitempty"`
	Checklist        CloseChecklist     `json:"checklist"`
	Capabilities     PeriodCapabilities `json:"capabilities"`
	StatusChangedBy  string             `json:"status_changed_by,omitempty"`
	TransitionReason string             `json:"transition_reason,omitempty"`
}

func (p Period) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: period name is required", ErrInvalidArgument)
	}
	if p.StartDate.IsZero() || p.EndDate.IsZero() || p.EndDate.Before(p.StartDate) {
		return fmt.Errorf("%w: invalid period date range", ErrInvalidArgument)
	}
	if !p.Status.Valid() {
		return fmt.Errorf("%w: invalid period status", ErrInvalidArgument)
	}
	return nil
}

type CloseChecklist struct {
	UnpostedDocuments       int        `json:"unposted_documents"`
	PendingFiscalDocuments  int        `json:"pending_fiscal_documents"`
	PostingErrors           int        `json:"posting_errors"`
	MissingMappings         int        `json:"missing_mappings"`
	MissingExchangeRates    int        `json:"missing_exchange_rates"`
	UnclosedReconciliations int        `json:"unclosed_reconciliations"`
	PendingDrafts           int        `json:"pending_drafts"`
	EvaluatedAt             *time.Time `json:"evaluated_at,omitempty"`
}

func (c CloseChecklist) BlockingCount() int {
	return c.UnpostedDocuments +
		c.PendingFiscalDocuments +
		c.PostingErrors +
		c.MissingMappings +
		c.MissingExchangeRates +
		c.UnclosedReconciliations +
		c.PendingDrafts
}

type EntrySource struct {
	Type           string    `json:"type"`
	ID             uuid.UUID `json:"id"`
	Event          string    `json:"event"`
	IdempotencyKey string    `json:"-"`
}

type EntryKind string

const (
	EntryManual      EntryKind = "manual"
	EntrySale        EntryKind = "sale"
	EntryPurchase    EntryKind = "purchase"
	EntryCollection  EntryKind = "collection"
	EntryPayment     EntryKind = "payment"
	EntryRefund      EntryKind = "refund"
	EntryInventory   EntryKind = "inventory"
	EntryCOGS        EntryKind = "cogs"
	EntryTax         EntryKind = "tax"
	EntryAdjustment  EntryKind = "adjustment"
	EntryClosing     EntryKind = "closing"
	EntryInflation   EntryKind = "inflation"
	EntryRevaluation EntryKind = "revaluation"
	EntryReversal    EntryKind = "reversal"
)

func (k EntryKind) Valid() bool {
	switch k {
	case EntryManual, EntrySale, EntryPurchase, EntryCollection, EntryPayment,
		EntryRefund, EntryInventory, EntryCOGS, EntryTax, EntryAdjustment,
		EntryClosing, EntryInflation, EntryRevaluation, EntryReversal:
		return true
	default:
		return false
	}
}

func (s EntrySource) Validate() error {
	if strings.TrimSpace(s.Type) == "" || strings.TrimSpace(s.Event) == "" {
		return fmt.Errorf("%w: entry source type and event are required", ErrInvalidArgument)
	}
	if s.ID == uuid.Nil {
		return fmt.Errorf("%w: entry source id is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(s.IdempotencyKey) == "" {
		return fmt.Errorf("%w: idempotency key is required", ErrInvalidArgument)
	}
	return nil
}

type JournalLine struct {
	ID                 uuid.UUID  `json:"id"`
	AccountID          uuid.UUID  `json:"account_id"`
	AccountCode        string     `json:"account_code,omitempty"`
	AccountName        string     `json:"account_name,omitempty"`
	Debit              Decimal    `json:"debit"`
	Credit             Decimal    `json:"credit"`
	TransactionDebit   Decimal    `json:"transaction_debit"`
	TransactionCredit  Decimal    `json:"transaction_credit"`
	Currency           Currency   `json:"currency"`
	ExchangeRate       Decimal    `json:"exchange_rate"`
	ExchangeRateDate   time.Time  `json:"exchange_rate_date"`
	ExchangeRateSource string     `json:"exchange_rate_source"`
	PartyID            *uuid.UUID `json:"party_id,omitempty"`
	OpenItemID         *uuid.UUID `json:"open_item_id,omitempty"`
	Memo               string     `json:"memo,omitempty"`
	LineNo             int        `json:"line_no"`
}

func (l JournalLine) Validate(posting bool) error {
	if l.AccountID == uuid.Nil {
		return fmt.Errorf("%w: journal line account is required", ErrInvalidArgument)
	}
	for _, amount := range []Decimal{
		l.Debit,
		l.Credit,
		l.TransactionDebit,
		l.TransactionCredit,
	} {
		if err := validateAmountBounds(amount); err != nil {
			return err
		}
	}
	if !l.ExchangeRate.IsZero() {
		if l.ExchangeRate.Sign() < 0 {
			return fmt.Errorf("%w: journal line exchange rate must be positive", ErrInvalidArgument)
		}
		if err := validateExchangeRateBounds(l.ExchangeRate); err != nil {
			return err
		}
	}
	if l.Debit.Sign() < 0 || l.Credit.Sign() < 0 ||
		l.TransactionDebit.Sign() < 0 || l.TransactionCredit.Sign() < 0 {
		return fmt.Errorf("%w: journal amounts cannot be negative", ErrInvalidArgument)
	}
	if posting {
		if l.Debit.IsZero() == l.Credit.IsZero() {
			return fmt.Errorf("%w: each line must have exactly one positive debit or credit", ErrInvalidArgument)
		}
		if l.TransactionDebit.IsZero() == l.TransactionCredit.IsZero() {
			return fmt.Errorf("%w: each line must have exactly one positive transaction debit or credit", ErrInvalidArgument)
		}
		if (!l.Debit.IsZero()) != (!l.TransactionDebit.IsZero()) {
			return fmt.Errorf("%w: functional and transaction amounts must use the same side", ErrInvalidArgument)
		}
		if l.ExchangeRate.Sign() <= 0 {
			return fmt.Errorf("%w: journal line exchange rate must be positive", ErrInvalidArgument)
		}
	}
	return nil
}

type JournalEntry struct {
	ID                    uuid.UUID     `json:"id"`
	Number                int64         `json:"number"`
	Date                  time.Time     `json:"date"`
	Reference             string        `json:"reference,omitempty"`
	Kind                  EntryKind     `json:"kind"`
	PostingKind           string        `json:"posting_kind"`
	FunctionalCurrency    Currency      `json:"functional_currency"`
	Currency              Currency      `json:"currency"`
	ExchangeRate          Decimal       `json:"exchange_rate"`
	ExchangeRateDate      time.Time     `json:"exchange_rate_date"`
	ExchangeRateSource    string        `json:"exchange_rate_source"`
	Source                EntrySource   `json:"source"`
	Description           string        `json:"description"`
	CreatedBy             string        `json:"created_by"`
	CreatedAt             time.Time     `json:"created_at"`
	ReversesEntryID       *uuid.UUID    `json:"reverses_entry_id,omitempty"`
	ReversesEntryNumber   *int64        `json:"reverses_entry_number,omitempty"`
	ReversedByEntryID     *uuid.UUID    `json:"reversed_by_entry_id,omitempty"`
	ReversedByEntryNumber *int64        `json:"reversed_by_entry_number,omitempty"`
	ReversalReason        string        `json:"reversal_reason,omitempty"`
	IsAdjustment          bool          `json:"is_adjustment"`
	DraftID               *uuid.UUID    `json:"draft_id,omitempty"`
	Lines                 []JournalLine `json:"lines"`
	DebitTotal            Decimal       `json:"-"`
	CreditTotal           Decimal       `json:"-"`
}

func (e JournalEntry) ValidateForPosting() error {
	if e.Date.IsZero() {
		return fmt.Errorf("%w: entry date is required", ErrInvalidArgument)
	}
	if !e.Kind.Valid() {
		return fmt.Errorf("%w: invalid journal entry kind", ErrInvalidArgument)
	}
	if strings.TrimSpace(e.PostingKind) == "" {
		return fmt.Errorf("%w: posting kind is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(e.Description) == "" {
		return fmt.Errorf("%w: entry description is required", ErrInvalidArgument)
	}
	if len(strings.TrimSpace(e.Reference)) > 160 {
		return fmt.Errorf("%w: entry reference must not exceed 160 characters", ErrInvalidArgument)
	}
	if strings.TrimSpace(e.CreatedBy) == "" {
		return fmt.Errorf("%w: entry actor is required", ErrInvalidArgument)
	}
	if err := e.Source.Validate(); err != nil {
		return err
	}
	if e.ExchangeRate.Sign() <= 0 {
		return fmt.Errorf("%w: exchange rate must be positive", ErrInvalidArgument)
	}
	if e.Currency.Code() == e.FunctionalCurrency.Code() {
		if !e.ExchangeRate.Equal(One) {
			return fmt.Errorf(
				"%w: functional-currency entry must use exchange rate 1",
				ErrInvalidArgument,
			)
		}
		if !e.ExchangeRateDate.IsZero() ||
			strings.TrimSpace(e.ExchangeRateSource) != "" {
			return fmt.Errorf(
				"%w: functional-currency entry must not include exchange-rate metadata",
				ErrInvalidArgument,
			)
		}
	} else if e.ExchangeRateDate.IsZero() ||
		e.ExchangeRateDate.After(e.Date) ||
		strings.TrimSpace(e.ExchangeRateSource) == "" {
		return fmt.Errorf(
			"%w: foreign-currency entry requires a valid exchange-rate date and source",
			ErrInvalidArgument,
		)
	}
	if len(e.Lines) < 2 {
		return fmt.Errorf("%w: an entry needs at least two lines", ErrInvalidArgument)
	}
	var debit, credit Decimal
	for index, line := range e.Lines {
		if err := line.Validate(true); err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
		debit = debit.Add(line.Debit)
		credit = credit.Add(line.Credit)
		transactionAmount := line.TransactionDebit.Add(line.TransactionCredit)
		functionalAmount := line.Debit.Add(line.Credit)
		if !convert(transactionAmount, line.ExchangeRate, e.FunctionalCurrency).Equal(functionalAmount) {
			return fmt.Errorf("line %d: %w: currency conversion is inconsistent", index+1, ErrInvalidArgument)
		}
		if line.Currency.Code() == e.FunctionalCurrency.Code() {
			if !line.ExchangeRate.Equal(One) || !transactionAmount.Equal(functionalAmount) {
				return fmt.Errorf("line %d: %w: functional-currency line must use exchange rate 1", index+1, ErrInvalidArgument)
			}
		} else if line.ExchangeRateDate.IsZero() ||
			line.ExchangeRateDate.After(e.Date) ||
			strings.TrimSpace(line.ExchangeRateSource) == "" {
			return fmt.Errorf("line %d: %w: foreign-currency line requires rate date and source", index+1, ErrInvalidArgument)
		}
	}
	if !debit.Equal(credit) {
		return ErrUnbalancedEntry
	}
	return nil
}

func (e JournalEntry) Totals() (debit Decimal, credit Decimal) {
	if len(e.Lines) == 0 && (!e.DebitTotal.IsZero() || !e.CreditTotal.IsZero()) {
		return e.DebitTotal, e.CreditTotal
	}
	for _, line := range e.Lines {
		debit = debit.Add(line.Debit)
		credit = credit.Add(line.Credit)
	}
	return debit, credit
}

type Draft struct {
	ID                 uuid.UUID          `json:"id"`
	Version            int64              `json:"version"`
	IdempotencyKey     string             `json:"-"`
	Date               time.Time          `json:"date"`
	Reference          string             `json:"reference,omitempty"`
	Kind               EntryKind          `json:"kind"`
	FunctionalCurrency Currency           `json:"functional_currency"`
	Currency           Currency           `json:"currency"`
	ExchangeRate       Decimal            `json:"exchange_rate"`
	ExchangeRateDate   time.Time          `json:"exchange_rate_date"`
	ExchangeRateSource string             `json:"exchange_rate_source"`
	Description        string             `json:"description"`
	IsAdjustment       bool               `json:"is_adjustment"`
	SourceType         string             `json:"source_type,omitempty"`
	SourceID           string             `json:"source_id,omitempty"`
	Lines              []JournalLine      `json:"lines"`
	CreatedBy          string             `json:"created_by"`
	UpdatedBy          string             `json:"updated_by"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	PostingStatus      DraftPostingStatus `json:"posting_status"`
}

func (d Draft) ValidateForSave() error {
	if d.Date.IsZero() {
		return fmt.Errorf("%w: draft date is required", ErrInvalidArgument)
	}
	if !d.Kind.Valid() {
		return fmt.Errorf("%w: invalid draft kind", ErrInvalidArgument)
	}
	if strings.TrimSpace(d.IdempotencyKey) == "" {
		return fmt.Errorf("%w: draft idempotency key is required", ErrInvalidArgument)
	}
	if len(strings.TrimSpace(d.Reference)) > 160 {
		return fmt.Errorf("%w: draft reference must not exceed 160 characters", ErrInvalidArgument)
	}
	if len(d.Description) > 500 {
		return fmt.Errorf("%w: draft description must not exceed 500 characters", ErrInvalidArgument)
	}
	if d.ExchangeRate.Sign() <= 0 {
		return fmt.Errorf("%w: draft exchange rate must be positive", ErrInvalidArgument)
	}
	if d.Currency.Code() == d.FunctionalCurrency.Code() {
		if !d.ExchangeRate.Equal(One) {
			return fmt.Errorf(
				"%w: functional-currency draft must use exchange rate 1",
				ErrInvalidArgument,
			)
		}
		if !d.ExchangeRateDate.IsZero() || strings.TrimSpace(d.ExchangeRateSource) != "" {
			return fmt.Errorf(
				"%w: functional-currency draft must not include exchange-rate metadata",
				ErrInvalidArgument,
			)
		}
	} else {
		if d.ExchangeRateDate.IsZero() ||
			d.ExchangeRateDate.After(d.Date) ||
			strings.TrimSpace(d.ExchangeRateSource) == "" {
			return fmt.Errorf(
				"%w: foreign-currency draft requires a valid exchange-rate date and source",
				ErrInvalidArgument,
			)
		}
	}
	if len(strings.TrimSpace(d.ExchangeRateSource)) > 160 {
		return fmt.Errorf(
			"%w: draft exchange-rate source must not exceed 160 characters",
			ErrInvalidArgument,
		)
	}
	for index, line := range d.Lines {
		if err := line.Validate(false); err != nil {
			return fmt.Errorf("line %d: %w", index+1, err)
		}
		if line.Debit.IsZero() == line.Credit.IsZero() {
			return fmt.Errorf("%w: draft line must contain exactly one debit or credit", ErrInvalidArgument)
		}
		if line.TransactionDebit.IsZero() == line.TransactionCredit.IsZero() {
			return fmt.Errorf(
				"%w: draft line must contain exactly one transaction debit or credit",
				ErrInvalidArgument,
			)
		}
		if (!line.Debit.IsZero()) != (!line.TransactionDebit.IsZero()) {
			return fmt.Errorf(
				"%w: functional and transaction amounts must use the same side",
				ErrInvalidArgument,
			)
		}
		if line.Currency.Code() != d.Currency.Code() ||
			!line.ExchangeRate.Equal(d.ExchangeRate) ||
			!line.ExchangeRateDate.Equal(d.ExchangeRateDate) ||
			strings.TrimSpace(line.ExchangeRateSource) !=
				strings.TrimSpace(d.ExchangeRateSource) {
			return fmt.Errorf(
				"line %d: %w: currency metadata differs from draft header",
				index+1,
				ErrInvalidArgument,
			)
		}
		transactionAmount := line.TransactionDebit.Add(line.TransactionCredit)
		functionalAmount := line.Debit.Add(line.Credit)
		if !convert(
			transactionAmount,
			d.ExchangeRate,
			d.FunctionalCurrency,
		).Equal(functionalAmount) {
			return fmt.Errorf(
				"line %d: %w: currency conversion is inconsistent",
				index+1,
				ErrInvalidArgument,
			)
		}
	}
	return nil
}

func (d Draft) ToEntry(source EntrySource, actor string) JournalEntry {
	return JournalEntry{
		Date:               d.Date,
		Reference:          strings.TrimSpace(d.Reference),
		Kind:               d.Kind,
		PostingKind:        "primary",
		FunctionalCurrency: d.FunctionalCurrency,
		Currency:           d.Currency,
		ExchangeRate:       d.ExchangeRate,
		ExchangeRateDate:   d.ExchangeRateDate,
		ExchangeRateSource: d.ExchangeRateSource,
		Source:             source,
		Description:        d.Description,
		CreatedBy:          actor,
		IsAdjustment:       d.IsAdjustment,
		DraftID:            &d.ID,
		Lines:              append([]JournalLine(nil), d.Lines...),
	}
}

type DraftPostingState string

const (
	DraftPostingIncomplete DraftPostingState = "incomplete"
	DraftPostingUnbalanced DraftPostingState = "unbalanced"
	DraftPostingBlocked    DraftPostingState = "blocked"
	DraftPostingReady      DraftPostingState = "ready"
)

type DraftPostingIssue string

const (
	PostingDescriptionRequired DraftPostingIssue = "description_required"
	PostingMinimumLines        DraftPostingIssue = "minimum_lines"
	PostingLineAccountRequired DraftPostingIssue = "line_account_required"
	PostingLineSideInvalid     DraftPostingIssue = "line_side_invalid"
	PostingUnbalanced          DraftPostingIssue = "unbalanced"
	PostingZeroTotal           DraftPostingIssue = "zero_total"
	PostingPeriodClosed        DraftPostingIssue = "period_closed"
	PostingAccountArchived     DraftPostingIssue = "account_archived"
	PostingAccountNotPostable  DraftPostingIssue = "account_not_postable"
)

type DraftPostingContext struct {
	PeriodAllowsPosting bool
	Accounts            map[uuid.UUID]Account
}

type DraftPostingStatus struct {
	State      DraftPostingState   `json:"state"`
	Difference Decimal             `json:"difference"`
	Issues     []DraftPostingIssue `json:"issues"`
}

type DraftPostingSummaryContext struct {
	Description         string
	LineCount           int
	Debit               Decimal
	Credit              Decimal
	PeriodAllowsPosting bool
	HasArchivedAccount  bool
	HasInvalidAccount   bool
}

func EvaluateDraftPostingSummary(
	context DraftPostingSummaryContext,
) DraftPostingStatus {
	issues := make([]DraftPostingIssue, 0, 6)
	incomplete := false
	blocked := false
	if strings.TrimSpace(context.Description) == "" {
		issues = append(issues, PostingDescriptionRequired)
		incomplete = true
	}
	if context.LineCount < 2 {
		issues = append(issues, PostingMinimumLines)
		incomplete = true
	}
	if context.Debit.IsZero() && context.Credit.IsZero() {
		issues = append(issues, PostingZeroTotal)
		incomplete = true
	}
	unbalanced := !context.Debit.Equal(context.Credit)
	if unbalanced {
		issues = append(issues, PostingUnbalanced)
	}
	if context.HasArchivedAccount {
		issues = append(issues, PostingAccountArchived)
		blocked = true
	}
	if context.HasInvalidAccount {
		issues = append(issues, PostingAccountNotPostable)
		blocked = true
	}
	if !context.PeriodAllowsPosting {
		issues = append(issues, PostingPeriodClosed)
		blocked = true
	}
	state := DraftPostingReady
	switch {
	case incomplete:
		state = DraftPostingIncomplete
	case blocked:
		state = DraftPostingBlocked
	case unbalanced:
		state = DraftPostingUnbalanced
	}
	return DraftPostingStatus{
		State:      state,
		Difference: context.Debit.Sub(context.Credit).Abs(),
		Issues:     issues,
	}
}

func EvaluateDraftPostingStatus(draft Draft, context DraftPostingContext) DraftPostingStatus {
	var debit, credit Decimal
	issues := make([]DraftPostingIssue, 0, 6)
	incomplete := false
	blocked := false

	if strings.TrimSpace(draft.Description) == "" {
		issues = append(issues, PostingDescriptionRequired)
		incomplete = true
	}
	if len(draft.Lines) < 2 {
		issues = append(issues, PostingMinimumLines)
		incomplete = true
	}
	for _, line := range draft.Lines {
		debit = debit.Add(line.Debit)
		credit = credit.Add(line.Credit)
		if line.AccountID == uuid.Nil {
			issues = appendDraftPostingIssue(issues, PostingLineAccountRequired)
			incomplete = true
		}
		if line.Debit.Sign() < 0 || line.Credit.Sign() < 0 ||
			(line.Debit.IsZero() == line.Credit.IsZero()) {
			issues = appendDraftPostingIssue(issues, PostingLineSideInvalid)
			incomplete = true
		}
		account, ok := context.Accounts[line.AccountID]
		if !ok {
			issues = appendDraftPostingIssue(issues, PostingAccountNotPostable)
			blocked = true
			continue
		}
		if account.ArchivedAt != nil || account.TrashedAt != nil {
			issues = appendDraftPostingIssue(issues, PostingAccountArchived)
			blocked = true
		}
		if !account.Postable {
			issues = appendDraftPostingIssue(issues, PostingAccountNotPostable)
			blocked = true
		}
	}
	if debit.IsZero() && credit.IsZero() {
		issues = append(issues, PostingZeroTotal)
		incomplete = true
	}
	unbalanced := !debit.Equal(credit)
	if unbalanced {
		issues = append(issues, PostingUnbalanced)
	}
	if !context.PeriodAllowsPosting {
		issues = append(issues, PostingPeriodClosed)
		blocked = true
	}

	state := DraftPostingReady
	switch {
	case incomplete:
		state = DraftPostingIncomplete
	case blocked:
		state = DraftPostingBlocked
	case unbalanced:
		state = DraftPostingUnbalanced
	}
	return DraftPostingStatus{
		State:      state,
		Difference: debit.Sub(credit).Abs(),
		Issues:     issues,
	}
}

func appendDraftPostingIssue(
	issues []DraftPostingIssue,
	issue DraftPostingIssue,
) []DraftPostingIssue {
	for _, existing := range issues {
		if existing == issue {
			return issues
		}
	}
	return append(issues, issue)
}

type OpenItemKind string

const (
	Receivable OpenItemKind = "receivable"
	Payable    OpenItemKind = "payable"
)

type OpenItem struct {
	ID               uuid.UUID    `json:"id"`
	Kind             OpenItemKind `json:"kind"`
	PartyID          uuid.UUID    `json:"party_id"`
	AccountID        uuid.UUID    `json:"account_id"`
	EntryID          uuid.UUID    `json:"entry_id"`
	OriginLineID     uuid.UUID    `json:"origin_line_id"`
	SourceType       string       `json:"source_type"`
	SourceID         uuid.UUID    `json:"source_id"`
	IssueDate        time.Time    `json:"issue_date"`
	DueDate          time.Time    `json:"due_date"`
	Currency         Currency     `json:"currency"`
	OriginalAmount   Decimal      `json:"original_amount"`
	FunctionalAmount Decimal      `json:"functional_amount"`
	OpenAmount       Decimal      `json:"open_amount"`
	OpenFunctional   Decimal      `json:"open_functional_amount"`
}

type OpenItemApplication struct {
	ID                 uuid.UUID `json:"id"`
	OpenItemID         uuid.UUID `json:"open_item_id"`
	SettlementEntryID  uuid.UUID `json:"settlement_entry_id"`
	SettlementLineID   uuid.UUID `json:"settlement_line_id"`
	AppliedAt          time.Time `json:"applied_at"`
	Amount             Decimal   `json:"amount"`
	FunctionalAmount   Decimal   `json:"functional_amount"`
	ExchangeDifference Decimal   `json:"exchange_difference"`
	CreatedBy          string    `json:"created_by"`
}
