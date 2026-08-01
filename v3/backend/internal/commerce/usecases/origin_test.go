package usecases

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
	identity "github.com/devpablocristo/pymes/v3/backend/internal/identity/domain"
	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases"
)

type capturedDeliveryIdentity struct {
	metadata     identityusecases.RequestMetadata
	hasMetadata  bool
	principal    identity.Principal
	hasPrincipal bool
	delegated    string
	hasDelegated bool
}

func captureDeliveryIdentity(ctx context.Context) capturedDeliveryIdentity {
	metadata, hasMetadata := identityusecases.RequestMetadataFromContext(ctx)
	principal, hasPrincipal := identityusecases.PrincipalFromContext(ctx)
	delegated, hasDelegated := identityusecases.DelegatedActorFromContext(ctx)
	return capturedDeliveryIdentity{
		metadata: metadata, hasMetadata: hasMetadata,
		principal: principal, hasPrincipal: hasPrincipal,
		delegated: delegated, hasDelegated: hasDelegated,
	}
}

type originRelayStore struct {
	RelayStore
	events          []domain.Event
	sale            domain.Sale
	purchase        domain.Purchase
	payment         domain.Payment
	application     domain.PendingAccountingApplication
	reversal        domain.AccountingReversal
	applied         []domain.PendingAccountingApplication
	uncertain       []domain.PendingFiscal
	consultAttempt  int
	retried         bool
	published       int
	fiscalApplied   bool
	applicationDone bool
	reversalDone    bool
}

func (s *originRelayStore) Lease(context.Context, int, time.Duration) ([]domain.Event, error) {
	return s.events, nil
}

func (s *originRelayStore) Retry(context.Context, domain.Event) error {
	s.retried = true
	return nil
}

func (s *originRelayStore) DeadLetter(context.Context, domain.Event, string) error {
	return nil
}

func (s *originRelayStore) MarkPublished(context.Context, domain.Event) error {
	s.published++
	return nil
}

func (s *originRelayStore) GetSale(context.Context, string, string) (domain.Sale, error) {
	return s.sale, nil
}

func (s *originRelayStore) ApplyFiscalResult(context.Context, domain.Sale, domain.FiscalResult) error {
	s.fiscalApplied = true
	return nil
}

func (s *originRelayStore) MarkSalePosted(context.Context, domain.Sale, domain.AccountingEvent) error {
	return nil
}

func (s *originRelayStore) GetPurchase(context.Context, string, string) (domain.Purchase, error) {
	return s.purchase, nil
}

func (s *originRelayStore) MarkPurchasePosted(context.Context, domain.Purchase, domain.AccountingEvent) error {
	return nil
}

func (s *originRelayStore) GetPayment(context.Context, string, string) (domain.Payment, error) {
	return s.payment, nil
}

func (s *originRelayStore) MarkPaymentPosted(context.Context, domain.Payment, domain.AccountingEvent) error {
	return nil
}

func (s *originRelayStore) GetAccountingApplication(
	context.Context, string, string,
) (domain.PendingAccountingApplication, error) {
	return s.application, nil
}

func (s *originRelayStore) MarkAccountingApplicationApplied(
	context.Context, domain.PendingAccountingApplication, domain.AccountingEvent,
) error {
	s.applicationDone = true
	return nil
}

func (s *originRelayStore) ListAppliedAccountingApplications(
	context.Context, string, string, string,
) ([]domain.PendingAccountingApplication, error) {
	return s.applied, nil
}

func (s *originRelayStore) MarkAccountingApplicationReversed(
	context.Context, domain.PendingAccountingApplication, domain.AccountingEvent,
) error {
	return nil
}

func (s *originRelayStore) GetAccountingReversal(
	context.Context, string, string,
) (domain.AccountingReversal, error) {
	return s.reversal, nil
}

func (s *originRelayStore) MarkAccountingReversalCompleted(
	context.Context, domain.AccountingReversal, domain.AccountingEvent,
) error {
	s.reversalDone = true
	return nil
}

func (s *originRelayStore) ListUncertainSales(context.Context, int) ([]domain.PendingFiscal, error) {
	return s.uncertain, nil
}

func (s *originRelayStore) ReserveFiscalConsultAttempt(context.Context, string, string) (int, error) {
	if s.consultAttempt <= 0 {
		return 1, nil
	}
	return s.consultAttempt, nil
}

