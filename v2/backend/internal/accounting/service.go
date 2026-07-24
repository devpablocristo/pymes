package accounting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	transactor Transactor
	ids        IDGenerator
	clock      Clock
}

func NewService(transactor Transactor) (*Service, error) {
	return NewServiceWithDependencies(transactor, UUIDGenerator{}, SystemClock{})
}

func NewServiceWithDependencies(
	transactor Transactor,
	ids IDGenerator,
	clock Clock,
) (*Service, error) {
	if transactor == nil {
		return nil, fmt.Errorf("%w: accounting transactor is required", ErrInvalidArgument)
	}
	if ids == nil || clock == nil {
		return nil, fmt.Errorf("%w: accounting service dependencies are required", ErrInvalidArgument)
	}
	return &Service{transactor: transactor, ids: ids, clock: clock}, nil
}

type CreateAccountCommand struct {
	Code          string
	Name          string
	Class         AccountClass
	NormalBalance NormalBalance
	Monetary      MonetaryClassification
	ParentID      *uuid.UUID
	Postable      bool
}

func (s *Service) CreateAccount(
	ctx context.Context,
	scope Scope,
	command CreateAccountCommand,
) (Account, error) {
	account := Account{
		ID:            s.ids.NewID(),
		Code:          strings.TrimSpace(command.Code),
		Name:          strings.TrimSpace(command.Name),
		Class:         command.Class,
		NormalBalance: command.NormalBalance,
		Monetary:      command.Monetary,
		ParentID:      command.ParentID,
		Postable:      command.Postable,
		Version:       1,
	}
	if account.NormalBalance == "" {
		account.NormalBalance = DefaultNormalBalance(account.Class)
	}
	if account.Monetary == "" {
		account.Monetary = Monetary
	}
	if err := validateAccountForMutation(account); err != nil {
		return Account{}, err
	}
	var created Account
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		if account.ParentID != nil {
			parent, err := repos.GetAccount(ctx, *account.ParentID)
			if err != nil {
				return err
			}
			if parent.Postable || parent.ArchivedAt != nil || parent.Class != account.Class {
				return fmt.Errorf("%w: invalid account parent", ErrConflict)
			}
		}
		var err error
		created, err = repos.CreateAccount(ctx, account)
		return err
	})
	return created, err
}

type UpdateAccountCommand struct {
	ID              uuid.UUID
	ExpectedVersion int64
	Code            string
	Name            string
	Class           AccountClass
	NormalBalance   NormalBalance
	Monetary        MonetaryClassification
	ParentID        *uuid.UUID
	Postable        bool
}

func (s *Service) UpdateAccount(
	ctx context.Context,
	scope Scope,
	command UpdateAccountCommand,
) (Account, error) {
	if command.ID == uuid.Nil || command.ExpectedVersion <= 0 {
		return Account{}, fmt.Errorf("%w: account id and version are required", ErrInvalidArgument)
	}
	var updated Account
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		current, err := repos.GetAccount(ctx, command.ID)
		if err != nil {
			return err
		}
		current.Code = strings.TrimSpace(command.Code)
		current.Name = strings.TrimSpace(command.Name)
		current.Class = command.Class
		current.NormalBalance = command.NormalBalance
		current.Monetary = command.Monetary
		current.ParentID = command.ParentID
		current.Postable = command.Postable
		if err := validateAccountForMutation(current); err != nil {
			return err
		}
		if current.ParentID != nil {
			parent, parentErr := repos.GetAccount(ctx, *current.ParentID)
			if parentErr != nil {
				return parentErr
			}
			if parent.Postable || parent.ArchivedAt != nil || parent.Class != current.Class {
				return fmt.Errorf("%w: invalid account parent", ErrConflict)
			}
		}
		updated, err = repos.UpdateAccount(ctx, current, command.ExpectedVersion)
		return err
	})
	return updated, err
}

