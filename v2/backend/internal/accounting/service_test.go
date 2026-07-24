package accounting

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServicePostsIdempotentlyInsideTenantBoundary(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	entry := manualEntryFixture(repository)

	first, err := service.PostEntry(context.Background(), scope, entry)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.PostEntry(context.Background(), scope, entry)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent replay IDs = %s and %s", first.ID, second.ID)
	}
	if repository.postCount != 1 {
		t.Fatalf("repository post count = %d, want 1", repository.postCount)
	}
	if repository.lastScope.OrganizationID != scope.OrganizationID {
		t.Fatalf("tenant scope = %s, want %s", repository.lastScope.OrganizationID, scope.OrganizationID)
	}
	changedIntent := entry
	changedIntent.Source.Event = "manual.changed-payload"
	if _, err := service.PostEntry(
		context.Background(),
		scope,
		changedIntent,
	); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed idempotent intent error = %v", err)
	}
}

func TestServicePostingPlanReplayDoesNotDuplicateOpenItemEffects(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	entry := manualEntryFixture(repository)
	for index := range entry.Lines {
		entry.Lines[index].ID = uuid.New()
	}
	partyID := uuid.New()
	plan := PostingPlan{
		Entry: entry,
		OpenItems: []OpenItem{{
			ID:               uuid.New(),
			Kind:             Receivable,
			PartyID:          partyID,
			AccountID:        entry.Lines[0].AccountID,
			SourceType:       entry.Source.Type,
			SourceID:         entry.Source.ID,
			IssueDate:        entry.Date,
			DueDate:          entry.Date,
			Currency:         MustCurrency("ARS"),
			OriginalAmount:   MustDecimal("100"),
			FunctionalAmount: MustDecimal("100"),
			OpenAmount:       MustDecimal("100"),
			OpenFunctional:   MustDecimal("100"),
		}},
	}
	first, err := service.PostPlan(context.Background(), scope, plan)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.PostPlan(context.Background(), scope, plan)
	if err != nil {
		t.Fatal(err)
	}
	if first.Entry.ID != second.Entry.ID {
		t.Fatalf("idempotent entry IDs = %s and %s", first.Entry.ID, second.Entry.ID)
	}
	if len(repository.openItems) != 1 {
		t.Fatalf("open item count = %d, want 1", len(repository.openItems))
	}
}

func TestServiceListsTenantScopedOpenItems(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	receivableID := uuid.New()
	payableID := uuid.New()
	repository.openItems[receivableID] = OpenItem{
		ID:       receivableID,
		Kind:     Receivable,
		PartyID:  uuid.New(),
		Currency: MustCurrency("ARS"),
	}
	repository.openItems[payableID] = OpenItem{
		ID:       payableID,
		Kind:     Payable,
		PartyID:  uuid.New(),
		Currency: MustCurrency("USD"),
	}

	items, err := service.ListOpenItems(
		context.Background(),
		scope,
		OpenItemFilter{Kind: Receivable},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != receivableID {
		t.Fatalf("receivable list = %#v", items)
	}
	if repository.lastScope.OrganizationID != scope.OrganizationID {
		t.Fatalf(
			"tenant scope = %s, want %s",
			repository.lastScope.OrganizationID,
			scope.OrganizationID,
		)
	}
}

func TestServiceReversalLeavesOriginalByteForByteAndIsIdempotent(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	posted, err := service.PostEntry(context.Background(), scope, manualEntryFixture(repository))
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(repository.entries[posted.ID])
	if err != nil {
		t.Fatal(err)
	}
	command := ReverseEntryCommand{
		EntryID:        posted.ID,
		Date:           posted.Date,
		Reason:         "Corrección de imputación",
		IdempotencyKey: "reverse-1",
	}
	reversal, err := service.ReverseEntry(context.Background(), scope, command)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(repository.entries[posted.ID])
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("original changed\nbefore=%s\nafter=%s", before, after)
	}
	if reversal.ReversesEntryID == nil || *reversal.ReversesEntryID != posted.ID {
		t.Fatalf("reversal link = %v", reversal.ReversesEntryID)
	}
	for index := range posted.Lines {
		if !reversal.Lines[index].Debit.Equal(posted.Lines[index].Credit) ||
			!reversal.Lines[index].Credit.Equal(posted.Lines[index].Debit) {
			t.Fatalf("line %d was not exactly inverted", index+1)
		}
	}
	replayed, err := service.ReverseEntry(context.Background(), scope, command)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != reversal.ID {
		t.Fatalf("replayed reversal ID = %s, want %s", replayed.ID, reversal.ID)
	}
	command.IdempotencyKey = "reverse-different-intent"
	if _, err := service.ReverseEntry(context.Background(), scope, command); !errors.Is(err, ErrAlreadyReversed) {
		t.Fatalf("second direct reversal error = %v", err)
	}

	reversalOfReversal, err := service.ReverseEntry(
		context.Background(),
		scope,
		ReverseEntryCommand{
			EntryID:        reversal.ID,
			Date:           reversal.Date,
			Reason:         "Restablecer el asiento original",
			IdempotencyKey: "reverse-the-reversal",
		},
	)
	if err != nil {
		t.Fatalf("reverse reversal: %v", err)
	}
	if reversalOfReversal.ReversesEntryID == nil ||
		*reversalOfReversal.ReversesEntryID != reversal.ID {
		t.Fatalf("reversal-of-reversal link = %v", reversalOfReversal.ReversesEntryID)
	}
	for index := range posted.Lines {
		if !reversalOfReversal.Lines[index].Debit.Equal(posted.Lines[index].Debit) ||
			!reversalOfReversal.Lines[index].Credit.Equal(posted.Lines[index].Credit) {
			t.Fatalf("reversal-of-reversal line %d does not restore original direction", index+1)
		}
	}
}

