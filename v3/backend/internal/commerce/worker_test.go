package commerce

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	identityusecases "github.com/devpablocristo/pymes/v3/backend/internal/identity"
)

type deadLetterStore struct {
	RelayStore
	event        domain.Event
	retried      bool
	deadLettered bool
	published    bool
	failureCode  string
	sale         domain.Sale
	failure      domain.AccountingFailure
}

type topicLeaseStore struct {
	deadLetterStore
	topics []string
}

func (store *topicLeaseStore) LeaseTopics(
	_ context.Context,
	topics []string,
	_ int,
	_ time.Duration,
) ([]domain.Event, error) {
	store.topics = append([]string(nil), topics...)
	return nil, nil
}

func TestDurableWorkerLeasesOnlyOwnedContextTopics(t *testing.T) {
	store := &topicLeaseStore{}
	worker := DurableWorker{
		Store:      store,
		Fiscal:     NewFakeFiscal(),
		Accounting: NewFakeAccounting(),
	}
	if err := worker.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"FiscalAuthorizationRequested": true,
		"AccountingPostingRequested":   true,
	}
	for _, topic := range store.topics {
		if topic == "NotificationRequested" ||
			topic == "CalendarSyncRequested" {
			t.Fatalf("foreign topic leased by commerce: %q", topic)
		}
		delete(want, topic)
	}
	if len(want) != 0 {
		t.Fatalf("missing owned lease topics: %v; got=%v", want, store.topics)
	}
}

func (s *deadLetterStore) LeaseTopics(
	context.Context,
	[]string,
	int,
	time.Duration,
) ([]domain.Event, error) {
	return []domain.Event{s.event}, nil
}

func (s *deadLetterStore) Retry(context.Context, domain.Event) error {
	s.retried = true
	return nil
}

func (s *deadLetterStore) DeadLetter(_ context.Context, event domain.Event, failureCode string) error {
	if event.ID != s.event.ID {
		return domain.ErrLeaseLost
	}
	s.deadLettered = true
	s.failureCode = failureCode
	return nil
}

func (s *deadLetterStore) MarkPublished(_ context.Context, event domain.Event) error {
	if event.ID != s.event.ID {
		return domain.ErrLeaseLost
	}
	s.published = true
	return nil
}

func (s *deadLetterStore) RecordAccountingPeriodLocked(
	_ context.Context,
	event domain.Event,
	failure domain.AccountingFailure,
) error {
	if event.ID != s.event.ID {
		return domain.ErrLeaseLost
	}
	s.failure = failure
	return nil
}

func (s *deadLetterStore) ListUncertainSales(context.Context, int) ([]domain.PendingFiscal, error) {
	return nil, nil
}

func (s *deadLetterStore) GetSale(context.Context, string, string) (domain.Sale, error) {
	return s.sale, nil
}

type periodLockedAccounting struct {
	metadata *identityusecases.RequestMetadata
}

func (a periodLockedAccounting) Post(ctx context.Context, _ domain.PostingCommand) (domain.AccountingEvent, error) {
	if a.metadata != nil {
		*a.metadata, _ = identityusecases.RequestMetadataFromContext(ctx)
	}
	return domain.AccountingEvent{}, domain.ErrPeriodLocked
}
func (periodLockedAccounting) Reverse(context.Context, domain.ReversalCommand) (domain.AccountingEvent, error) {
	return domain.AccountingEvent{}, nil
}
func (periodLockedAccounting) ApplyOpenItem(context.Context, domain.AccountingApplicationCommand) (domain.AccountingEvent, error) {
	return domain.AccountingEvent{}, nil
}
func (periodLockedAccounting) ReverseOpenItemApplication(context.Context, domain.AccountingApplicationReversalCommand) (domain.AccountingEvent, error) {
	return domain.AccountingEvent{}, nil
}

func TestDurableWorkerDeadLettersExhaustedEvent(t *testing.T) {
	store := &deadLetterStore{event: domain.Event{
		ID:             "event-exhausted",
		OrganizationID: "org-a",
		Topic:          "FiscalAuthorizationRequested",
		Payload:        []byte("{"),
		Attempts:       3,
		LeaseToken:     "lease",
	}}
	worker := DurableWorker{
		Store:       store,
		Fiscal:      NewFakeFiscal(),
		Accounting:  NewFakeAccounting(),
		LeaseFor:    time.Second,
		MaxAttempts: 3,
	}
	if err := worker.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !store.deadLettered || store.retried || store.failureCode != "DELIVERY_FAILED" {
		t.Fatalf("dead_lettered=%v retried=%v failure_code=%q", store.deadLettered, store.retried, store.failureCode)
	}
}

func TestDurableWorkerDoesNotBlindlyRetryLockedPeriod(t *testing.T) {
	sale := postingTestSale("FA", "ARS", "121", json.RawMessage(`{
		"issue_date":"2026-07-31",
		"currency":"ARS",
		"totals":{"net":"100","vat":"21","exempt":"0","total":"121"}
	}`))
	sale.Status = domain.SaleAuthorizedPendingPosting
	var metadata identityusecases.RequestMetadata
	store := &deadLetterStore{
		event: domain.Event{
			ID:             "event-period-locked",
			OrganizationID: sale.OrganizationID,
			Topic:          "AccountingPostingRequested",
			Payload:        []byte(`{"sale_id":"sale_test"}`),
			RequestID:      "request-period-locked",
			ActorRef:       "user-period-locked",
			SourceVersion:  1,
			SnapshotDigest: sale.SnapshotDigest,
			CorrelationID:  "sale-correlation",
			Attempts:       1,
			LeaseToken:     "lease",
		},
		sale: sale,
	}
	worker := DurableWorker{
		Store:       store,
		Fiscal:      NewFakeFiscal(),
		Accounting:  periodLockedAccounting{metadata: &metadata},
		LeaseFor:    time.Second,
		MaxAttempts: 10,
	}
	if err := worker.DispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.deadLettered || store.retried || !store.published {
		t.Fatalf("dead_lettered=%v retried=%v published=%v", store.deadLettered, store.retried, store.published)
	}
	if store.failure.FailureCode != domain.ErrPeriodLocked.Error() ||
		store.failure.SourceKind != "sale" ||
		store.failure.SourceID != sale.ID ||
		store.failure.CommandKind != "posting" ||
		len(store.failure.CommandPayload) == 0 ||
		store.failure.Origin.RequestID != store.event.RequestID ||
		store.failure.Origin.ActorRef != store.event.ActorRef {
		t.Fatalf("failure=%+v", store.failure)
	}
	if metadata.RequestID != store.event.RequestID ||
		metadata.CorrelationID != store.event.CorrelationID {
		t.Fatalf("delivery metadata = %#v", metadata)
	}
}
