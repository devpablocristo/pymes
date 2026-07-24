package fiscal

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryFiscalRepository struct {
	mu            sync.Mutex
	byKey         map[string]QueueCommand
	bySource      map[string]Voucher
	voucher       Voucher
	finalizations int
	uncertain     int
}

func newMemoryFiscalRepository() *memoryFiscalRepository {
	return &memoryFiscalRepository{
		byKey: make(map[string]QueueCommand), bySource: make(map[string]Voucher),
	}
}

func (repository *memoryFiscalRepository) Enqueue(_ context.Context, command QueueCommand) (QueueResult, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	key := command.OrganizationID.String() + ":" + command.IdempotencyKey
	if previous, found := repository.byKey[key]; found {
		if previous.Fingerprint != command.Fingerprint {
			return QueueResult{}, ErrIdempotencyConflict
		}
		sourceKey := command.OrganizationID.String() + ":" + command.Source.ID.String() + ":" + string(command.Operation)
		return QueueResult{Voucher: repository.bySource[sourceKey], Replay: true}, nil
	}
	sourceKey := command.OrganizationID.String() + ":" + command.Source.ID.String() + ":" + string(command.Operation)
	if _, found := repository.bySource[sourceKey]; found {
		return QueueResult{}, ErrSourceAlreadyUsed
	}
	voucher := Voucher{
		ID: uuid.New(), OrganizationID: command.OrganizationID, Source: command.Source,
		Operation: command.Operation, PointOfSale: command.PointOfSale,
		AuthorityType: command.AuthorityType, Status: StatusQueued,
		Snapshot: command.Snapshot, CreatedAt: command.RequestedAt, UpdatedAt: command.RequestedAt,
	}
	repository.byKey[key] = command
	repository.bySource[sourceKey] = voucher
	repository.voucher = voucher
	return QueueResult{Voucher: voucher}, nil
}

func (repository *memoryFiscalRepository) Get(context.Context, uuid.UUID, uuid.UUID) (Voucher, error) {
	return repository.voucher, nil
}
func (repository *memoryFiscalRepository) LeaseNext(context.Context, string, time.Time, time.Time) (Lease, error) {
	return Lease{}, ErrNoWork
}
func (repository *memoryFiscalRepository) RenewLease(context.Context, uuid.UUID, string, time.Time) error {
	return nil
}
func (repository *memoryFiscalRepository) ReleaseLease(context.Context, uuid.UUID, string, time.Time, string) error {
	return nil
}
func (repository *memoryFiscalRepository) AssignNumber(
	_ context.Context, _ uuid.UUID, _ string, number int64, at time.Time,
) (Voucher, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.voucher.Number = number
	repository.voucher.UpdatedAt = at
	return repository.voucher, nil
}
func (repository *memoryFiscalRepository) MarkProcessing(
	_ context.Context, _ uuid.UUID, _ string, at time.Time,
) (Voucher, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.voucher.Status = StatusProcessing
	repository.voucher.UpdatedAt = at
	return repository.voucher, nil
}
func (repository *memoryFiscalRepository) MarkUncertain(context.Context, uuid.UUID, string, Failure) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.uncertain++
	repository.voucher.Status = StatusUncertain
	return nil
}
func (repository *memoryFiscalRepository) MarkRejected(context.Context, uuid.UUID, string, Authorization) error {
	return nil
}
func (repository *memoryFiscalRepository) FinalizeAuthorized(_ context.Context, finalization Finalization) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.finalizations++
	repository.voucher.Status = StatusAuthorized
	repository.voucher.Authorization = &finalization.Authorization
	return nil
}

type recoveryAuthority struct {
	authorizeCalls atomic.Int32
	consult        AuthorityLookup
	consultErr     error
}

func (authority *recoveryAuthority) LastAuthorized(context.Context, Voucher) (int64, error) {
	return 41, nil
}
func (authority *recoveryAuthority) Authorize(context.Context, Voucher) (AuthorityResult, error) {
	authority.authorizeCalls.Add(1)
	return AuthorityResult{}, errors.New("should not authorize during successful recovery")
}
func (authority *recoveryAuthority) Consult(context.Context, Voucher) (AuthorityLookup, error) {
	return authority.consult, authority.consultErr
}

type heartbeatQueue struct {
	mu       sync.Mutex
	lease    Lease
	leased   bool
	renewed  chan struct{}
	renewErr error
}

func (queue *heartbeatQueue) LeaseNext(
	context.Context,
	string,
	time.Time,
	time.Time,
) (Lease, error) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if queue.leased {
		return Lease{}, ErrNoWork
	}
	queue.leased = true
	return queue.lease, nil
}

func (queue *heartbeatQueue) RenewLease(
	context.Context,
	uuid.UUID,
	string,
	time.Time,
) error {
	select {
	case queue.renewed <- struct{}{}:
	default:
	}
	return queue.renewErr
}

func (*heartbeatQueue) ReleaseLease(
	context.Context,
	uuid.UUID,
	string,
	time.Time,
	string,
) error {
	return nil
}

type blockingAuthority struct {
	started chan struct{}
	release chan struct{}
}

func (*blockingAuthority) LastAuthorized(context.Context, Voucher) (int64, error) {
	return 41, nil
}

func (authority *blockingAuthority) Authorize(
	ctx context.Context,
	_ Voucher,
) (AuthorityResult, error) {
	close(authority.started)
	select {
	case <-ctx.Done():
		return AuthorityResult{}, ctx.Err()
	case <-authority.release:
		return AuthorityResult{
			Decision:    DecisionAuthorized,
			Code:        "74212345678901",
			ExpiresOn:   "2026-08-03",
			Number:      42,
			ProcessedAt: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		}, nil
	}
}