func TestServiceReversalEligibilityAndDateRules(t *testing.T) {
	t.Parallel()

	t.Run("legacy manual entry without a source is reversible", func(t *testing.T) {
		repository, service, scope := serviceFixture(t)
		entry := manualEntryFixture(repository)
		entry.ID = uuid.New()
		entry.Number = 1
		entry.Source = EntrySource{}
		repository.entries[entry.ID] = cloneEntry(entry)

		if _, err := service.ReverseEntry(
			context.Background(),
			scope,
			ReverseEntryCommand{
				EntryID:        entry.ID,
				Date:           entry.Date,
				Reason:         "Corrección de asiento legacy",
				IdempotencyKey: "reverse-legacy-manual",
			},
		); err != nil {
			t.Fatalf("reverse legacy manual entry: %v", err)
		}
	})

	t.Run("adjustment sources remain reversible unless documentary", func(t *testing.T) {
		repository, service, scope := serviceFixture(t)
		entry := manualEntryFixture(repository)
		entry.Kind = EntryAdjustment
		entry.Source.Type = "inflation"
		entry.Source.Event = "adjustment"
		entry.Source.IdempotencyKey = "inflation-adjustment"
		posted, err := service.PostEntry(context.Background(), scope, entry)
		if err != nil {
			t.Fatalf("post adjustment: %v", err)
		}
		if _, err := service.ReverseEntry(
			context.Background(),
			scope,
			ReverseEntryCommand{
				EntryID:        posted.ID,
				Date:           posted.Date,
				Reason:         "Corregir ajuste",
				IdempotencyKey: "reverse-adjustment",
			},
		); err != nil {
			t.Fatalf("reverse adjustment: %v", err)
		}

		documentary := manualEntryFixture(repository)
		documentary.Kind = EntryAdjustment
		documentary.Source.Type = "sale"
		documentary.Source.ID = uuid.New()
		documentary.Source.Event = "adjustment"
		documentary.Source.IdempotencyKey = "sale-adjustment"
		postedDocumentary, err := service.PostEntry(context.Background(), scope, documentary)
		if err != nil {
			t.Fatalf("post documentary adjustment fixture: %v", err)
		}
		if _, err := service.ReverseEntry(
			context.Background(),
			scope,
			ReverseEntryCommand{
				EntryID:        postedDocumentary.ID,
				Date:           postedDocumentary.Date,
				Reason:         "No debe permitirse",
				IdempotencyKey: "reverse-documentary-adjustment",
			},
		); !errors.Is(err, ErrReversalNotAllowed) {
			t.Fatalf("documentary adjustment reversal error = %v", err)
		}
	})

	t.Run("open item effects block reversal", func(t *testing.T) {
		repository, service, scope := serviceFixture(t)
		posted, err := service.PostEntry(
			context.Background(),
			scope,
			manualEntryFixture(repository),
		)
		if err != nil {
			t.Fatal(err)
		}
		repository.openItems[uuid.New()] = OpenItem{
			ID:      uuid.New(),
			EntryID: posted.ID,
		}
		if _, err := service.ReverseEntry(
			context.Background(),
			scope,
			ReverseEntryCommand{
				EntryID:        posted.ID,
				Date:           posted.Date,
				Reason:         "No debe permitirse",
				IdempotencyKey: "reverse-open-item",
			},
		); !errors.Is(err, ErrReversalNotAllowed) {
			t.Fatalf("open-item reversal error = %v", err)
		}
	})

	t.Run("reversal date cannot precede original", func(t *testing.T) {
		repository, service, scope := serviceFixture(t)
		posted, err := service.PostEntry(
			context.Background(),
			scope,
			manualEntryFixture(repository),
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.ReverseEntry(
			context.Background(),
			scope,
			ReverseEntryCommand{
				EntryID:        posted.ID,
				Date:           posted.Date.AddDate(0, 0, -1),
				Reason:         "Fecha inválida",
				IdempotencyKey: "reverse-before-original",
			},
		); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("earlier reversal date error = %v", err)
		}
	})

	t.Run("concurrent direct reversal is rechecked after the period lock", func(t *testing.T) {
		repository, service, scope := serviceFixture(t)
		posted, err := service.PostEntry(
			context.Background(),
			scope,
			manualEntryFixture(repository),
		)
		if err != nil {
			t.Fatal(err)
		}
		repository.directReversalOnCall = 2
		repository.concurrentReversal = JournalEntry{
			ID:   uuid.New(),
			Date: posted.Date,
			Source: EntrySource{
				Type:           "journal_entry",
				ID:             posted.ID,
				Event:          "reversal",
				IdempotencyKey: "concurrent-reversal",
			},
			ReversesEntryID: &posted.ID,
		}

		if _, err := service.ReverseEntry(
			context.Background(),
			scope,
			ReverseEntryCommand{
				EntryID:        posted.ID,
				Date:           posted.Date,
				Reason:         "Compite con otra reversa",
				IdempotencyKey: "losing-reversal",
			},
		); !errors.Is(err, ErrAlreadyReversed) {
			t.Fatalf("concurrent reversal error = %v", err)
		}
	})
}

