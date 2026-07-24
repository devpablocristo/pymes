package fiscal

import (
	"context"
	"crypto"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound            = errors.New("fiscal record not found")
	ErrNoWork              = errors.New("no fiscal work available")
	ErrIdempotencyConflict = errors.New("fiscal idempotency key conflicts with a different command")
	ErrSourceAlreadyUsed   = errors.New("source document already has a fiscal operation")
	ErrLeaseLost           = errors.New("fiscal worker lease lost")
	ErrSequenceConflict    = errors.New("fiscal authority sequence conflict")
	ErrUncertainResponse   = errors.New("fiscal authority response is uncertain")
)

// KMS keeps durable private key material under managed-key custody. WSAA
// signing uses PublicKey and SignDigest; an envelope adapter may decrypt the
// private key only transiently for one signature and must never persist or log
// the plaintext. Encryption methods cover other fiscal secrets at rest.
type KMS interface {
	Encrypt(ctx context.Context, keyReference string, plaintext, additionalData []byte) ([]byte, error)
	Decrypt(ctx context.Context, keyReference string, ciphertext, additionalData []byte) ([]byte, error)
	PublicKey(ctx context.Context, keyReference string) (crypto.PublicKey, error)
	SignDigest(ctx context.Context, keyReference string, digest []byte, hash crypto.Hash) ([]byte, error)
}

// SigningKeyImporter is the provisioning capability used by the certificate
// endpoint. Production implementations place the key under KMS/HSM custody
// (directly or with envelope encryption) and return an opaque kms://, vault://,
// or secret:// reference; the raw key is never written to PostgreSQL or object
// storage.
type SigningKeyImporter interface {
	ImportSigningKey(
		ctx context.Context,
		keyReference string,
		privateKeyPEM, additionalData []byte,
	) (string, error)
}

type ImmutableObject struct {
	Key         string
	ContentType string
	Body        []byte
	SHA256      string
}

// ObjectStore implementations must make PutImmutable idempotent for identical
// bytes and fail if the same key already contains different bytes.
type ObjectStore interface {
	PutImmutable(ctx context.Context, object ImmutableObject) error
	Get(ctx context.Context, key string) (ImmutableObject, error)
}

type QueueCommand struct {
	OrganizationID uuid.UUID
	IdempotencyKey string
	Fingerprint    string
	Source         SourceReference
	Operation      Operation
	Environment    string
	PointOfSale    int
	AuthorityType  int
	Snapshot       Snapshot
	Actor          string
	RequestedAt    time.Time
}

type QueueResult struct {
	Voucher Voucher
	Replay  bool
}

// CommandRepository atomically enforces both (organization,idempotency key)
// fingerprint semantics and (organization,source,operation) uniqueness.
type CommandRepository interface {
	Enqueue(ctx context.Context, command QueueCommand) (QueueResult, error)
	Get(ctx context.Context, organizationID, voucherID uuid.UUID) (Voucher, error)
}

type Lease struct {
	Voucher Voucher
	Token   string
	Until   time.Time
}

// LeaseRepository is implemented with durable row leasing (normally
// FOR UPDATE SKIP LOCKED). Every mutation verifies Token to prevent stale
// workers from finalizing work after losing their lease.
type LeaseRepository interface {
	LeaseNext(ctx context.Context, workerID string, now, until time.Time) (Lease, error)
	RenewLease(ctx context.Context, voucherID uuid.UUID, token string, until time.Time) error
	ReleaseLease(ctx context.Context, voucherID uuid.UUID, token string, retryAt time.Time, cause string) error
}

type SerialKey struct {
	OrganizationID uuid.UUID
	Environment    string
	PointOfSale    int
	AuthorityType  int
}

// SerialLocker is available only for short database-only series operations.
// Never wrap Authority calls with a transaction-scoped advisory lock; worker
// serialization is enforced durably by the unresolved-series lease invariant.
type SerialLocker interface {
	WithinSerial(ctx context.Context, key SerialKey, work func(context.Context) error) error
}

type AuthorityResult struct {
	Decision     AuthorityDecision
	Code         string
	ExpiresOn    string
	Number       int64
	ProcessedAt  time.Time
	Observations []AuthorityNote
	Errors       []AuthorityNote
	RawResponse  []byte
}

type AuthorityLookup struct {
	Found  bool
	Result AuthorityResult
}

// Authority is the country adapter used by the neutral worker. An adapter must
// classify a lost/ambiguous authorization response with ErrUncertainResponse.
type Authority interface {
	LastAuthorized(ctx context.Context, voucher Voucher) (int64, error)
	Authorize(ctx context.Context, voucher Voucher) (AuthorityResult, error)
	Consult(ctx context.Context, voucher Voucher) (AuthorityLookup, error)
}

type RenderedArtifact struct {
	Kind        string
	ContentType string
	Body        []byte
}

// ArtifactRenderer renders only from Snapshot and authority output; it never
// reloads a mutable commercial document.
type ArtifactRenderer interface {
	Render(ctx context.Context, snapshot Snapshot, authorization Authorization) ([]RenderedArtifact, error)
}

type ArtifactReference struct {
	Kind        string `json:"kind"`
	ContentType string `json:"content_type"`
	Key         string `json:"key"`
	SHA256      string `json:"sha256"`
}

type PostingIntent struct {
	OrganizationID uuid.UUID
	VoucherID      uuid.UUID
	Source         SourceReference
	Operation      Operation
	SnapshotHash   string
	AuthorityCode  string
}

type Finalization struct {
	VoucherID     uuid.UUID
	LeaseToken    string
	Authorization Authorization
	Artifacts     []ArtifactReference
	Posting       PostingIntent
}

// VoucherRepository persists state transitions. FinalizeAuthorized is one
// database transaction that stores the authorization, queues/creates the local
// accounting effect, and makes the immutable artifact references visible.
type VoucherRepository interface {
	CommandRepository
	LeaseRepository
	AssignNumber(ctx context.Context, voucherID uuid.UUID, leaseToken string, number int64, at time.Time) (Voucher, error)
	MarkProcessing(ctx context.Context, voucherID uuid.UUID, leaseToken string, at time.Time) (Voucher, error)
	MarkUncertain(ctx context.Context, voucherID uuid.UUID, leaseToken string, failure Failure) error
	MarkRejected(ctx context.Context, voucherID uuid.UUID, leaseToken string, authorization Authorization) error
	FinalizeAuthorized(ctx context.Context, finalization Finalization) error
}

// WorkerPort permits the process host to run one leased fiscal job or a
// recurring loop without coupling the domain to a particular scheduler.
type WorkerPort interface {
	RunOnce(ctx context.Context) (bool, error)
}