func (s *Service) ArchiveAccount(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
	expectedVersion int64,
) (Account, error) {
	if id == uuid.Nil || expectedVersion <= 0 {
		return Account{}, fmt.Errorf("%w: account id and version are required", ErrInvalidArgument)
	}
	var account Account
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		postings, mappings, children, err := repos.AccountUsage(ctx, id)
		if err != nil {
			return err
		}
		if children > 0 {
			return fmt.Errorf("%w: archive child accounts first", ErrAccountInUse)
		}
		_ = postings
		_ = mappings
		account, err = repos.ArchiveAccount(ctx, id, expectedVersion, s.clock.Now(), scope.ActorID)
		return err
	})
	return account, err
}

func (s *Service) RestoreAccount(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
	expectedVersion int64,
) (Account, error) {
	var account Account
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		account, err = repos.RestoreAccount(ctx, id, expectedVersion, scope.ActorID)
		return err
	})
	return account, err
}

func (s *Service) TrashUnusedAccount(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
	expectedVersion int64,
) error {
	return s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		postings, mappings, children, err := repos.AccountUsage(ctx, id)
		if err != nil {
			return err
		}
		if postings != 0 || mappings != 0 || children != 0 {
			return ErrAccountInUse
		}
		return repos.DeleteUnusedAccount(ctx, id, expectedVersion)
	})
}

func (s *Service) ListAccounts(
	ctx context.Context,
	scope Scope,
	includeArchived bool,
) ([]Account, error) {
	var accounts []Account
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		accounts, err = repos.ListAccounts(ctx, includeArchived)
		return err
	})
	return accounts, err
}

func (s *Service) SetAccountMapping(
	ctx context.Context,
	scope Scope,
	role string,
	accountID uuid.UUID,
	expectedVersion int64,
) (AccountMapping, error) {
	mapping := AccountMapping{
		Role:      strings.TrimSpace(role),
		AccountID: accountID,
		UpdatedBy: scope.ActorID,
	}
	if err := mapping.Validate(); err != nil {
		return AccountMapping{}, err
	}
	var saved AccountMapping
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		account, err := repos.GetAccount(ctx, accountID)
		if err != nil {
			return err
		}
		if !account.Postable {
			return ErrAccountNotPostable
		}
		if account.ArchivedAt != nil {
			return ErrAccountArchived
		}
		saved, err = repos.SetMapping(ctx, mapping, expectedVersion)
		return err
	})
	return saved, err
}

func (s *Service) ListAccountMappings(
	ctx context.Context,
	scope Scope,
) ([]AccountMapping, error) {
	var mappings []AccountMapping
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		mappings, err = repos.ListMappings(ctx)
		return err
	})
	return mappings, err
}

func (s *Service) CreateDraft(
	ctx context.Context,
	scope Scope,
	draft Draft,
) (Draft, error) {
	if draft.ID == uuid.Nil {
		draft.ID = s.ids.NewID()
	}
	if draft.Version == 0 {
		draft.Version = 1
	}
	draft.CreatedBy = scope.ActorID
	draft.UpdatedBy = scope.ActorID
	now := s.clock.Now()
	draft.CreatedAt = now
	draft.UpdatedAt = now
	normalizeDraftLines(&draft)
	if err := draft.ValidateForSave(); err != nil {
		return Draft{}, err
	}
	var created Draft
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		created, err = repos.CreateDraft(ctx, draft)
		return err
	})
	return created, err
}

func (s *Service) UpdateDraft(
	ctx context.Context,
	scope Scope,
	draft Draft,
	expectedVersion int64,
) (Draft, error) {
	if draft.ID == uuid.Nil || expectedVersion <= 0 {
		return Draft{}, fmt.Errorf("%w: draft id and version are required", ErrInvalidArgument)
	}
	draft.UpdatedBy = scope.ActorID
	draft.UpdatedAt = s.clock.Now()
	normalizeDraftLines(&draft)
	if err := draft.ValidateForSave(); err != nil {
		return Draft{}, err
	}
	var updated Draft
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		updated, err = repos.UpdateDraft(ctx, draft, expectedVersion)
		return err
	})
	return updated, err
}