func TestServiceRejectsPostingAgainstClosedReconciliation(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	repository.closedReconciliation = true
	if _, err := service.PostEntry(
		context.Background(),
		scope,
		manualEntryFixture(repository),
	); !errors.Is(err, ErrReconciliationClosed) {
		t.Fatalf("closed-reconciliation posting error = %v", err)
	}
	if repository.postCount != 0 {
		t.Fatalf("posted entries = %d, want 0", repository.postCount)
	}
}

func TestServiceRejectsPostingToLockedAndNonAdjustmentToSoftClosedPeriod(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	entry := manualEntryFixture(repository)
	period := repository.periods[repository.periodID]
	period.Status = PeriodLocked
	repository.periods[period.ID] = period
	if _, err := service.PostEntry(context.Background(), scope, entry); !errors.Is(err, ErrPeriodClosed) {
		t.Fatalf("locked-period error = %v", err)
	}

	period.Status = PeriodSoftClosed
	repository.periods[period.ID] = period
	entry.Source.ID = uuid.New()
	entry.Source.IdempotencyKey = "manual-soft-closed"
	if _, err := service.PostEntry(context.Background(), scope, entry); !errors.Is(err, ErrPeriodClosed) {
		t.Fatalf("soft-closed error = %v", err)
	}
	entry.IsAdjustment = true
	entry.Kind = EntryAdjustment
	scope.CanPostAdjustments = true
	if _, err := service.PostEntry(context.Background(), scope, entry); err != nil {
		t.Fatalf("adjustment in soft-closed period: %v", err)
	}

	manualFlaggedAsAdjustment := manualEntryFixture(repository)
	manualFlaggedAsAdjustment.IsAdjustment = true
	manualFlaggedAsAdjustment.Source.ID = uuid.New()
	manualFlaggedAsAdjustment.Source.IdempotencyKey = "manual-flagged-adjustment"
	if _, err := service.PostEntry(
		context.Background(),
		scope,
		manualFlaggedAsAdjustment,
	); !errors.Is(err, ErrPeriodClosed) {
		t.Fatalf("manual entry flagged as adjustment error = %v", err)
	}

	inflation := manualEntryFixture(repository)
	inflation.Kind = EntryInflation
	inflation.IsAdjustment = false
	inflation.Source.Type = "inflation"
	inflation.Source.ID = uuid.New()
	inflation.Source.IdempotencyKey = "inflation-soft-closed"
	if _, err := service.PostEntry(context.Background(), scope, inflation); err != nil {
		t.Fatalf("inflation in soft-closed period: %v", err)
	}
}