func (*blockingAuthority) Consult(context.Context, Voucher) (AuthorityLookup, error) {
	return AuthorityLookup{}, nil
}

func TestConcurrentQueueProducesOneVoucher(t *testing.T) {
	t.Parallel()

	repository := newMemoryFiscalRepository()
	service := NewService(repository)
	service.now = func() time.Time { return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC) }
	organizationID := uuid.New()
	sourceID := uuid.New()
	input := QueueVoucherInput{
		OrganizationID: organizationID,
		IdempotencyKey: "same-command-key-1234",
		Source:         SourceReference{Kind: "sale", ID: sourceID},
		Operation:      OperationInvoice, Environment: "homologation",
		PointOfSale: 3, AuthorityType: 1,
		Snapshot: snapshotFixture(t), Actor: "accountant",
	}

	const requests = 16
	results := make(chan QueueResult, requests)
	errs := make(chan error, requests)
	var wait sync.WaitGroup
	for range requests {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := service.Queue(context.Background(), input)
			results <- result
			errs <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var voucherID uuid.UUID
	for result := range results {
		if voucherID == uuid.Nil {
			voucherID = result.Voucher.ID
		}
		if result.Voucher.ID != voucherID {
			t.Fatalf("concurrent request created voucher %s, want %s", result.Voucher.ID, voucherID)
		}
	}
	if got, want := len(repository.bySource), 1; got != want {
		t.Fatalf("stored vouchers = %d, want %d", got, want)
	}
}

func TestProcessorRecoversExactUncertainNumberWithoutReauthorizing(t *testing.T) {
	t.Parallel()

	repository := newMemoryFiscalRepository()
	repository.voucher = Voucher{
		ID: uuid.New(), OrganizationID: uuid.New(),
		Source:    SourceReference{Kind: "sale", ID: uuid.New()},
		Operation: OperationInvoice, PointOfSale: 3, AuthorityType: 1,
		Number: 42, Status: StatusUncertain, Snapshot: snapshotFixture(t),
	}
	authority := &recoveryAuthority{consult: AuthorityLookup{
		Found: true,
		Result: AuthorityResult{
			Decision: DecisionAuthorized, Code: "74212345678901", Number: 42,
			ProcessedAt: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		},
	}}
	processor := NewProcessor(repository, authority, nil, nil)
	if err := processor.Process(context.Background(), Lease{
		Voucher: repository.voucher, Token: "lease-token",
	}); err != nil {
		t.Fatal(err)
	}
	if got := authority.authorizeCalls.Load(); got != 0 {
		t.Fatalf("authorize calls = %d, want 0", got)
	}
	if repository.finalizations != 1 || repository.voucher.Status != StatusAuthorized {
		t.Fatalf("recovery did not finalize exactly once: %#v", repository.voucher)
	}
}

func TestWorkerRenewsLeaseWhileAuthorityCallIsInFlight(t *testing.T) {
	t.Parallel()

	repository := newMemoryFiscalRepository()
	repository.voucher = Voucher{
		ID: uuid.New(), OrganizationID: uuid.New(),
		Source:    SourceReference{Kind: "sale", ID: uuid.New()},
		Operation: OperationInvoice, Environment: "homologation",
		PointOfSale: 3, AuthorityType: 1,
		Status: StatusProcessing, Snapshot: snapshotFixture(t),
	}
	authority := &blockingAuthority{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	queue := &heartbeatQueue{
		lease:   Lease{Voucher: repository.voucher, Token: "worker:lease"},
		renewed: make(chan struct{}, 1),
	}
	worker := NewWorker(
		queue,
		NewProcessor(repository, authority, nil, nil),
		"worker",
		30*time.Millisecond,
	)
	result := make(chan error, 1)
	go func() {
		_, err := worker.RunOnce(context.Background())
		result <- err
	}()

	select {
	case <-authority.started:
	case <-time.After(time.Second):
		t.Fatal("authority call did not start")
	}
	select {
	case <-queue.renewed:
	case <-time.After(time.Second):
		t.Fatal("worker did not renew the lease during the authority call")
	}
	close(authority.release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if repository.finalizations != 1 {
		t.Fatalf("finalizations = %d, want 1", repository.finalizations)
	}
}

func TestWorkerCancelsAuthorityCallWhenLeaseRenewalFails(t *testing.T) {
	t.Parallel()

	repository := newMemoryFiscalRepository()
	repository.voucher = Voucher{
		ID: uuid.New(), OrganizationID: uuid.New(),
		Source:    SourceReference{Kind: "sale", ID: uuid.New()},
		Operation: OperationInvoice, Environment: "homologation",
		PointOfSale: 3, AuthorityType: 1,
		Status: StatusProcessing, Snapshot: snapshotFixture(t),
	}
	authority := &blockingAuthority{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	queue := &heartbeatQueue{
		lease:   Lease{Voucher: repository.voucher, Token: "worker:lease"},
		renewed: make(chan struct{}, 1), renewErr: ErrLeaseLost,
	}
	worker := NewWorker(
		queue,
		NewProcessor(repository, authority, nil, nil),
		"worker",
		30*time.Millisecond,
	)
	_, err := worker.RunOnce(context.Background())
	if !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("RunOnce() error = %v, want ErrLeaseLost", err)
	}
	if repository.finalizations != 0 {
		t.Fatalf("finalizations = %d, want 0", repository.finalizations)
	}
}