func (s *Service) DeleteDraft(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
	expectedVersion int64,
) error {
	return s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		return repos.DeleteDraft(ctx, id, expectedVersion)
	})
}

func (s *Service) PostDraft(
	ctx context.Context,
	scope Scope,
	id uuid.UUID,
	expectedVersion int64,
	idempotencyKey string,
) (JournalEntry, error) {
	if id == uuid.Nil || expectedVersion <= 0 || strings.TrimSpace(idempotencyKey) == "" {
		return JournalEntry{}, fmt.Errorf("%w: draft id, version and idempotency key are required", ErrInvalidArgument)
	}
	var posted JournalEntry
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		draft, err := repos.GetDraft(ctx, id, true)
		if err != nil {
			return err
		}
		if draft.Version != expectedVersion {
			return ErrVersionConflict
		}
		source := EntrySource{
			Type:           "manual_draft",
			ID:             id,
			Event:          "primary",
			IdempotencyKey: idempotencyKey,
		}
		entry := draft.ToEntry(source, scope.ActorID)
		entry.DraftID = &id
		posted, err = s.postEntryInTx(ctx, repos, scope, entry)
		if err != nil {
			return err
		}
		return repos.MarkDraftPosted(ctx, id, expectedVersion, posted.ID)
	})
	return posted, err
}

func (s *Service) PostPlan(
	ctx context.Context,
	scope Scope,
	plan PostingPlan,
) (PostingPlan, error) {
	var posted PostingPlan
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		entry, created, err := s.postEntryWithStatusInTx(ctx, repos, scope, plan.Entry)
		if err != nil {
			return err
		}
		posted.Entry = entry
		// The entry and its open-item effects are committed in this transaction.
		// An idempotent replay must not create those effects a second time.
		if !created {
			return nil
		}
		for _, item := range plan.OpenItems {
			item.EntryID = entry.ID
			for _, line := range entry.Lines {
				if line.AccountID == item.AccountID {
					item.OriginLineID = line.ID
					break
				}
			}
			if item.OriginLineID == uuid.Nil {
				return fmt.Errorf("%w: open item control line", ErrNotFound)
			}
			created, createErr := repos.CreateOpenItem(ctx, item)
			if createErr != nil {
				return createErr
			}
			posted.OpenItems = append(posted.OpenItems, created)
		}
		for _, application := range plan.Applications {
			application.SettlementEntryID = entry.ID
			for _, line := range entry.Lines {
				if line.PartyID != nil {
					application.SettlementLineID = line.ID
					break
				}
			}
			if application.SettlementLineID == uuid.Nil {
				return fmt.Errorf("%w: settlement control line", ErrNotFound)
			}
			item, applyErr := repos.ApplyOpenItem(ctx, application)
			if applyErr != nil {
				return applyErr
			}
			posted.Applications = append(posted.Applications, application)
			if len(posted.OpenItems) == 0 {
				posted.OpenItems = append(posted.OpenItems, item)
			}
		}
		return nil
	})
	return posted, err
}

func (s *Service) PostEntry(
	ctx context.Context,
	scope Scope,
	entry JournalEntry,
) (JournalEntry, error) {
	var posted JournalEntry
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		posted, err = s.postEntryInTx(ctx, repos, scope, entry)
		return err
	})
	return posted, err
}

func (s *Service) GetEntry(
	ctx context.Context,
	scope Scope,
	entryID uuid.UUID,
) (JournalEntry, error) {
	if entryID == uuid.Nil {
		return JournalEntry{}, fmt.Errorf(
			"%w: journal entry is required",
			ErrInvalidArgument,
		)
	}
	var entry JournalEntry
	err := s.withTenant(ctx, scope, func(
		ctx context.Context,
		repos Repositories,
	) error {
		var err error
		entry, err = repos.GetEntry(ctx, entryID)
		return err
	})
	return entry, err
}