func TestServicePeriodCloseChecklistAndAuditedReopenRules(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	repository.checklist = CloseChecklist{UnpostedDocuments: 1}
	period := repository.periods[repository.periodID]
	if _, checklist, err := service.TransitionPeriod(
		context.Background(),
		scope,
		period.ID,
		period.Version,
		PeriodSoftClosed,
		"",
	); !errors.Is(err, ErrConflict) || checklist.BlockingCount() != 1 {
		t.Fatalf("blocked close = checklist %+v error %v", checklist, err)
	}

	repository.checklist = CloseChecklist{}
	softClosed, _, err := service.TransitionPeriod(
		context.Background(),
		scope,
		period.ID,
		period.Version,
		PeriodSoftClosed,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	locked, _, err := service.TransitionPeriod(
		context.Background(),
		scope,
		period.ID,
		softClosed.Version,
		PeriodLocked,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.TransitionPeriod(
		context.Background(),
		scope,
		period.ID,
		locked.Version,
		PeriodOpen,
		"",
	); err == nil {
		t.Fatal("reopen without reason unexpectedly succeeded")
	}
	scope.CanReopenPeriods = true
	reopened, _, err := service.TransitionPeriod(
		context.Background(),
		scope,
		period.ID,
		locked.Version,
		PeriodOpen,
		"Ajuste de auditoría",
	)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ReopenedReason != "Ajuste de auditoría" || reopened.ReopenedBy != scope.ActorID {
		t.Fatalf("reopen audit = %+v", reopened)
	}
}

func TestServiceCreatesCurrencyRevaluationIdempotentlyFromTenantPositions(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	gainID := uuid.New()
	lossID := uuid.New()
	repository.mappings[RoleFXGain] = AccountMapping{
		Role:      RoleFXGain,
		AccountID: gainID,
		Version:   1,
	}
	repository.mappings[RoleFXLoss] = AccountMapping{
		Role:      RoleFXLoss,
		AccountID: lossID,
		Version:   1,
	}
	repository.fxPositions = []CurrencyRevaluationPosition{{
		AccountID:      uuid.New(),
		AccountCode:    "1.1.20",
		AccountName:    "Banco USD",
		NormalBalance:  NormalDebit,
		Currency:       MustCurrency("USD"),
		CurrencyAmount: MustDecimal("10"),
		CarryingAmount: MustDecimal("9000"),
	}}
	rate, err := NewClosingExchangeRate(
		dateFixture(),
		MustCurrency("USD"),
		MustCurrency("ARS"),
		MustDecimal("1000"),
		"BNA",
		"",
		[]byte("rate"),
	)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.CreateCurrencyRevaluation(
		context.Background(),
		scope,
		dateFixture(),
		MustCurrency("ARS"),
		[]ClosingExchangeRate{rate},
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateCurrencyRevaluation(
		context.Background(),
		scope,
		dateFixture(),
		MustCurrency("ARS"),
		[]ClosingExchangeRate{rate},
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || len(repository.revaluations) != 1 {
		t.Fatalf("revaluation replay = %s/%s, stored %d", first.ID, second.ID, len(repository.revaluations))
	}
	if first.SourceChecksum != second.SourceChecksum {
		t.Fatalf("revaluation checksums = %s/%s", first.SourceChecksum, second.SourceChecksum)
	}
}

func TestServicePostDraftUsesOptimisticVersionAndMarksDraftPosted(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	entry := manualEntryFixture(repository)
	draftID := uuid.New()
	draft := Draft{
		ID:                 draftID,
		Version:            3,
		IdempotencyKey:     "draft-create",
		Date:               entry.Date,
		Kind:               EntryManual,
		FunctionalCurrency: entry.FunctionalCurrency,
		Currency:           entry.Currency,
		ExchangeRate:       entry.ExchangeRate,
		Description:        entry.Description,
		Lines:              entry.Lines,
		CreatedBy:          scope.ActorID,
		UpdatedBy:          scope.ActorID,
	}
	repository.drafts[draftID] = draft
	if _, err := service.PostDraft(context.Background(), scope, draftID, 2, "draft-post"); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale draft error = %v", err)
	}
	posted, err := service.PostDraft(context.Background(), scope, draftID, 3, "draft-post")
	if err != nil {
		t.Fatal(err)
	}
	if repository.postedDraft[draftID] != posted.ID {
		t.Fatalf("posted draft link = %s, want %s", repository.postedDraft[draftID], posted.ID)
	}
	replayed, err := service.PostDraft(
		context.Background(),
		scope,
		draftID,
		3,
		"draft-post",
	)
	if err != nil {
		t.Fatalf("replay posted draft: %v", err)
	}
	if replayed.ID != posted.ID || repository.postCount != 1 {
		t.Fatalf(
			"posted draft replay = %s, count %d; want %s, 1",
			replayed.ID,
			repository.postCount,
			posted.ID,
		)
	}
}

func TestServiceCreateDraftReplaysSameIntentAndRejectsKeyReuse(t *testing.T) {
	t.Parallel()

	repository, service, scope := serviceFixture(t)
	draft := Draft{
		IdempotencyKey:     "draft-create-replay",
		Date:               dateFixture(),
		Kind:               EntryManual,
		FunctionalCurrency: MustCurrency("ARS"),
		Currency:           MustCurrency("ARS"),
		ExchangeRate:       One,
		Description:        "",
	}
	first, err := service.CreateDraft(context.Background(), scope, draft)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateDraft(context.Background(), scope, draft)
	if err != nil {
		t.Fatalf("replay draft create: %v", err)
	}
	if second.ID != first.ID || len(repository.drafts) != 1 {
		t.Fatalf(
			"draft replay = %s/%s, stored %d",
			first.ID,
			second.ID,
			len(repository.drafts),
		)
	}
	draft.Description = "different intent"
	if _, err := service.CreateDraft(
		context.Background(),
		scope,
		draft,
	); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("draft key reuse error = %v", err)
	}
}

func serviceFixture(t *testing.T) (*serviceRepository, *Service, Scope) {
	t.Helper()
	repository := newServiceRepository()
	transactor := &serviceTransactor{repository: repository}
	service, err := NewServiceWithDependencies(transactor, UUIDGenerator{}, fixedClock{value: dateFixture()})
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{
		OrganizationID:      uuid.New(),
		ActorID:             "user_1",
		CanManageAccounting: true,
	}
	transactor.scopeTarget = &repository.lastScope
	return repository, service, scope
}

func manualEntryFixture(repository *serviceRepository) JournalEntry {
	debitAccount := Account{
		ID:            uuid.New(),
		Code:          "1.1.01",
		Name:          "Caja",
		Class:         AccountAsset,
		NormalBalance: NormalDebit,
		Monetary:      Monetary,
		Postable:      true,
		Version:       1,
	}
	creditAccount := Account{
		ID:            uuid.New(),
		Code:          "3.1.01",
		Name:          "Capital",
		Class:         AccountEquity,
		NormalBalance: NormalCredit,
		Monetary:      NonMonetary,
		Postable:      true,
		Version:       1,
	}
	repository.accounts[debitAccount.ID] = debitAccount
	repository.accounts[creditAccount.ID] = creditAccount
	return JournalEntry{
		Date:               dateFixture(),
		Kind:               EntryManual,
		PostingKind:        "primary",
		FunctionalCurrency: MustCurrency("ARS"),
		Currency:           MustCurrency("ARS"),
		ExchangeRate:       One,
		Source: EntrySource{
			Type:           "manual",
			ID:             uuid.New(),
			Event:          "primary",
			IdempotencyKey: "manual-1",
		},
		Description: "Aporte inicial",
		CreatedBy:   "ignored-by-service",
		Lines: []JournalLine{
			functionalLine(debitAccount.ID, debitSide, MustDecimal("100"), MustCurrency("ARS"), nil, "Aporte"),
			functionalLine(creditAccount.ID, creditSide, MustDecimal("100"), MustCurrency("ARS"), nil, "Aporte"),
		},
	}
}

type fixedClock struct {
	value time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.value
}

type serviceTransactor struct {
	repository  *serviceRepository
	scopeTarget *Scope
}

func (transactor *serviceTransactor) WithinTenant(
	ctx context.Context,
	scope Scope,
	fn func(context.Context, Repositories) error,
) error {
	if transactor.scopeTarget != nil {
		*transactor.scopeTarget = scope
	}
	return fn(ctx, transactor.repository)
}

type serviceRepository struct {
	accounts             map[uuid.UUID]Account
	mappings             map[string]AccountMapping
	drafts               map[uuid.UUID]Draft
	postedDraft          map[uuid.UUID]uuid.UUID
	entries              map[uuid.UUID]JournalEntry
	periods              map[uuid.UUID]Period
	periodID             uuid.UUID
	checklist            CloseChecklist
	postCount            int
	lastScope            Scope
	nextNumber           int64
	openItems            map[uuid.UUID]OpenItem
	reconciliation       map[uuid.UUID]Reconciliation
	workpapers           map[uuid.UUID]InflationWorkpaper
	revaluations         map[uuid.UUID]CurrencyRevaluationWorkpaper
	fxPositions          []CurrencyRevaluationPosition
	reportLines          []ReportLine
	closedReconciliation bool
	directReversalOnCall int
	directReversalCalls  int
	concurrentReversal   JournalEntry
}

func newServiceRepository() *serviceRepository {
	periodID := uuid.New()
	return &serviceRepository{
		accounts:    make(map[uuid.UUID]Account),
		mappings:    make(map[string]AccountMapping),
		drafts:      make(map[uuid.UUID]Draft),
		postedDraft: make(map[uuid.UUID]uuid.UUID),
		entries:     make(map[uuid.UUID]JournalEntry),
		periods: map[uuid.UUID]Period{
			periodID: {
				ID:        periodID,
				Name:      "2026-07",
				StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
				Status:    PeriodOpen,
				Version:   1,
			},
		},
		periodID:       periodID,
		nextNumber:     1,
		openItems:      make(map[uuid.UUID]OpenItem),
		reconciliation: make(map[uuid.UUID]Reconciliation),
		workpapers:     make(map[uuid.UUID]InflationWorkpaper),
		revaluations:   make(map[uuid.UUID]CurrencyRevaluationWorkpaper),
	}
}

func (repository *serviceRepository) ListAccounts(_ context.Context, includeArchived bool) ([]Account, error) {
	result := make([]Account, 0, len(repository.accounts))
	for _, account := range repository.accounts {
		if includeArchived || account.ArchivedAt == nil {
			result = append(result, account)
		}
	}
	return result, nil
}

func (repository *serviceRepository) GetAccount(_ context.Context, id uuid.UUID) (Account, error) {
	account, ok := repository.accounts[id]
	if !ok {
		return Account{}, ErrNotFound
	}
	return account, nil
}

func (repository *serviceRepository) CreateAccount(_ context.Context, account Account) (Account, error) {
	repository.accounts[account.ID] = account
	return account, nil
}

func (repository *serviceRepository) UpdateAccount(_ context.Context, account Account, version int64) (Account, error) {
	current, ok := repository.accounts[account.ID]
	if !ok {
		return Account{}, ErrNotFound
	}
	if current.Version != version {
		return Account{}, ErrVersionConflict
	}
	account.Version = version + 1
	repository.accounts[account.ID] = account
	return account, nil
}

func (repository *serviceRepository) AccountUsage(_ context.Context, id uuid.UUID) (int64, int64, int64, error) {
	if _, ok := repository.accounts[id]; !ok {
		return 0, 0, 0, ErrNotFound
	}
	return 0, 0, 0, nil
}

func (repository *serviceRepository) ArchiveAccount(_ context.Context, id uuid.UUID, version int64, at time.Time, _ string) (Account, error) {
	account, err := repository.GetAccount(context.Background(), id)
	if err != nil {
		return Account{}, err
	}
	if account.Version != version {
		return Account{}, ErrVersionConflict
	}
	account.ArchivedAt = &at
	account.Version++
	repository.accounts[id] = account
	return account, nil
}

func (repository *serviceRepository) RestoreAccount(_ context.Context, id uuid.UUID, version int64, _ string) (Account, error) {
	account, err := repository.GetAccount(context.Background(), id)
	if err != nil {
		return Account{}, err
	}
	if account.Version != version {
		return Account{}, ErrVersionConflict
	}
	account.ArchivedAt = nil
	account.Version++
	repository.accounts[id] = account
	return account, nil
}

func (repository *serviceRepository) DeleteUnusedAccount(_ context.Context, id uuid.UUID, version int64) error {
	account, ok := repository.accounts[id]
	if !ok {
		return ErrNotFound
	}
	if account.Version != version {
		return ErrVersionConflict
	}
	delete(repository.accounts, id)
	return nil
}

func (repository *serviceRepository) ListMappings(context.Context) ([]AccountMapping, error) {
	result := make([]AccountMapping, 0, len(repository.mappings))
	for _, mapping := range repository.mappings {
		result = append(result, mapping)
	}
	return result, nil
}

func (repository *serviceRepository) GetMappings(_ context.Context, roles []string) (map[string]AccountMapping, error) {
	result := make(map[string]AccountMapping, len(roles))
	for _, role := range roles {
		mapping, ok := repository.mappings[role]
		if !ok {
			return nil, ErrMappingMissing
		}
		result[role] = mapping
	}
	return result, nil
}

func (repository *serviceRepository) SetMapping(_ context.Context, mapping AccountMapping, version int64) (AccountMapping, error) {
	if current, ok := repository.mappings[mapping.Role]; ok && current.Version != version {
		return AccountMapping{}, ErrVersionConflict
	}
	mapping.Version = max(version+1, 1)
	repository.mappings[mapping.Role] = mapping
	return mapping, nil
}

func (repository *serviceRepository) CreateDraft(_ context.Context, draft Draft) (Draft, error) {
	repository.drafts[draft.ID] = draft
	return draft, nil
}

func (repository *serviceRepository) GetDraft(_ context.Context, id uuid.UUID, _ bool) (Draft, error) {
	draft, ok := repository.drafts[id]
	if !ok {
		return Draft{}, ErrNotFound
	}
	return draft, nil
}

func (repository *serviceRepository) FindDraftByIdempotency(
	_ context.Context,
	idempotencyKey string,
) (Draft, error) {
	for _, draft := range repository.drafts {
		if draft.IdempotencyKey == idempotencyKey {
			return draft, nil
		}
	}
	return Draft{}, ErrNotFound
}

func (repository *serviceRepository) UpdateDraft(_ context.Context, draft Draft, version int64) (Draft, error) {
	current, ok := repository.drafts[draft.ID]
	if !ok {
		return Draft{}, ErrNotFound
	}
	if current.Version != version {
		return Draft{}, ErrVersionConflict
	}
	draft.Version = version + 1
	repository.drafts[draft.ID] = draft
	return draft, nil
}

func (repository *serviceRepository) DiscardDraft(
	_ context.Context,
	id uuid.UUID,
	version int64,
	_ string,
	_ string,
) error {
	draft, ok := repository.drafts[id]
	if !ok {
		return ErrNotFound
	}
	if draft.Version != version {
		return ErrVersionConflict
	}
	delete(repository.drafts, id)
	return nil
}

func (repository *serviceRepository) MarkDraftPosted(_ context.Context, id uuid.UUID, version int64, entryID uuid.UUID) error {
	draft, ok := repository.drafts[id]
	if !ok {
		return ErrNotFound
	}
	if draft.Version != version {
		return ErrVersionConflict
	}
	repository.postedDraft[id] = entryID
	return nil
}

func (repository *serviceRepository) PostEntry(_ context.Context, entry JournalEntry) (JournalEntry, error) {
	repository.postCount++
	entry.Number = repository.nextNumber
	repository.nextNumber++
	entry.CreatedAt = dateFixture()
	repository.entries[entry.ID] = cloneEntry(entry)
	return cloneEntry(entry), nil
}

func (repository *serviceRepository) GetEntry(_ context.Context, id uuid.UUID) (JournalEntry, error) {
	entry, ok := repository.entries[id]
	if !ok {
		return JournalEntry{}, ErrNotFound
	}
	return cloneEntry(entry), nil
}

func (repository *serviceRepository) FindEntryBySource(_ context.Context, source EntrySource) (JournalEntry, error) {
	for _, entry := range repository.entries {
		if entry.Source.Type == source.Type && entry.Source.ID == source.ID {
			return cloneEntry(entry), nil
		}
	}
	return JournalEntry{}, ErrNotFound
}

func (repository *serviceRepository) FindDirectReversal(_ context.Context, id uuid.UUID) (JournalEntry, error) {
	repository.directReversalCalls++
	if repository.directReversalOnCall > 0 &&
		repository.directReversalCalls >= repository.directReversalOnCall &&
		repository.concurrentReversal.ID != uuid.Nil {
		return cloneEntry(repository.concurrentReversal), nil
	}
	for _, entry := range repository.entries {
		if entry.ReversesEntryID != nil && *entry.ReversesEntryID == id {
			return cloneEntry(entry), nil
		}
	}
	return JournalEntry{}, ErrNotFound
}

func (repository *serviceRepository) EntryHasOpenItemEffects(
	_ context.Context,
	id uuid.UUID,
) (bool, error) {
	for _, item := range repository.openItems {
		if item.EntryID == id {
			return true, nil
		}
	}
	return false, nil
}

func (repository *serviceRepository) TouchesClosedReconciliation(
	context.Context,
	time.Time,
	[]uuid.UUID,
) (bool, error) {
	return repository.closedReconciliation, nil
}

func (repository *serviceRepository) ListJournal(context.Context, JournalFilter) (PageResult[JournalEntry], error) {
	return PageResult[JournalEntry]{}, nil
}

func (repository *serviceRepository) ReportLines(context.Context, time.Time, time.Time) ([]ReportLine, error) {
	return append([]ReportLine(nil), repository.reportLines...), nil
}

func (repository *serviceRepository) AccountOpeningBalance(context.Context, uuid.UUID, time.Time) (Decimal, error) {
	return Zero, nil
}

func (repository *serviceRepository) ListOpenItems(
	_ context.Context,
	filter OpenItemFilter,
) ([]OpenItem, error) {
	result := make([]OpenItem, 0, len(repository.openItems))
	for _, item := range repository.openItems {
		if filter.Kind != "" && item.Kind != filter.Kind {
			continue
		}
		if filter.PartyID != nil && item.PartyID != *filter.PartyID {
			continue
		}
		if filter.Currency != nil && item.Currency != *filter.Currency {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (repository *serviceRepository) CreateOpenItem(_ context.Context, item OpenItem) (OpenItem, error) {
	repository.openItems[item.ID] = item
	return item, nil
}

func (repository *serviceRepository) ApplyOpenItem(_ context.Context, application OpenItemApplication) (OpenItem, error) {
	item, ok := repository.openItems[application.OpenItemID]
	if !ok {
		return OpenItem{}, ErrNotFound
	}
	item.OpenAmount = item.OpenAmount.Sub(application.Amount)
	item.OpenFunctional = item.OpenFunctional.Sub(application.FunctionalAmount)
	repository.openItems[item.ID] = item
	return item, nil
}

func (repository *serviceRepository) ListPeriods(context.Context) ([]Period, error) {
	result := make([]Period, 0, len(repository.periods))
	for _, period := range repository.periods {
		result = append(result, period)
	}
	return result, nil
}

func (repository *serviceRepository) GetPeriod(_ context.Context, id uuid.UUID, _ bool) (Period, error) {
	period, ok := repository.periods[id]
	if !ok {
		return Period{}, ErrNotFound
	}
	return period, nil
}

func (repository *serviceRepository) FindPeriodForDate(_ context.Context, date time.Time, _ bool) (Period, error) {
	for _, period := range repository.periods {
		if !date.Before(period.StartDate) && !date.After(period.EndDate) {
			return period, nil
		}
	}
	return Period{}, ErrNotFound
}

func (repository *serviceRepository) CreatePeriod(_ context.Context, period Period) (Period, error) {
	repository.periods[period.ID] = period
	return period, nil
}

func (repository *serviceRepository) UpdatePeriod(_ context.Context, period Period, version int64) (Period, error) {
	current, ok := repository.periods[period.ID]
	if !ok {
		return Period{}, ErrNotFound
	}
	if current.Version != version {
		return Period{}, ErrVersionConflict
	}
	repository.periods[period.ID] = period
	return period, nil
}

func (repository *serviceRepository) CloseChecklist(context.Context, uuid.UUID) (CloseChecklist, error) {
	return repository.checklist, nil
}

func (repository *serviceRepository) CreateStatementImport(_ context.Context, statement StatementImport) (StatementImport, error) {
	return statement, nil
}

func (repository *serviceRepository) FindStatementImportByHash(context.Context, uuid.UUID, string) (StatementImport, error) {
	return StatementImport{}, ErrNotFound
}

func (repository *serviceRepository) ListStatementMovements(context.Context, uuid.UUID) ([]StatementMovement, error) {
	return nil, nil
}

func (repository *serviceRepository) CreateReconciliation(_ context.Context, reconciliation Reconciliation) (Reconciliation, error) {
	repository.reconciliation[reconciliation.ID] = reconciliation
	return reconciliation, nil
}

func (repository *serviceRepository) GetReconciliation(_ context.Context, id uuid.UUID, _ bool) (Reconciliation, error) {
	reconciliation, ok := repository.reconciliation[id]
	if !ok {
		return Reconciliation{}, ErrNotFound
	}
	return reconciliation, nil
}

func (repository *serviceRepository) ListReconciliations(context.Context, PageRequest) (PageResult[Reconciliation], error) {
	result := PageResult[Reconciliation]{}
	for _, reconciliation := range repository.reconciliation {
		result.Items = append(result.Items, reconciliation)
	}
	return result, nil
}

func (repository *serviceRepository) SaveReconciliation(_ context.Context, reconciliation Reconciliation, version int64) (Reconciliation, error) {
	if current, ok := repository.reconciliation[reconciliation.ID]; ok && current.Version != version {
		return Reconciliation{}, ErrVersionConflict
	}
	reconciliation.Version = version + 1
	repository.reconciliation[reconciliation.ID] = reconciliation
	return reconciliation, nil
}

func (repository *serviceRepository) ListReconciliationCandidates(context.Context, uuid.UUID, time.Time, time.Time) ([]ReconciliationLedgerCandidate, error) {
	return nil, nil
}

func (repository *serviceRepository) UpsertInflationIndices(context.Context, []InflationIndex) error {
	return nil
}

func (repository *serviceRepository) ListInflationIndices(context.Context, time.Time, time.Time) ([]InflationIndex, error) {
	return nil, nil
}

func (repository *serviceRepository) ListInflationPositions(context.Context, time.Time) ([]InflationPosition, error) {
	return nil, nil
}

func (repository *serviceRepository) CreateInflationWorkpaper(_ context.Context, workpaper InflationWorkpaper) (InflationWorkpaper, error) {
	repository.workpapers[workpaper.ID] = workpaper
	return workpaper, nil
}

func (repository *serviceRepository) GetInflationWorkpaper(_ context.Context, id uuid.UUID) (InflationWorkpaper, error) {
	workpaper, ok := repository.workpapers[id]
	if !ok {
		return InflationWorkpaper{}, ErrNotFound
	}
	return workpaper, nil
}

func (repository *serviceRepository) ListCurrencyRevaluationPositions(
	context.Context,
	time.Time,
	Currency,
) ([]CurrencyRevaluationPosition, error) {
	return append([]CurrencyRevaluationPosition(nil), repository.fxPositions...), nil
}

func (repository *serviceRepository) CreateCurrencyRevaluationWorkpaper(
	_ context.Context,
	workpaper CurrencyRevaluationWorkpaper,
) (CurrencyRevaluationWorkpaper, error) {
	for _, existing := range repository.revaluations {
		if existing.ClosingDate.Equal(workpaper.ClosingDate) &&
			existing.SourceChecksum == workpaper.SourceChecksum {
			return existing, nil
		}
	}
	repository.drafts[workpaper.Draft.ID] = workpaper.Draft
	repository.revaluations[workpaper.ID] = workpaper
	return workpaper, nil
}

func (repository *serviceRepository) GetCurrencyRevaluationWorkpaper(
	_ context.Context,
	id uuid.UUID,
) (CurrencyRevaluationWorkpaper, error) {
	workpaper, ok := repository.revaluations[id]
	if !ok {
		return CurrencyRevaluationWorkpaper{}, ErrNotFound
	}
	return workpaper, nil
}

func cloneEntry(entry JournalEntry) JournalEntry {
	entry.Lines = append([]JournalLine(nil), entry.Lines...)
	return entry
}
