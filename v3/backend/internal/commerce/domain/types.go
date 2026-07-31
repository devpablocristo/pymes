// Package domain contains the commercial model and the ports owned by commerce.
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"time"
)

type SaleStatus string

const (
	SaleDraft                    SaleStatus = "draft"
	SaleFiscalPending            SaleStatus = "fiscal_pending"
	SaleFiscalUncertain          SaleStatus = "fiscal_uncertain"
	SaleFiscalRejected           SaleStatus = "fiscal_rejected"
	SaleAuthorizedPendingPosting SaleStatus = "authorized_pending_posting"
	SalePosted                   SaleStatus = "posted"
)

type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

func (m Money) Valid() bool {
	value, ok := new(big.Rat).SetString(m.Amount)
	return len(m.Currency) == 3 && ok && value.Sign() >= 0
}

type VoucherReference struct {
	PointOfSale   int    `json:"point_of_sale"`
	DocumentType  string `json:"document_type"`
	VoucherNumber int    `json:"voucher_number"`
}
type Party struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Kind           string `json:"kind"`
	DisplayName    string `json:"display_name"`
	TaxIdentifier  string `json:"tax_identifier,omitempty"`
}
type Purchase struct {
	ID                  string    `json:"id"`
	OrganizationID      string    `json:"organization_id"`
	SupplierRef         string    `json:"supplier_ref"`
	ExternalDocumentRef string    `json:"external_document_ref"`
	Total               Money     `json:"total"`
	Status              string    `json:"status"`
	SourceDocumentID    string    `json:"source_document_id,omitempty"`
	JournalEntryID      string    `json:"journal_entry_id,omitempty"`
	OpenItemID          string    `json:"open_item_id,omitempty"`
	SnapshotDigest      string    `json:"snapshot_digest"`
	CorrelationID       string    `json:"correlation_id"`
	CreatedAt           time.Time `json:"created_at"`
}
type Payment struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Direction      string    `json:"direction"`
	PartyRef       string    `json:"party_ref"`
	Total          Money     `json:"total"`
	Status         string    `json:"status"`
	JournalEntryID string    `json:"journal_entry_id,omitempty"`
	OpenItemID     string    `json:"open_item_id,omitempty"`
	CorrelationID  string    `json:"correlation_id"`
	CreatedAt      time.Time `json:"created_at"`
}
type OpenItemApplication struct {
	ID                      string `json:"id"`
	OrganizationID          string `json:"organization_id,omitempty"`
	PaymentID               string `json:"payment_id"`
	DocumentKind            string `json:"document_kind"`
	DocumentID              string `json:"document_id"`
	Amount                  Money  `json:"amount"`
	Status                  string `json:"status,omitempty"`
	DocumentOpenItemID      string `json:"document_open_item_id,omitempty"`
	PaymentOpenItemID       string `json:"payment_open_item_id,omitempty"`
	AccountingApplicationID string `json:"accounting_application_id,omitempty"`
}
type Sale struct {
	ID                string           `json:"id"`
	OrganizationID    string           `json:"organization_id"`
	RecipientRef      string           `json:"recipient_ref"`
	Voucher           VoucherReference `json:"voucher"`
	FiscalEnvironment string           `json:"fiscal_environment"`
	Total             Money            `json:"total"`
	Status            SaleStatus       `json:"status"`
	SnapshotDigest    string           `json:"snapshot_digest"`
	CAE               string           `json:"cae,omitempty"`
	JournalEntryID    string           `json:"journal_entry_id,omitempty"`
	OpenItemID        string           `json:"open_item_id,omitempty"`
	SourceDocumentID  string           `json:"source_document_id,omitempty"`
	CorrelationID     string           `json:"correlation_id"`
	FiscalSnapshot    json.RawMessage  `json:"fiscal_snapshot,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}
type FiscalRequest struct {
	RequestID      string           `json:"request_id"`
	OrganizationID string           `json:"organization_id"`
	CredentialRef  string           `json:"credential_ref"`
	Voucher        VoucherReference `json:"voucher"`
	Total          Money            `json:"total"`
	SnapshotDigest string           `json:"snapshot_digest"`
	CorrelationID  string           `json:"correlation_id"`
	FiscalSnapshot json.RawMessage  `json:"fiscal_snapshot,omitempty"`
}
type FiscalResult struct {
	RequestID           string   `json:"request_id"`
	OrganizationID      string   `json:"organization_id"`
	Status              string   `json:"status"`
	CAE                 string   `json:"cae,omitempty"`
	CAEExpiresOn        string   `json:"cae_expires_on,omitempty"`
	AuthorityResultCode string   `json:"authority_result_code,omitempty"`
	AuthorityMessages   []string `json:"authority_messages,omitempty"`
	ArtifactRef         string   `json:"artifact_ref,omitempty"`
	SnapshotDigest      string   `json:"snapshot_digest"`
	ObservedAt          string   `json:"observed_at"`
	CorrelationID       string   `json:"correlation_id"`
}
type PendingFiscal struct {
	Sale          Sale
	CredentialRef string
}
type FiscalClient interface {
	Authorize(context.Context, FiscalRequest) (FiscalResult, error)
	Consult(context.Context, FiscalRequest) (FiscalResult, error)
}
type PostingLine struct {
	AccountCode string `json:"account_code"`
	Debit       Money  `json:"debit"`
	Credit      Money  `json:"credit"`
	Memo        string `json:"memo,omitempty"`
	OpenItem    bool   `json:"open_item,omitempty"`
	PartyRef    string `json:"party_ref,omitempty"`
}
type PostingCommand struct {
	CommandID      string        `json:"command_id"`
	OrganizationID string        `json:"organization_id"`
	SourceType     string        `json:"source_type"`
	SourceID       string        `json:"source_id"`
	SourceVersion  int           `json:"source_version"`
	SnapshotDigest string        `json:"snapshot_digest"`
	CorrelationID  string        `json:"correlation_id"`
	EffectiveAt    time.Time     `json:"effective_at"`
	Description    string        `json:"description"`
	Lines          []PostingLine `json:"lines"`
}
type AccountingEvent struct {
	EventID        string   `json:"event_id"`
	CommandID      string   `json:"command_id"`
	OrganizationID string   `json:"organization_id"`
	Status         string   `json:"status"`
	JournalEntryID string   `json:"journal_entry_id,omitempty"`
	OpenItemIDs    []string `json:"open_item_ids,omitempty"`
	ApplicationID  string   `json:"application_id,omitempty"`
	OccurredAt     string   `json:"occurred_at,omitempty"`
	CorrelationID  string   `json:"correlation_id,omitempty"`
}
type ReversalCommand struct {
	CommandID              string    `json:"command_id"`
	OrganizationID         string    `json:"organization_id"`
	OriginalJournalEntryID string    `json:"original_journal_entry_id"`
	EffectiveAt            time.Time `json:"effective_at"`
	Reason                 string    `json:"reason"`
	CorrelationID          string    `json:"correlation_id"`
}
type AccountingApplicationCommand struct {
	CommandID        string    `json:"command_id"`
	OrganizationID   string    `json:"organization_id"`
	DebitOpenItemID  string    `json:"debit_open_item_id"`
	CreditOpenItemID string    `json:"credit_open_item_id"`
	Amount           Money     `json:"amount"`
	AppliedAt        time.Time `json:"applied_at"`
	CorrelationID    string    `json:"correlation_id"`
}
type PendingAccountingApplication struct {
	ID               string
	OrganizationID   string
	SourceKind       string
	SourceID         string
	DebitOpenItemID  string
	CreditOpenItemID string
	Amount           Money
	Status           string
	ApplicationID    string
	CorrelationID    string
}
type AccountingReversal struct {
	ID                     string    `json:"id"`
	OrganizationID         string    `json:"organization_id"`
	DocumentKind           string    `json:"document_kind"`
	DocumentID             string    `json:"document_id"`
	OriginalJournalEntryID string    `json:"original_journal_entry_id"`
	EffectiveAt            time.Time `json:"effective_at"`
	Reason                 string    `json:"reason"`
	Status                 string    `json:"status"`
	ReversalJournalEntryID string    `json:"reversal_journal_entry_id,omitempty"`
	CorrelationID          string    `json:"correlation_id"`
}
type AccountingApplicationReversalCommand struct {
	CommandID      string    `json:"command_id"`
	OrganizationID string    `json:"organization_id"`
	ApplicationID  string    `json:"application_id"`
	ReversedAt     time.Time `json:"reversed_at"`
	Reason         string    `json:"reason"`
	CorrelationID  string    `json:"correlation_id"`
}
type AccountingClient interface {
	Post(context.Context, PostingCommand) (AccountingEvent, error)
	Reverse(context.Context, ReversalCommand) (AccountingEvent, error)
	ApplyOpenItem(context.Context, AccountingApplicationCommand) (AccountingEvent, error)
	ReverseOpenItemApplication(context.Context, AccountingApplicationReversalCommand) (AccountingEvent, error)
}
type Event struct {
	ID             string
	OrganizationID string
	Topic          string
	Payload        []byte
	PayloadHash    string
	IdempotencyKey string
	CorrelationID  string
	AvailableAt    time.Time
	Attempts       int
	LeaseToken     string
	LeaseExpiresAt time.Time
	PublishedAt    *time.Time
}

var ErrLeaseLost = errors.New("outbox lease lost")

var ErrOrganizationNotReady = errors.New("ORG_NOT_PROVISIONED")