func (s *Service) ListOpenItems(
	ctx context.Context,
	scope Scope,
	filter OpenItemFilter,
) ([]OpenItem, error) {
	if filter.Kind != "" && filter.Kind != Receivable && filter.Kind != Payable {
		return nil, fmt.Errorf("%w: open item kind", ErrInvalidArgument)
	}
	if filter.PartyID != nil && *filter.PartyID == uuid.Nil {
		return nil, fmt.Errorf("%w: open item party", ErrInvalidArgument)
	}
	if filter.Currency != nil && filter.Currency.Code() == "" {
		return nil, fmt.Errorf("%w: open item currency", ErrInvalidArgument)
	}
	var items []OpenItem
	err := s.withTenant(ctx, scope, func(
		ctx context.Context,
		repos Repositories,
	) error {
		var err error
		items, err = repos.ListOpenItems(ctx, filter)
		return err
	})
	return items, err
}

type ReverseEntryCommand struct {
	EntryID        uuid.UUID
	Date           time.Time
	Reason         string
	IdempotencyKey string
}

func (s *Service) ReverseEntry(
	ctx context.Context,
	scope Scope,
	command ReverseEntryCommand,
) (JournalEntry, error) {
	if command.EntryID == uuid.Nil || command.Date.IsZero() ||
		strings.TrimSpace(command.Reason) == "" || strings.TrimSpace(command.IdempotencyKey) == "" {
		return JournalEntry{}, fmt.Errorf("%w: reversal entry, date, reason and idempotency key are required", ErrInvalidArgument)
	}
	var posted JournalEntry
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		original, err := repos.GetEntry(ctx, command.EntryID)
		if err != nil {
			return err
		}
		if existing, findErr := repos.FindDirectReversal(ctx, command.EntryID); findErr == nil {
			posted = existing
			if existing.Source.IdempotencyKey == command.IdempotencyKey {
				return nil
			}
			return ErrAlreadyReversed
		} else if !errors.Is(findErr, ErrNotFound) {
			return findErr
		}
		reversal := JournalEntry{
			Date:               command.Date,
			Kind:               EntryReversal,
			PostingKind:        "reversal",
			FunctionalCurrency: original.FunctionalCurrency,
			Currency:           original.Currency,
			ExchangeRate:       original.ExchangeRate,
			ExchangeRateDate:   original.ExchangeRateDate,
			ExchangeRateSource: original.ExchangeRateSource,
			Source: EntrySource{
				Type:           "journal_entry",
				ID:             original.ID,
				Event:          "reversal",
				IdempotencyKey: command.IdempotencyKey,
			},
			Description:     "Reversa: " + original.Description,
			CreatedBy:       scope.ActorID,
			ReversesEntryID: &original.ID,
			ReversalReason:  strings.TrimSpace(command.Reason),
			IsAdjustment:    true,
			Lines:           make([]JournalLine, 0, len(original.Lines)),
		}
		for index, line := range original.Lines {
			reversal.Lines = append(reversal.Lines, JournalLine{
				AccountID:          line.AccountID,
				Debit:              line.Credit,
				Credit:             line.Debit,
				TransactionDebit:   line.TransactionCredit,
				TransactionCredit:  line.TransactionDebit,
				Currency:           line.Currency,
				ExchangeRate:       line.ExchangeRate,
				ExchangeRateDate:   line.ExchangeRateDate,
				ExchangeRateSource: line.ExchangeRateSource,
				PartyID:            line.PartyID,
				Memo:               "Reversa: " + line.Memo,
				LineNo:             index + 1,
			})
		}
		posted, err = s.postEntryInTx(ctx, repos, scope, reversal)
		return err
	})
	return posted, err
}

func (s *Service) CreatePeriod(
	ctx context.Context,
	scope Scope,
	period Period,
) (Period, error) {
	if period.ID == uuid.Nil {
		period.ID = s.ids.NewID()
	}
	period.Status = PeriodOpen
	period.Version = 1
	if err := period.Validate(); err != nil {
		return Period{}, err
	}
	var created Period
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		created, err = repos.CreatePeriod(ctx, period)
		return err
	})
	return created, err
}

