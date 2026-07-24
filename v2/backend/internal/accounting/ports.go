package accounting

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type PageRequest struct {
	Limit int
	After string
}

func (p PageRequest) normalized() PageRequest {
	if p.Limit <= 0 {
		p.Limit = 50
	}
	if p.Limit > 200 {
		p.Limit = 200
	}
	return p
}

type PageResult[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// Transactor is the security boundary of the accounting module. An
// implementation must start a database transaction, apply app.org_id with
// transaction-local scope, and expose repositories bound to that transaction.
type Transactor interface {
	WithinTenant(context.Context, Scope, func(context.Context, Repositories) error) error
}

type Repositories interface {
	AccountRepository
	DraftRepository
	JournalRepository
	PeriodRepository
	ReconciliationRepository
	InflationRepository
	RevaluationRepository
}

type AccountRepository interface {
	ListAccounts(context.Context, bool) ([]Account, error)
	GetAccount(context.Context, uuid.UUID) (Account, error)
	CreateAccount(context.Context, Account) (Account, error)
	UpdateAccount(context.Context, Account, int64) (Account, error)
	AccountUsage(context.Context, uuid.UUID) (postings int64, mappings int64, children int64, err error)
	ArchiveAccount(context.Context, uuid.UUID, int64, time.Time, string) (Account, error)
	RestoreAccount(context.Context, uuid.UUID, int64, string) (Account, error)
	DeleteUnusedAccount(context.Context, uuid.UUID, int64) error
	ListMappings(context.Context) ([]AccountMapping, error)
	GetMappings(context.Context, []string) (map[string]AccountMapping, error)
	SetMapping(context.Context, AccountMapping, int64) (AccountMapping, error)
}

type DraftRepository interface {
	CreateDraft(context.Context, Draft) (Draft, error)
	GetDraft(context.Context, uuid.UUID, bool) (Draft, error)
	FindDraftByIdempotency(context.Context, string) (Draft, error)
	UpdateDraft(context.Context, Draft, int64) (Draft, error)
	DiscardDraft(context.Context, uuid.UUID, int64, string, string) error
	MarkDraftPosted(context.Context, uuid.UUID, int64, uuid.UUID) error
}

type JournalFilter struct {
	From          *time.Time
	To            *time.Time
	AccountID     *uuid.UUID
	SourceType    string
	SourceID      *uuid.UUID
	PartyID       *uuid.UUID
	Query         string
	ReversalState string
	IncludeLines  bool
	AfterNumber   int64
	Limit         int
}

type JournalRepository interface {
	PostEntry(context.Context, JournalEntry) (JournalEntry, error)
	GetEntry(context.Context, uuid.UUID) (JournalEntry, error)
	FindEntryBySource(context.Context, EntrySource) (JournalEntry, error)
	FindDirectReversal(context.Context, uuid.UUID) (JournalEntry, error)
	EntryHasOpenItemEffects(context.Context, uuid.UUID) (bool, error)
	TouchesClosedReconciliation(
		context.Context,
		time.Time,
		[]uuid.UUID,
	) (bool, error)
	ListJournal(context.Context, JournalFilter) (PageResult[JournalEntry], error)
	ReportLines(context.Context, time.Time, time.Time) ([]ReportLine, error)
	AccountOpeningBalance(context.Context, uuid.UUID, time.Time) (Decimal, error)
	ListOpenItems(context.Context, OpenItemFilter) ([]OpenItem, error)
	CreateOpenItem(context.Context, OpenItem) (OpenItem, error)
	ApplyOpenItem(context.Context, OpenItemApplication) (OpenItem, error)
}

type PeriodRepository interface {
	ListPeriods(context.Context) ([]Period, error)
	GetPeriod(context.Context, uuid.UUID, bool) (Period, error)
	FindPeriodForDate(context.Context, time.Time, bool) (Period, error)
	CreatePeriod(context.Context, Period) (Period, error)
	UpdatePeriod(context.Context, Period, int64) (Period, error)
	CloseChecklist(context.Context, uuid.UUID) (CloseChecklist, error)
}

type OpenItemFilter struct {
	Kind     OpenItemKind
	PartyID  *uuid.UUID
	AsOf     time.Time
	Overdue  *bool
	Currency *Currency
}

type PeriodAudit struct {
	ID         uuid.UUID    `json:"id"`
	PeriodID   uuid.UUID    `json:"period_id"`
	FromStatus PeriodStatus `json:"from_status"`
	ToStatus   PeriodStatus `json:"to_status"`
	Reason     string       `json:"reason"`
	ActorID    string       `json:"actor_id"`
	OccurredAt time.Time    `json:"occurred_at"`
}

type ReconciliationRepository interface {
	CreateStatementImport(context.Context, StatementImport) (StatementImport, error)
	FindStatementImportByHash(context.Context, uuid.UUID, string) (StatementImport, error)
	ListStatementMovements(context.Context, uuid.UUID) ([]StatementMovement, error)
	CreateReconciliation(context.Context, Reconciliation) (Reconciliation, error)
	GetReconciliation(context.Context, uuid.UUID, bool) (Reconciliation, error)
	ListReconciliations(context.Context, PageRequest) (PageResult[Reconciliation], error)
	SaveReconciliation(context.Context, Reconciliation, int64) (Reconciliation, error)
	ListReconciliationCandidates(context.Context, uuid.UUID, time.Time, time.Time) ([]ReconciliationLedgerCandidate, error)
}

type InflationRepository interface {
	UpsertInflationIndices(context.Context, []InflationIndex) error
	ListInflationIndices(context.Context, time.Time, time.Time) ([]InflationIndex, error)
	ListInflationPositions(context.Context, time.Time) ([]InflationPosition, error)
	CreateInflationWorkpaper(context.Context, InflationWorkpaper) (InflationWorkpaper, error)
	GetInflationWorkpaper(context.Context, uuid.UUID) (InflationWorkpaper, error)
}

type RevaluationRepository interface {
	ListCurrencyRevaluationPositions(
		context.Context,
		time.Time,
		Currency,
	) ([]CurrencyRevaluationPosition, error)
	CreateCurrencyRevaluationWorkpaper(
		context.Context,
		CurrencyRevaluationWorkpaper,
	) (CurrencyRevaluationWorkpaper, error)
	GetCurrencyRevaluationWorkpaper(
		context.Context,
		uuid.UUID,
	) (CurrencyRevaluationWorkpaper, error)
}

type IDGenerator interface {
	NewID() uuid.UUID
}

type UUIDGenerator struct{}

func (UUIDGenerator) NewID() uuid.UUID {
	return uuid.New()
}

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}
