package fiscal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

type QueueVoucherInput struct {
	OrganizationID uuid.UUID
	IdempotencyKey string
	Source         SourceReference
	Operation      Operation
	Environment    string
	PointOfSale    int
	AuthorityType  int
	Snapshot       Snapshot
	Actor          string
}

type Service struct {
	repository CommandRepository
	now        func() time.Time
}

func NewService(repository CommandRepository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (service *Service) Queue(ctx context.Context, input QueueVoucherInput) (QueueResult, error) {
	if service == nil || service.repository == nil {
		return QueueResult{}, errors.New("fiscal command repository is required")
	}
	if input.OrganizationID == uuid.Nil {
		return QueueResult{}, errors.New("organization context is required")
	}
	key := strings.TrimSpace(input.IdempotencyKey)
	if len(key) < 16 || len(key) > 128 {
		return QueueResult{}, errors.New("idempotency key must contain between 16 and 128 characters")
	}
	if err := input.Source.Validate(); err != nil {
		return QueueResult{}, err
	}
	if !input.Operation.Valid() {
		return QueueResult{}, fmt.Errorf("invalid fiscal operation %q", input.Operation)
	}
	input.Environment = strings.ToLower(strings.TrimSpace(input.Environment))
	if input.Environment != "homologation" && input.Environment != "production" {
		return QueueResult{}, errors.New("fiscal environment must be homologation or production")
	}
	if input.PointOfSale <= 0 || input.PointOfSale > 99999 {
		return QueueResult{}, errors.New("point of sale must be between 1 and 99999")
	}
	if input.AuthorityType <= 0 {
		return QueueResult{}, errors.New("authority voucher type is required")
	}
	if len(input.Snapshot.canonical) == 0 {
		return QueueResult{}, errors.New("fiscal snapshot is required")
	}

	fingerprint, err := queueFingerprint(input)
	if err != nil {
		return QueueResult{}, err
	}
	now := service.now().UTC()
	return service.repository.Enqueue(ctx, QueueCommand{
		OrganizationID: input.OrganizationID,
		IdempotencyKey: key,
		Fingerprint:    fingerprint,
		Source:         input.Source,
		Operation:      input.Operation,
		Environment:    input.Environment,
		PointOfSale:    input.PointOfSale,
		AuthorityType:  input.AuthorityType,
		Snapshot:       input.Snapshot,
		Actor:          strings.TrimSpace(input.Actor),
		RequestedAt:    now,
	})
}

func queueFingerprint(input QueueVoucherInput) (string, error) {
	raw, err := json.Marshal(struct {
		Source        SourceReference `json:"source"`
		Operation     Operation       `json:"operation"`
		Environment   string          `json:"environment"`
		PointOfSale   int             `json:"point_of_sale"`
		AuthorityType int             `json:"authority_type"`
		SnapshotHash  string          `json:"snapshot_hash"`
	}{
		Source:        input.Source,
		Operation:     input.Operation,
		Environment:   input.Environment,
		PointOfSale:   input.PointOfSale,
		AuthorityType: input.AuthorityType,
		SnapshotHash:  input.Snapshot.Hash(),
	})
	if err != nil {
		return "", fmt.Errorf("build fiscal command fingerprint: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

type Processor struct {
	repository VoucherRepository
	authority  Authority
	renderer   ArtifactRenderer
	objects    ObjectStore
	now        func() time.Time
}

func NewProcessor(
	repository VoucherRepository,
	authority Authority,
	renderer ArtifactRenderer,
	objects ObjectStore,
) *Processor {
	return &Processor{
		repository: repository,
		authority:  authority,
		renderer:   renderer,
		objects:    objects,
		now:        time.Now,
	}
}

// Process handles a previously leased voucher. A persisted number is always
// reconciled with Consult before another authorization call. It is never
// replaced with a later number after an ambiguous response.
func (processor *Processor) Process(ctx context.Context, lease Lease) error {
	if processor == nil || processor.repository == nil || processor.authority == nil {
		return errors.New("fiscal processor dependencies are incomplete")
	}
	if lease.Voucher.ID == uuid.Nil || strings.TrimSpace(lease.Token) == "" {
		return errors.New("valid fiscal lease is required")
	}
	// Cross-process serialization is a durable queue invariant: only one
	// processing/uncertain voucher may own a series. Do not wrap this method in
	// a transaction-scoped advisory lock because it performs remote WSAA/WSFE
	// calls. AssignNumber commits the reservation before Authorize.
	return processor.processSerial(ctx, lease)
}

func (processor *Processor) processSerial(ctx context.Context, lease Lease) error {
	voucher := lease.Voucher
	freshNumber := false
	if voucher.Number == 0 {
		last, err := processor.authority.LastAuthorized(ctx, voucher)
		if err != nil {
			return fmt.Errorf("get last fiscal number: %w", err)
		}
		voucher, err = processor.repository.AssignNumber(
			ctx, voucher.ID, lease.Token, last+1, processor.now().UTC(),
		)
		if err != nil {
			return fmt.Errorf("assign fiscal number: %w", err)
		}
		freshNumber = true
	}

	if !freshNumber {
		lookup, err := processor.authority.Consult(ctx, voucher)
		if err != nil {
			return processor.keepUncertain(ctx, voucher, lease.Token, "authority_consult_failed", err)
		}
		if lookup.Found {
			return processor.applyAuthorityResult(ctx, voucher, lease.Token, lookup.Result)
		}
		last, err := processor.authority.LastAuthorized(ctx, voucher)
		if err != nil {
			return processor.keepUncertain(ctx, voucher, lease.Token, "authority_sequence_unknown", err)
		}
		if last >= voucher.Number {
			return processor.keepUncertain(ctx, voucher, lease.Token, "authority_sequence_conflict", ErrSequenceConflict)
		}
	}

	var err error
	voucher, err = processor.repository.MarkProcessing(ctx, voucher.ID, lease.Token, processor.now().UTC())
	if err != nil {
		return fmt.Errorf("mark fiscal voucher processing: %w", err)
	}
	result, err := processor.authority.Authorize(ctx, voucher)
	if err != nil {
		code := "authority_request_failed"
		if errors.Is(err, ErrUncertainResponse) {
			code = "authority_response_uncertain"
		}
		return processor.keepUncertain(ctx, voucher, lease.Token, code, err)
	}
	return processor.applyAuthorityResult(ctx, voucher, lease.Token, result)
}

func (processor *Processor) applyAuthorityResult(
	ctx context.Context,
	voucher Voucher,
	leaseToken string,
	result AuthorityResult,
) error {
	if result.Number == 0 {
		result.Number = voucher.Number
	}
	if result.Number != voucher.Number {
		return processor.keepUncertain(ctx, voucher, leaseToken, "authority_number_mismatch", ErrSequenceConflict)
	}
	authorization := Authorization{
		Decision:     result.Decision,
		Code:         result.Code,
		ExpiresOn:    result.ExpiresOn,
		Number:       result.Number,
		ProcessedAt:  result.ProcessedAt.UTC(),
		Observations: append([]AuthorityNote(nil), result.Observations...),
		Errors:       append([]AuthorityNote(nil), result.Errors...),
		ResponseHash: responseDigest(result.RawResponse),
	}
	if err := authorization.Validate(); err != nil {
		return processor.keepUncertain(ctx, voucher, leaseToken, "invalid_authority_result", err)
	}
	if authorization.Decision == DecisionRejected {
		if err := processor.repository.MarkRejected(ctx, voucher.ID, leaseToken, authorization); err != nil {
			return fmt.Errorf("mark fiscal voucher rejected: %w", err)
		}
		return nil
	}

	artifacts, err := processor.persistArtifacts(ctx, voucher, authorization, result.RawResponse)
	if err != nil {
		return processor.keepUncertain(ctx, voucher, leaseToken, "artifact_persistence_failed", err)
	}
	for _, artifact := range artifacts {
		if artifact.Kind == "authority_response" {
			authorization.ResponseObject = artifact.Key
			break
		}
	}
	finalization := Finalization{
		VoucherID:     voucher.ID,
		LeaseToken:    leaseToken,
		Authorization: authorization,
		Artifacts:     artifacts,
		Posting: PostingIntent{
			OrganizationID: voucher.OrganizationID,
			VoucherID:      voucher.ID,
			Source:         voucher.Source,
			Operation:      voucher.Operation,
			SnapshotHash:   voucher.Snapshot.Hash(),
			AuthorityCode:  authorization.Code,
		},
	}
	if err := processor.repository.FinalizeAuthorized(ctx, finalization); err != nil {
		return processor.keepUncertain(ctx, voucher, leaseToken, "authorization_finalize_failed", err)
	}
	return nil
}

func (processor *Processor) persistArtifacts(
	ctx context.Context,
	voucher Voucher,
	authorization Authorization,
	rawResponse []byte,
) ([]ArtifactReference, error) {
	rendered := make([]RenderedArtifact, 0, 3)
	if len(rawResponse) > 0 {
		rendered = append(rendered, RenderedArtifact{
			Kind:        "authority_response",
			ContentType: "application/xml",
			Body:        append([]byte(nil), rawResponse...),
		})
	}
	if processor.renderer != nil {
		artifacts, err := processor.renderer.Render(ctx, voucher.Snapshot, authorization)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, artifacts...)
	}
	if len(rendered) == 0 {
		return nil, nil
	}
	if processor.objects == nil {
		return nil, errors.New("fiscal object store is required for rendered artifacts")
	}

	references := make([]ArtifactReference, 0, len(rendered))
	kinds := make(map[string]struct{}, len(rendered))
	for _, artifact := range rendered {
		kind := strings.TrimSpace(artifact.Kind)
		if kind == "" || strings.TrimSpace(artifact.ContentType) == "" || len(artifact.Body) == 0 {
			return nil, errors.New("rendered fiscal artifact is incomplete")
		}
		if _, duplicate := kinds[kind]; duplicate {
			return nil, fmt.Errorf("duplicate rendered fiscal artifact kind %q", kind)
		}
		kinds[kind] = struct{}{}
		sum := sha256.Sum256(artifact.Body)
		digest := hex.EncodeToString(sum[:])
		objectKey := path.Join(
			"fiscal",
			voucher.OrganizationID.String(),
			voucher.ID.String(),
			kind+"-"+digest,
		)
		object := ImmutableObject{
			Key:         objectKey,
			ContentType: artifact.ContentType,
			Body:        append([]byte(nil), artifact.Body...),
			SHA256:      digest,
		}
		if err := processor.objects.PutImmutable(ctx, object); err != nil {
			return nil, fmt.Errorf("put immutable fiscal artifact %q: %w", kind, err)
		}
		references = append(references, ArtifactReference{
			Kind: kind, ContentType: artifact.ContentType, Key: objectKey, SHA256: digest,
		})
	}
	return references, nil
}

func (processor *Processor) keepUncertain(
	ctx context.Context,
	voucher Voucher,
	leaseToken, code string,
	cause error,
) error {
	failure := Failure{
		Code:       code,
		Message:    safeFailureMessage(cause),
		OccurredAt: processor.now().UTC(),
	}
	if err := processor.repository.MarkUncertain(ctx, voucher.ID, leaseToken, failure); err != nil {
		return errors.Join(cause, fmt.Errorf("mark fiscal voucher uncertain: %w", err))
	}
	return cause
}

func safeFailureMessage(cause error) string {
	if cause == nil {
		return "unknown fiscal processing failure"
	}
	message := strings.TrimSpace(cause.Error())
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

type Worker struct {
	queue     LeaseRepository
	processor *Processor
	workerID  string
	leaseFor  time.Duration
	now       func() time.Time
}

func NewWorker(queue LeaseRepository, processor *Processor, workerID string, leaseFor time.Duration) *Worker {
	if leaseFor <= 0 {
		leaseFor = 30 * time.Second
	}
	return &Worker{
		queue: queue, processor: processor, workerID: strings.TrimSpace(workerID),
		leaseFor: leaseFor, now: time.Now,
	}
}

func (worker *Worker) RunOnce(ctx context.Context) (bool, error) {
	if worker == nil || worker.queue == nil || worker.processor == nil || worker.workerID == "" {
		return false, errors.New("fiscal worker dependencies are incomplete")
	}
	now := worker.now().UTC()
	lease, err := worker.queue.LeaseNext(ctx, worker.workerID, now, now.Add(worker.leaseFor))
	if errors.Is(err, ErrNoWork) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	processContext, cancelProcess := context.WithCancel(ctx)
	heartbeatDone := make(chan error, 1)
	go worker.keepLease(processContext, cancelProcess, lease, heartbeatDone)

	processErr := worker.processor.Process(processContext, lease)
	cancelProcess()
	heartbeatErr := <-heartbeatDone
	if processErr != nil {
		if heartbeatErr != nil {
			return true, errors.Join(processErr, heartbeatErr)
		}
		return true, processErr
	}
	// Successful finalization is authoritative: it verified both the token and
	// an unexpired lease in the same transaction. A final heartbeat can race
	// with that terminal transition and observe zero updated rows.
	return true, nil
}

func (worker *Worker) keepLease(
	ctx context.Context,
	cancelProcess context.CancelFunc,
	lease Lease,
	done chan<- error,
) {
	interval := worker.leaseFor / 3
	if interval <= 0 {
		interval = worker.leaseFor / 2
	}
	if interval > 30*time.Second {
		interval = 30 * time.Second
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			done <- nil
			return
		case <-timer.C:
			until := worker.now().UTC().Add(worker.leaseFor)
			if err := worker.queue.RenewLease(ctx, lease.Voucher.ID, lease.Token, until); err != nil {
				cancelProcess()
				done <- fmt.Errorf("renew fiscal processing lease: %w", err)
				return
			}
			timer.Reset(interval)
		}
	}
}