func (s *Service) TransitionPeriod(
	ctx context.Context,
	scope Scope,
	periodID uuid.UUID,
	expectedVersion int64,
	target PeriodStatus,
	reason string,
) (Period, CloseChecklist, error) {
	if periodID == uuid.Nil || expectedVersion <= 0 || !target.Valid() {
		return Period{}, CloseChecklist{}, fmt.Errorf("%w: invalid period transition", ErrInvalidArgument)
	}
	var (
		updated   Period
		checklist CloseChecklist
	)
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		period, err := repos.GetPeriod(ctx, periodID, true)
		if err != nil {
			return err
		}
		if period.Version != expectedVersion {
			return ErrVersionConflict
		}
		if err := validatePeriodTransition(period.Status, target); err != nil {
			return err
		}
		if target == PeriodSoftClosed || target == PeriodLocked {
			checklist, err = repos.CloseChecklist(ctx, periodID)
			if err != nil {
				return err
			}
			if checklist.BlockingCount() != 0 {
				return fmt.Errorf("%w: close checklist has %d blocking items", ErrConflict, checklist.BlockingCount())
			}
		}
		if target == PeriodOpen && period.Status != PeriodOpen {
			if !scope.CanReopenPeriods {
				return fmt.Errorf("%w: period reopen permission is required", ErrConflict)
			}
			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("%w: reopening reason is required", ErrInvalidArgument)
			}
			now := s.clock.Now()
			period.ReopenedAt = &now
			period.ReopenedBy = scope.ActorID
			period.ReopenedReason = strings.TrimSpace(reason)
		}
		period.Status = target
		period.StatusChangedBy = scope.ActorID
		period.TransitionReason = strings.TrimSpace(reason)
		period.Version++
		updated, err = repos.UpdatePeriod(ctx, period, expectedVersion)
		return err
	})
	return updated, checklist, err
}

func (s *Service) Journal(
	ctx context.Context,
	scope Scope,
	filter JournalFilter,
) (PageResult[JournalEntry], error) {
	var result PageResult[JournalEntry]
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		result, err = repos.ListJournal(ctx, filter)
		return err
	})
	return result, err
}

func (s *Service) TrialBalance(
	ctx context.Context,
	scope Scope,
	from, asOf time.Time,
) (TrialBalance, error) {
	var result TrialBalance
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		lines, err := repos.ReportLines(
			ctx,
			time.Date(1900, time.January, 1, 0, 0, 0, 0, time.UTC),
			asOf,
		)
		if err != nil {
			return err
		}
		result = BuildTrialBalanceWithOpening(from, asOf, lines)
		return nil
	})
	return result, err
}

func (s *Service) GeneralLedger(
	ctx context.Context,
	scope Scope,
	accountID uuid.UUID,
	from, to time.Time,
) (GeneralLedger, error) {
	var result GeneralLedger
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		account, err := repos.GetAccount(ctx, accountID)
		if err != nil {
			return err
		}
		opening, err := repos.AccountOpeningBalance(ctx, accountID, from)
		if err != nil {
			return err
		}
		lines, err := repos.ReportLines(ctx, from, to)
		if err != nil {
			return err
		}
		result = BuildGeneralLedger(account, from, to, opening, lines)
		return nil
	})
	return result, err
}

func (s *Service) BalanceSheet(
	ctx context.Context,
	scope Scope,
	fiscalYearStart, asOf time.Time,
) (BalanceSheet, error) {
	trial, err := s.TrialBalance(ctx, scope, fiscalYearStart, asOf)
	if err != nil {
		return BalanceSheet{}, err
	}
	return BuildBalanceSheet(trial), nil
}

func (s *Service) IncomeStatement(
	ctx context.Context,
	scope Scope,
	from, to time.Time,
) (IncomeStatement, error) {
	trial, err := s.TrialBalance(ctx, scope, from, to)
	if err != nil {
		return IncomeStatement{}, err
	}
	return BuildIncomeStatement(trial), nil
}