type originRecordingFiscal struct {
	authorizedRequest domain.FiscalRequest
	authorizedContext capturedDeliveryIdentity
	consultedRequest  domain.FiscalRequest
	consultedContext  capturedDeliveryIdentity
}

func (f *originRecordingFiscal) Authorize(
	ctx context.Context, request domain.FiscalRequest,
) (domain.FiscalResult, error) {
	f.authorizedRequest = request
	f.authorizedContext = captureDeliveryIdentity(ctx)
	return fiscalResultForRequest(request), nil
}

func (f *originRecordingFiscal) Consult(
	ctx context.Context, request domain.FiscalRequest,
) (domain.FiscalResult, error) {
	f.consultedRequest = request
	f.consultedContext = captureDeliveryIdentity(ctx)
	return fiscalResultForRequest(request), nil
}

func fiscalResultForRequest(request domain.FiscalRequest) domain.FiscalResult {
	return domain.FiscalResult{
		RequestID: request.RequestID, OrganizationID: request.OrganizationID,
		IdempotencyKey: request.IdempotencyKey, SourceVersion: request.SourceVersion,
		Status: "authorized", SnapshotDigest: request.SnapshotDigest,
		CorrelationID: request.CorrelationID,
	}
}

type originRecordingAccounting struct {
	posts                   []domain.PostingCommand
	postContexts            []capturedDeliveryIdentity
	applications            []domain.AccountingApplicationCommand
	applicationReversals    []domain.AccountingApplicationReversalCommand
	journalReversals        []domain.ReversalCommand
	applicationContexts     []capturedDeliveryIdentity
	applicationRevContexts  []capturedDeliveryIdentity
	journalReversalContexts []capturedDeliveryIdentity
}

func (a *originRecordingAccounting) Post(
	ctx context.Context, command domain.PostingCommand,
) (domain.AccountingEvent, error) {
	a.posts = append(a.posts, command)
	a.postContexts = append(a.postContexts, captureDeliveryIdentity(ctx))
	result := accountingEventForCommand(
		command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID,
	)
	result.Status = "posted"
	result.JournalEntryID = "journal-entry"
	result.OpenItemIDs = []string{"open-item"}
	return result, nil
}

func (a *originRecordingAccounting) Reverse(
	ctx context.Context, command domain.ReversalCommand,
) (domain.AccountingEvent, error) {
	a.journalReversals = append(a.journalReversals, command)
	a.journalReversalContexts = append(a.journalReversalContexts, captureDeliveryIdentity(ctx))
	result := accountingEventForCommand(
		command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID,
	)
	result.Status = "reversed"
	result.JournalEntryID = "reversal-journal-entry"
	return result, nil
}

func (a *originRecordingAccounting) ApplyOpenItem(
	ctx context.Context, command domain.AccountingApplicationCommand,
) (domain.AccountingEvent, error) {
	a.applications = append(a.applications, command)
	a.applicationContexts = append(a.applicationContexts, captureDeliveryIdentity(ctx))
	result := accountingEventForCommand(
		command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID,
	)
	result.Status = "applied"
	result.ApplicationID = "application-result"
	return result, nil
}

func (a *originRecordingAccounting) ReverseOpenItemApplication(
	ctx context.Context, command domain.AccountingApplicationReversalCommand,
) (domain.AccountingEvent, error) {
	a.applicationReversals = append(a.applicationReversals, command)
	a.applicationRevContexts = append(a.applicationRevContexts, captureDeliveryIdentity(ctx))
	result := accountingEventForCommand(
		command.CommandID, command.OrganizationID, command.IdempotencyKey,
		command.SourceVersion, command.SnapshotDigest, command.CorrelationID,
	)
	result.Status = "reversed"
	return result, nil
}

func accountingEventForCommand(
	commandID, organizationID, idempotencyKey string,
	sourceVersion int,
	snapshotDigest, correlationID string,
) domain.AccountingEvent {
	return domain.AccountingEvent{
		EventID: "accounting-event", CommandID: commandID,
		OrganizationID: organizationID, IdempotencyKey: idempotencyKey,
		SourceVersion: sourceVersion, SnapshotDigest: snapshotDigest,
		CorrelationID: correlationID,
	}
}

