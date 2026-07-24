package fiscal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Operation string

const (
	OperationInvoice    Operation = "invoice"
	OperationCreditNote Operation = "credit_note"
	OperationDebitNote  Operation = "debit_note"
)

func (operation Operation) Valid() bool {
	switch operation {
	case OperationInvoice, OperationCreditNote, OperationDebitNote:
		return true
	default:
		return false
	}
}

type VoucherStatus string

const (
	StatusQueued     VoucherStatus = "queued"
	StatusProcessing VoucherStatus = "processing"
	StatusAuthorized VoucherStatus = "authorized"
	StatusRejected   VoucherStatus = "rejected"
	StatusUncertain  VoucherStatus = "uncertain"
)

func (status VoucherStatus) Terminal() bool {
	return status == StatusAuthorized || status == StatusRejected
}

type SourceReference struct {
	Kind string    `json:"kind"`
	ID   uuid.UUID `json:"id"`
}

func (reference SourceReference) Validate() error {
	if strings.TrimSpace(reference.Kind) == "" {
		return errors.New("source kind is required")
	}
	if reference.ID == uuid.Nil {
		return errors.New("source id is required")
	}
	return nil
}

// Voucher is the durable fiscal orchestration aggregate. OrganizationID never
// participates in JSON serialization; tenant identity comes from the verified
// request/session context and is only carried internally to persistence ports.
type Voucher struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"-"`
	Source         SourceReference `json:"source"`
	Operation      Operation       `json:"operation"`
	Environment    string          `json:"environment"`
	PointOfSale    int             `json:"point_of_sale"`
	AuthorityType  int             `json:"authority_type"`
	Number         int64           `json:"number,omitempty"`
	Status         VoucherStatus   `json:"status"`
	Snapshot       Snapshot        `json:"snapshot"`
	Authorization  *Authorization  `json:"authorization,omitempty"`
	Failure        *Failure        `json:"failure,omitempty"`
	CreatedBy      string          `json:"created_by"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type Failure struct {
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	OccurredAt time.Time `json:"occurred_at"`
}

type AuthorityDecision string

const (
	DecisionAuthorized AuthorityDecision = "authorized"
	DecisionRejected   AuthorityDecision = "rejected"
)

type AuthorityNote struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Authorization struct {
	Decision       AuthorityDecision `json:"decision"`
	Code           string            `json:"code,omitempty"`
	ExpiresOn      string            `json:"expires_on,omitempty"`
	Number         int64             `json:"number"`
	ProcessedAt    time.Time         `json:"processed_at"`
	Observations   []AuthorityNote   `json:"observations,omitempty"`
	Errors         []AuthorityNote   `json:"errors,omitempty"`
	ResponseHash   string            `json:"response_hash,omitempty"`
	ResponseObject string            `json:"response_object,omitempty"`
}

func (authorization Authorization) Validate() error {
	if authorization.Number <= 0 {
		return errors.New("authorization number is required")
	}
	switch authorization.Decision {
	case DecisionAuthorized:
		if strings.TrimSpace(authorization.Code) == "" {
			return errors.New("authorization code is required")
		}
	case DecisionRejected:
	default:
		return fmt.Errorf("invalid authority decision %q", authorization.Decision)
	}
	return nil
}

// Transition returns a changed copy and never mutates an already authorized or
// rejected voucher.
func (voucher Voucher) Transition(next VoucherStatus, at time.Time, failure *Failure) (Voucher, error) {
	if voucher.Status.Terminal() {
		return Voucher{}, fmt.Errorf("fiscal voucher in terminal state %q is immutable", voucher.Status)
	}
	allowed := false
	switch voucher.Status {
	case StatusQueued:
		allowed = next == StatusProcessing || next == StatusUncertain
	case StatusProcessing:
		allowed = next == StatusAuthorized || next == StatusRejected || next == StatusUncertain
	case StatusUncertain:
		allowed = next == StatusProcessing || next == StatusAuthorized || next == StatusRejected
	}
	if !allowed {
		return Voucher{}, fmt.Errorf("invalid fiscal voucher transition %q -> %q", voucher.Status, next)
	}
	voucher.Status = next
	voucher.UpdatedAt = at.UTC()
	voucher.Failure = failure
	return voucher, nil
}

func responseDigest(response []byte) string {
	if len(response) == 0 {
		return ""
	}
	sum := sha256.Sum256(response)
	return hex.EncodeToString(sum[:])
}