func (s *Service) PreviewAnnualClosing(
	ctx context.Context,
	scope Scope,
	command AnnualClosingCommand,
) (AnnualClosingWorkpaper, error) {
	if err := validateAnnualClosingCommand(command); err != nil {
		return AnnualClosingWorkpaper{}, err
	}
	var workpaper AnnualClosingWorkpaper
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		var err error
		workpaper, err = s.buildAnnualClosingInTx(ctx, repos, scope, command)
		return err
	})
	return workpaper, err
}

func (s *Service) CreateAnnualClosingDraft(
	ctx context.Context,
	scope Scope,
	command AnnualClosingCommand,
) (Draft, error) {
	if err := validateAnnualClosingCommand(command); err != nil {
		return Draft{}, err
	}
	var created Draft
	err := s.withTenant(ctx, scope, func(ctx context.Context, repos Repositories) error {
		existing, findErr := repos.FindDraftByIdempotency(ctx, command.IdempotencyKey)
		if findErr == nil {
			if existing.Kind != EntryClosing ||
				existing.SourceType != "annual_closing" ||
				existing.SourceID != annualClosingSourceID(command.From, command.To) ||
				existing.FunctionalCurrency.Code() != command.FunctionalCurrency.Code() {
				return ErrIdempotencyConflict
			}
			created = existing
			return nil
		}
		if !errors.Is(findErr, ErrNotFound) {
			return findErr
		}
		workpaper, err := s.buildAnnualClosingInTx(ctx, repos, scope, command)
		if err != nil {
			return err
		}
		created, err = repos.CreateDraft(ctx, workpaper.Draft)
		return err
	})
	return created, err
}

func (s *Service) buildAnnualClosingInTx(
	ctx context.Context,
	repos Repositories,
	scope Scope,
	command AnnualClosingCommand,
) (AnnualClosingWorkpaper, error) {
	lines, err := repos.ReportLines(ctx, command.From, command.To)
	if err != nil {
		return AnnualClosingWorkpaper{}, err
	}
	trial := BuildTrialBalance(command.From, command.To, lines)
	mappings, err := repos.GetMappings(ctx, []string{RoleCurrentResult})
	if err != nil {
		return AnnualClosingWorkpaper{}, err
	}
	resultAccount, err := repos.GetAccount(ctx, mappings[RoleCurrentResult].AccountID)
	if err != nil {
		return AnnualClosingWorkpaper{}, err
	}
	if resultAccount.Class != AccountEquity || !resultAccount.Postable {
		return AnnualClosingWorkpaper{}, fmt.Errorf("%w: current-result account must be a postable equity account", ErrConflict)
	}
	if resultAccount.ArchivedAt != nil {
		return AnnualClosingWorkpaper{}, ErrAccountArchived
	}
	workpaper, err := buildAnnualClosingWorkpaper(
		trial,
		command.FunctionalCurrency,
		resultAccount.ID,
		scope.ActorID,
		command.IdempotencyKey,
		s.clock.Now(),
		s.ids,
	)
	if err != nil {
		return AnnualClosingWorkpaper{}, err
	}
	for index := range workpaper.Lines {
		if workpaper.Lines[index].AccountID == resultAccount.ID {
			workpaper.Lines[index].AccountCode = resultAccount.Code
			workpaper.Lines[index].AccountName = resultAccount.Name
		}
	}
	return workpaper, nil
}

func validateAnnualClosingCommand(command AnnualClosingCommand) error {
	if command.From.IsZero() || command.To.IsZero() || command.To.Before(command.From) ||
		strings.TrimSpace(command.IdempotencyKey) == "" {
		return fmt.Errorf("%w: annual closing dates and idempotency key are required", ErrInvalidArgument)
	}
	return nil
}

func annualClosingSourceID(from, to time.Time) string {
	return from.Format("2006-01-02") + ":" + to.Format("2006-01-02")
}

func (s *Service) postEntryInTx(
	ctx context.Context,
	repos Repositories,
	scope Scope,
	entry JournalEntry,
) (JournalEntry, error) {
	posted, _, err := s.postEntryWithStatusInTx(ctx, repos, scope, entry)
	return posted, err
}