func TestDurableWorkerPropagatesOutboxIdentityAndPersistedPaymentMetadata(t *testing.T) {
	const (
		organizationID = "org-origin"
		requestID      = "request-from-outbox"
		correlationID  = "correlation-from-outbox"
		actorRef       = "user_from_outbox"
	)
	payment := domain.Payment{
		ID: "payment-origin", OrganizationID: organizationID, Direction: "receipt",
		PartyRef: "party-origin", Total: domain.Money{Amount: "25.00", Currency: "ARS"},
		Status: "confirmed", SnapshotDigest: strings.Repeat("b", 64),
		Origin: domain.OriginMetadata{
			RequestID: "aggregate-request", CorrelationID: "aggregate-correlation",
			ActorRef: "aggregate-actor", SourceVersion: 7,
		},
		CorrelationID: "legacy-correlation",
		CreatedAt:     time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC),
	}
	event := domain.Event{
		ID: "event-origin", OrganizationID: organizationID,
		Topic: "PaymentPostingRequested", Payload: []byte(`{"payment_id":"payment-origin"}`),
		RequestID: requestID, CorrelationID: correlationID, ActorRef: actorRef,
	}
	store := &originRelayStore{events: []domain.Event{event}, payment: payment}
	accounting := &originRecordingAccounting{}
	worker := DurableWorker{
		Store: store, Fiscal: &originRecordingFiscal{}, Accounting: accounting,
	}

	tainted := identityusecases.WithRequestMetadata(
		context.Background(),
		identityusecases.RequestMetadata{RequestID: "untrusted", CorrelationID: "untrusted"},
	)
	tainted = identityusecases.WithPrincipal(tainted, identity.Principal{
		OrganizationID: organizationID, ActorID: "untrusted",
		Role: identity.RoleOwner, MembershipStatus: "active",
	})
	tainted = identityusecases.WithDelegatedActor(tainted, "untrusted")
	if err := worker.DispatchOnce(tainted); err != nil {
		t.Fatal(err)
	}
	if len(accounting.posts) != 1 || len(accounting.postContexts) != 1 {
		t.Fatalf("posts=%d contexts=%d", len(accounting.posts), len(accounting.postContexts))
	}
	command := accounting.posts[0]
	if command.SourceVersion != 7 ||
		command.CommandID != accountingCommandID("payment", payment.ID, 7) ||
		command.IdempotencyKey != internalIdempotencyKey(organizationID, "accounting.post", payment.ID, 7) ||
		command.SnapshotDigest != payment.SnapshotDigest ||
		command.CorrelationID != payment.Origin.CorrelationID {
		t.Fatalf("posting command did not preserve origin metadata: %+v", command)
	}
	assertDeliveryIdentity(
		t, accounting.postContexts[0],
		organizationID, actorRef, requestID, correlationID, true,
	)
	if store.published != 1 || store.retried {
		t.Fatalf("published=%d retried=%v", store.published, store.retried)
	}
}

func TestDurableWorkerFallsBackToEventIDAndClearsInheritedActor(t *testing.T) {
	payment := domain.Payment{
		ID: "payment-fallback", OrganizationID: "org-fallback", Direction: "receipt",
		PartyRef: "party", Total: domain.Money{Amount: "1.00", Currency: "ARS"},
		Status: "confirmed", SnapshotDigest: strings.Repeat("c", 64),
		CreatedAt: time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC),
	}
	event := domain.Event{
		ID: "event-fallback", OrganizationID: payment.OrganizationID,
		Topic: "PaymentPostingRequested", Payload: []byte(`{"payment_id":"payment-fallback"}`),
	}
	store := &originRelayStore{events: []domain.Event{event}, payment: payment}
	accounting := &originRecordingAccounting{}
	worker := DurableWorker{
		Store: store, Fiscal: &originRecordingFiscal{}, Accounting: accounting,
	}
	tainted := identityusecases.WithPrincipal(context.Background(), identity.Principal{
		OrganizationID: payment.OrganizationID, ActorID: "caller-actor",
		Role: identity.RoleOwner, MembershipStatus: "active",
	})
	tainted = identityusecases.WithDelegatedActor(tainted, "caller-actor")
	if err := worker.DispatchOnce(tainted); err != nil {
		t.Fatal(err)
	}
	if len(accounting.postContexts) != 1 {
		t.Fatalf("contexts=%d", len(accounting.postContexts))
	}
	assertDeliveryIdentity(
		t, accounting.postContexts[0],
		payment.OrganizationID, "", event.ID, event.ID, false,
	)
}

func TestDurableWorkerRejectsCrossTenantAggregateBeforeDelivery(t *testing.T) {
	event := domain.Event{
		ID: "event-org-a", OrganizationID: "org-a", Topic: "PaymentPostingRequested",
		Payload:   []byte(`{"payment_id":"payment-org-b"}`),
		RequestID: "request-a", CorrelationID: "correlation-a", ActorRef: "actor-a",
	}
	store := &originRelayStore{
		events: []domain.Event{event},
		payment: domain.Payment{
			ID: "payment-org-b", OrganizationID: "org-b", Direction: "receipt",
			PartyRef: "party-b", Total: domain.Money{Amount: "1.00", Currency: "ARS"},
			Status: "confirmed", SnapshotDigest: strings.Repeat("d", 64),
		},
	}
	accounting := &originRecordingAccounting{}
	worker := DurableWorker{
		Store: store, Fiscal: &originRecordingFiscal{}, Accounting: accounting,
	}
	if err := worker.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !store.retried || store.published != 0 || len(accounting.posts) != 0 {
		t.Fatalf(
			"cross-tenant aggregate reached delivery: retried=%v published=%d posts=%d",
			store.retried, store.published, len(accounting.posts),
		)
	}
}

func TestPostingBuildersUsePersistedOriginMetadata(t *testing.T) {
	t.Run("sale", func(t *testing.T) {
		sale := postingTestSale("FA", "ARS", "121", json.RawMessage(`{
			"issue_date":"2026-07-31",
			"currency":"ARS",
			"totals":{"net":"100","vat":"21","exempt":"0","total":"121"}
		}`))
		sale.Origin = domain.OriginMetadata{SourceVersion: 9, CorrelationID: "sale-origin"}
		sale.CorrelationID = "sale-legacy"
		command, err := buildSalePostingCommand(sale, nil)
		if err != nil {
			t.Fatal(err)
		}
		if command.SourceVersion != 9 ||
			command.CommandID != accountingCommandID("posting", sale.ID, 9) ||
			command.IdempotencyKey != internalIdempotencyKey(sale.OrganizationID, "accounting.post", sale.ID, 9) ||
			command.CorrelationID != sale.Origin.CorrelationID {
			t.Fatalf("sale command=%+v", command)
		}
	})

	t.Run("purchase", func(t *testing.T) {
		purchase := postingTestPurchase()
		purchase.Origin = domain.OriginMetadata{SourceVersion: 11, CorrelationID: "purchase-origin"}
		purchase.CorrelationID = "purchase-legacy"
		command, err := buildPurchasePostingCommand(purchase)
		if err != nil {
			t.Fatal(err)
		}
		if command.SourceVersion != 11 ||
			command.CommandID != accountingCommandID("purchase", purchase.ID, 11) ||
			command.IdempotencyKey != internalIdempotencyKey(purchase.OrganizationID, "accounting.post", purchase.ID, 11) ||
			command.CorrelationID != purchase.Origin.CorrelationID {
			t.Fatalf("purchase command=%+v", command)
		}
	})
}