func (s *Service) postEntryWithStatusInTx(
	ctx context.Context,
	repos Repositories,
	scope Scope,
	entry JournalEntry,
) (JournalEntry, bool, error) {
	entry.CreatedBy = scope.ActorID
	if entry.ID == uuid.Nil {
		entry.ID = s.ids.NewID()
	}
	if err := entry.ValidateForPosting(); err != nil {
		return JournalEntry{}, false, err
	}
	existing, err := repos.FindEntryBySource(ctx, entry.Source)
	if err == nil {
		if existing.Source.IdempotencyKey != entry.Source.IdempotencyKey ||
			existing.Source.Event != entry.Source.Event {
			return JournalEntry{}, false, ErrIdempotencyConflict
		}
		return existing, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return JournalEntry{}, false, err
	}
	period, err := repos.FindPeriodForDate(ctx, entry.Date, true)
	if err != nil {
		return JournalEntry{}, false, err
	}
	// The period row serializes normal posting for a date. Recheck after taking
	// the lock so concurrent replays return the first committed entry.
	existing, err = repos.FindEntryBySource(ctx, entry.Source)
	if err == nil {
		if existing.Source.IdempotencyKey != entry.Source.IdempotencyKey ||
			existing.Source.Event != entry.Source.Event {
			return JournalEntry{}, false, ErrIdempotencyConflict
		}
		return existing, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return JournalEntry{}, false, err
	}
	if period.Status == PeriodLocked ||
		(period.Status == PeriodSoftClosed && (!entry.IsAdjustment || !scope.CanPostAdjustments)) {
		return JournalEntry{}, false, ErrPeriodClosed
	}
	seen := make(map[uuid.UUID]struct{}, len(entry.Lines))
	for _, line := range entry.Lines {
		if _, ok := seen[line.AccountID]; ok {
			continue
		}
		seen[line.AccountID] = struct{}{}
		account, accountErr := repos.GetAccount(ctx, line.AccountID)
		if accountErr != nil {
			return JournalEntry{}, false, accountErr
		}
		if account.ArchivedAt != nil {
			return JournalEntry{}, false, ErrAccountArchived
		}
		if !account.Postable {
			return JournalEntry{}, false, ErrAccountNotPostable
		}
	}
	posted, err := repos.PostEntry(ctx, entry)
	return posted, err == nil, err
}

func (s *Service) withTenant(
	ctx context.Context,
	scope Scope,
	fn func(context.Context, Repositories) error,
) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	return s.transactor.WithinTenant(ctx, scope, fn)
}

func validateAccountForMutation(account Account) error {
	if err := account.Validate(); err != nil {
		return err
	}
	if account.Postable && account.Monetary == NotApplicable {
		return fmt.Errorf("%w: postable account requires a monetary classification", ErrInvalidArgument)
	}
	return nil
}

func validatePeriodTransition(from, to PeriodStatus) error {
	if from == to {
		return ErrConflict
	}
	switch from {
	case PeriodOpen:
		if to == PeriodSoftClosed {
			return nil
		}
	case PeriodSoftClosed:
		if to == PeriodOpen || to == PeriodLocked {
			return nil
		}
	case PeriodLocked:
		if to == PeriodOpen {
			return nil
		}
	}
	return fmt.Errorf("%w: invalid period transition %s -> %s", ErrConflict, from, to)
}

func normalizeDraftLines(draft *Draft) {
	for index := range draft.Lines {
		line := &draft.Lines[index]
		line.LineNo = index + 1
		if line.TransactionDebit.IsZero() && line.TransactionCredit.IsZero() {
			line.TransactionDebit = line.Debit
			line.TransactionCredit = line.Credit
			line.Currency = draft.FunctionalCurrency
			line.ExchangeRate = One
			line.ExchangeRateDate = time.Time{}
			line.ExchangeRateSource = ""
			continue
		}
		if line.ExchangeRate.IsZero() {
			line.Currency = draft.Currency
			line.ExchangeRate = draft.ExchangeRate
			line.ExchangeRateDate = draft.ExchangeRateDate
			line.ExchangeRateSource = draft.ExchangeRateSource
		}
	}
}