func TestWorkerFiscalApplicationAndReversalUsePersistedOriginMetadata(t *testing.T) {
	t.Run("fiscal authorization", func(t *testing.T) {
		sale := postingTestSale("FA", "ARS", "121", json.RawMessage(`{}`))
		sale.Status = domain.SaleFiscalPending
		sale.Origin = domain.OriginMetadata{SourceVersion: 5, CorrelationID: "fiscal-origin"}
		sale.CorrelationID = "fiscal-legacy"
		store := &originRelayStore{sale: sale}
		fiscal := &originRecordingFiscal{}
		worker := DurableWorker{Store: store, Fiscal: fiscal, Accounting: &originRecordingAccounting{}}
		event := domain.Event{
			ID: "event-fiscal", OrganizationID: sale.OrganizationID,
			Topic:   "FiscalAuthorizationRequested",
			Payload: []byte(`{"sale_id":"sale_test","credential_ref":"credential"}`),
		}
		if err := worker.handle(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		request := fiscal.authorizedRequest
		if request.SourceVersion != 5 ||
			request.RequestID != "fiscal-authorize:sale_test:5" ||
			request.IdempotencyKey != internalIdempotencyKey(sale.OrganizationID, "fiscal.authorize", sale.ID, 5) ||
			request.CorrelationID != sale.Origin.CorrelationID ||
			!store.fiscalApplied {
			t.Fatalf("fiscal request=%+v applied=%v", request, store.fiscalApplied)
		}
	})

	t.Run("open item application", func(t *testing.T) {
		application := domain.PendingAccountingApplication{
			ID: "application-command", OrganizationID: "org-application",
			DebitOpenItemID: "debit", CreditOpenItemID: "credit",
			Amount: domain.Money{Amount: "10.00", Currency: "ARS"}, Status: "pending",
			SnapshotDigest: strings.Repeat("e", 64), CorrelationID: "application-legacy",
			Origin: domain.OriginMetadata{SourceVersion: 6, CorrelationID: "application-origin"},
		}
		store := &originRelayStore{application: application}
		accounting := &originRecordingAccounting{}
		worker := DurableWorker{Store: store, Fiscal: &originRecordingFiscal{}, Accounting: accounting}
		event := domain.Event{
			ID: "event-application", OrganizationID: application.OrganizationID,
			Topic:       "OpenItemApplicationRequested",
			Payload:     []byte(`{"application_id":"application-command"}`),
			AvailableAt: time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC),
		}
		if err := worker.handle(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if len(accounting.applications) != 1 {
			t.Fatalf("applications=%d", len(accounting.applications))
		}
		command := accounting.applications[0]
		if command.SourceVersion != 6 ||
			command.IdempotencyKey != internalIdempotencyKey(application.OrganizationID, "accounting.apply", application.ID, 6) ||
			command.SnapshotDigest != application.SnapshotDigest ||
			command.CorrelationID != application.Origin.CorrelationID ||
			!store.applicationDone {
			t.Fatalf("application command=%+v done=%v", command, store.applicationDone)
		}
	})

	t.Run("journal and application reversals", func(t *testing.T) {
		reversal := domain.AccountingReversal{
			ID: "reversal-command", OrganizationID: "org-reversal",
			DocumentKind: "sale", DocumentID: "sale-reversed",
			OriginalJournalEntryID: "original-journal",
			EffectiveAt:            time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC),
			Reason:                 "void", Status: "requested",
			SnapshotDigest: strings.Repeat("f", 64), CorrelationID: "reversal-legacy",
			Origin: domain.OriginMetadata{SourceVersion: 8, CorrelationID: "reversal-origin"},
		}
		application := domain.PendingAccountingApplication{
			ID: "applied-command", OrganizationID: reversal.OrganizationID,
			ApplicationID: "accounting-application", Status: "applied",
		}
		store := &originRelayStore{reversal: reversal, applied: []domain.PendingAccountingApplication{application}}
		accounting := &originRecordingAccounting{}
		worker := DurableWorker{Store: store, Fiscal: &originRecordingFiscal{}, Accounting: accounting}
		event := domain.Event{
			ID: "event-reversal", OrganizationID: reversal.OrganizationID,
			Topic:   "AccountingReversalRequested",
			Payload: []byte(`{"reversal_id":"reversal-command"}`),
		}
		if err := worker.handle(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if len(accounting.applicationReversals) != 1 || len(accounting.journalReversals) != 1 {
			t.Fatalf(
				"application reversals=%d journal reversals=%d",
				len(accounting.applicationReversals), len(accounting.journalReversals),
			)
		}
		applicationCommand := accounting.applicationReversals[0]
		if applicationCommand.SourceVersion != 8 ||
			applicationCommand.CommandID != accountingCommandID("application-reversal", application.ID, 8) ||
			applicationCommand.IdempotencyKey != internalIdempotencyKey(
				reversal.OrganizationID, "accounting.reverse-application", application.ID, 8,
			) ||
			applicationCommand.CorrelationID != reversal.Origin.CorrelationID {
			t.Fatalf("application reversal=%+v", applicationCommand)
		}
		journalCommand := accounting.journalReversals[0]
		if journalCommand.SourceVersion != 8 ||
			journalCommand.CommandID != accountingCommandID("journal-reversal", reversal.ID, 8) ||
			journalCommand.IdempotencyKey != internalIdempotencyKey(
				reversal.OrganizationID, "accounting.reverse", reversal.ID, 8,
			) ||
			journalCommand.SnapshotDigest != reversal.SnapshotDigest ||
			journalCommand.CorrelationID != reversal.Origin.CorrelationID ||
			!store.reversalDone {
			t.Fatalf("journal reversal=%+v done=%v", journalCommand, store.reversalDone)
		}
	})
}

func TestReconcileFiscalPropagatesPersistedActorAndSourceMetadata(t *testing.T) {
	sale := postingTestSale("FA", "ARS", "121", json.RawMessage(`{}`))
	sale.Status = domain.SaleFiscalUncertain
	sale.Origin = domain.OriginMetadata{
		RequestID: "original-request", CorrelationID: "reconcile-correlation",
		ActorRef: "reconcile-actor", SourceVersion: 12,
	}
	sale.CorrelationID = "legacy-correlation"
	store := &originRelayStore{
		uncertain:      []domain.PendingFiscal{{Sale: sale, CredentialRef: "credential"}},
		consultAttempt: 3,
	}
	fiscal := &originRecordingFiscal{}
	tainted := identityusecases.WithPrincipal(context.Background(), identity.Principal{
		OrganizationID: sale.OrganizationID, ActorID: "caller-actor",
		Role: identity.RoleOwner, MembershipStatus: "active",
	})
	tainted = identityusecases.WithDelegatedActor(tainted, "caller-actor")
	if err := (ReconcileFiscal{Store: store, Fiscal: fiscal}).Execute(tainted); err != nil {
		t.Fatal(err)
	}
	request := fiscal.consultedRequest
	if request.SourceVersion != 12 ||
		request.RequestID != "fiscal-consult:sale_test:3" ||
		request.IdempotencyKey != internalIdempotencyKey(
			sale.OrganizationID, "fiscal.consult", "sale_test:3", 12,
		) ||
		request.CorrelationID != sale.Origin.CorrelationID ||
		!store.fiscalApplied {
		t.Fatalf("consult request=%+v applied=%v", request, store.fiscalApplied)
	}
	assertDeliveryIdentity(
		t, fiscal.consultedContext,
		sale.OrganizationID, sale.Origin.ActorRef,
		request.RequestID, sale.Origin.CorrelationID, true,
	)
}

func TestPersistedSourceVersionFallsBackToOne(t *testing.T) {
	for _, value := range []int{-5, 0} {
		if got := persistedSourceVersion(domain.OriginMetadata{SourceVersion: value}); got != 1 {
			t.Fatalf("source version fallback for %d = %d", value, got)
		}
	}
	if got := persistedSourceVersion(domain.OriginMetadata{SourceVersion: 4}); got != 4 {
		t.Fatalf("source version = %d", got)
	}
}

func TestPaymentWithoutPersistedSnapshotFailsBeforeAccounting(t *testing.T) {
	payment := domain.Payment{
		ID: "payment-no-snapshot", OrganizationID: "org-no-snapshot",
		Direction: "receipt", PartyRef: "party",
		Total: domain.Money{Amount: "1.00", Currency: "ARS"}, Status: "confirmed",
	}
	store := &originRelayStore{payment: payment}
	accounting := &originRecordingAccounting{}
	worker := DurableWorker{Store: store, Fiscal: &originRecordingFiscal{}, Accounting: accounting}
	event := domain.Event{
		ID: "event-no-snapshot", OrganizationID: payment.OrganizationID,
		Topic:   "PaymentPostingRequested",
		Payload: []byte(`{"payment_id":"payment-no-snapshot"}`),
	}
	if err := worker.handle(context.Background(), event); err == nil {
		t.Fatal("expected missing persisted payment snapshot to fail")
	}
	if len(accounting.posts) != 0 {
		t.Fatalf("accounting posts=%d", len(accounting.posts))
	}
}

func assertDeliveryIdentity(
	t *testing.T,
	got capturedDeliveryIdentity,
	organizationID, actorRef, requestID, correlationID string,
	expectAuthorized bool,
) {
	t.Helper()
	if !got.hasMetadata ||
		got.metadata.RequestID != requestID ||
		got.metadata.CorrelationID != correlationID {
		t.Fatalf("request metadata=%+v present=%v", got.metadata, got.hasMetadata)
	}
	if !got.hasPrincipal ||
		got.principal.OrganizationID != organizationID ||
		got.principal.ActorID != actorRef ||
		got.principal.Role != identity.RoleMember ||
		got.principal.MembershipStatus != "active" ||
		got.principal.OrganizationStatus != "ready" {
		t.Fatalf("principal=%+v present=%v", got.principal, got.hasPrincipal)
	}
	if got.principal.CanRead(organizationID) != expectAuthorized {
		t.Fatalf(
			"principal authorization for %q=%v, want %v",
			organizationID, got.principal.CanRead(organizationID), expectAuthorized,
		)
	}
	if got.hasDelegated != (actorRef != "") || got.delegated != actorRef {
		t.Fatalf("delegated actor=%q present=%v", got.delegated, got.hasDelegated)
	}
}
