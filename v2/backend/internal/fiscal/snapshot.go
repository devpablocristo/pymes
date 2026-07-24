package fiscal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const SnapshotVersion = 1

// PartySnapshot contains presentation and tax data as it existed at emission
// time. Later changes to a customer, supplier, or organization cannot change an
// authorized document.
type PartySnapshot struct {
	Name             string `json:"name"`
	TaxID            string `json:"tax_id,omitempty"`
	TaxCondition     string `json:"tax_condition,omitempty"`
	DocumentType     string `json:"document_type,omitempty"`
	DocumentNumber   string `json:"document_number,omitempty"`
	Address          string `json:"address,omitempty"`
	ActivityStartDay string `json:"activity_start_day,omitempty"`
}

type CurrencySnapshot struct {
	Code       string  `json:"code"`
	Rate       Decimal `json:"rate"`
	RateDate   string  `json:"rate_date,omitempty"`
	RateSource string  `json:"rate_source,omitempty"`
}

type FiscalLineSnapshot struct {
	Position      int     `json:"position"`
	Description   string  `json:"description"`
	Quantity      Decimal `json:"quantity"`
	UnitPrice     Decimal `json:"unit_price"`
	NetAmount     Decimal `json:"net_amount"`
	TaxCode       string  `json:"tax_code,omitempty"`
	TaxRate       Decimal `json:"tax_rate"`
	TaxAmount     Decimal `json:"tax_amount"`
	ExemptAmount  Decimal `json:"exempt_amount"`
	UntaxedAmount Decimal `json:"untaxed_amount"`
	TotalAmount   Decimal `json:"total_amount"`
	CostAmount    Decimal `json:"cost_amount"`
	CostConfirmed bool    `json:"cost_confirmed"`
}

type TaxSnapshot struct {
	Code        string  `json:"code"`
	Description string  `json:"description,omitempty"`
	BaseAmount  Decimal `json:"base_amount"`
	Rate        Decimal `json:"rate"`
	Amount      Decimal `json:"amount"`
}

type FiscalTotalsSnapshot struct {
	NetTaxed     Decimal `json:"net_taxed"`
	NetUntaxed   Decimal `json:"net_untaxed"`
	Exempt       Decimal `json:"exempt"`
	VAT          Decimal `json:"vat"`
	OtherTaxes   Decimal `json:"other_taxes"`
	Total        Decimal `json:"total"`
	Functional   Decimal `json:"functional_total"`
	Withholdings Decimal `json:"withholdings"`
	Perceptions  Decimal `json:"perceptions"`
}

type AssociatedDocumentSnapshot struct {
	VoucherID   string `json:"voucher_id"`
	Type        int    `json:"type"`
	PointOfSale int    `json:"point_of_sale"`
	Number      int64  `json:"number"`
	IssueDate   string `json:"issue_date"`
	IssuerTaxID string `json:"issuer_tax_id,omitempty"`
}

type FiscalSnapshot struct {
	Version            int                         `json:"version"`
	CountryCode        string                      `json:"country_code"`
	IssueDate          string                      `json:"issue_date"`
	ServiceFrom        string                      `json:"service_from,omitempty"`
	ServiceTo          string                      `json:"service_to,omitempty"`
	PaymentDue         string                      `json:"payment_due,omitempty"`
	Issuer             PartySnapshot               `json:"issuer"`
	Receiver           PartySnapshot               `json:"receiver"`
	Currency           CurrencySnapshot            `json:"currency"`
	Lines              []FiscalLineSnapshot        `json:"lines"`
	Taxes              []TaxSnapshot               `json:"taxes,omitempty"`
	Totals             FiscalTotalsSnapshot        `json:"totals"`
	AssociatedDocument *AssociatedDocumentSnapshot `json:"associated_document,omitempty"`
	Metadata           map[string]string           `json:"metadata,omitempty"`
}

// Snapshot is an immutable canonical JSON value plus its content hash.
type Snapshot struct {
	canonical []byte
	hash      [sha256.Size]byte
}

func NewSnapshot(document FiscalSnapshot) (Snapshot, error) {
	if err := validateFiscalSnapshot(document); err != nil {
		return Snapshot{}, err
	}
	document.Lines = append([]FiscalLineSnapshot(nil), document.Lines...)
	document.Taxes = append([]TaxSnapshot(nil), document.Taxes...)
	if document.Metadata != nil {
		metadata := make(map[string]string, len(document.Metadata))
		for key, value := range document.Metadata {
			metadata[key] = value
		}
		document.Metadata = metadata
	}
	raw, err := json.Marshal(document)
	if err != nil {
		return Snapshot{}, fmt.Errorf("marshal fiscal snapshot: %w", err)
	}
	return snapshotFromCanonical(raw), nil
}

func ParseSnapshot(raw []byte, expectedHash string) (Snapshot, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return Snapshot{}, errors.New("fiscal snapshot is empty")
	}
	var document FiscalSnapshot
	if err := json.Unmarshal(raw, &document); err != nil {
		return Snapshot{}, fmt.Errorf("parse fiscal snapshot: %w", err)
	}
	rebuilt, err := NewSnapshot(document)
	if err != nil {
		return Snapshot{}, err
	}
	if !bytes.Equal(bytes.TrimSpace(raw), rebuilt.canonical) {
		return Snapshot{}, errors.New("fiscal snapshot is not canonical")
	}
	if expectedHash != "" && rebuilt.Hash() != strings.ToLower(expectedHash) {
		return Snapshot{}, errors.New("fiscal snapshot hash mismatch")
	}
	return rebuilt, nil
}

func snapshotFromCanonical(raw []byte) Snapshot {
	canonical := append([]byte(nil), raw...)
	return Snapshot{canonical: canonical, hash: sha256.Sum256(canonical)}
}

func validateFiscalSnapshot(document FiscalSnapshot) error {
	if document.Version != SnapshotVersion {
		return fmt.Errorf("unsupported fiscal snapshot version %d", document.Version)
	}
	if len(document.CountryCode) != 2 || document.CountryCode != strings.ToUpper(document.CountryCode) {
		return errors.New("country_code must be a two-letter uppercase code")
	}
	if strings.TrimSpace(document.IssueDate) == "" {
		return errors.New("issue_date is required")
	}
	if strings.TrimSpace(document.Issuer.Name) == "" {
		return errors.New("issuer name is required")
	}
	if strings.TrimSpace(document.Receiver.Name) == "" {
		return errors.New("receiver name is required")
	}
	if strings.TrimSpace(document.Currency.Code) == "" || document.Currency.Rate.Cmp(Decimal{}) <= 0 {
		return errors.New("currency code and positive rate are required")
	}
	if len(document.Lines) == 0 {
		return errors.New("at least one fiscal line is required")
	}
	for index, line := range document.Lines {
		if line.Position <= 0 || strings.TrimSpace(line.Description) == "" {
			return fmt.Errorf("line %d has invalid position or description", index)
		}
		if line.Quantity.Cmp(Decimal{}) <= 0 {
			return fmt.Errorf("line %d quantity must be positive", index)
		}
		for label, amount := range map[string]Decimal{
			"net_amount": line.NetAmount, "tax_amount": line.TaxAmount,
			"exempt_amount": line.ExemptAmount, "untaxed_amount": line.UntaxedAmount,
			"total_amount": line.TotalAmount, "cost_amount": line.CostAmount,
		} {
			if amount.IsNegative() {
				return fmt.Errorf("line %d %s cannot be negative", index, label)
			}
		}
	}
	if document.Totals.Total.IsNegative() {
		return errors.New("snapshot total cannot be negative")
	}
	return nil
}

func (snapshot Snapshot) Hash() string {
	return hex.EncodeToString(snapshot.hash[:])
}

func (snapshot Snapshot) CanonicalJSON() []byte {
	return append([]byte(nil), snapshot.canonical...)
}

func (snapshot Snapshot) Document() (FiscalSnapshot, error) {
	var document FiscalSnapshot
	if err := json.Unmarshal(snapshot.canonical, &document); err != nil {
		return FiscalSnapshot{}, fmt.Errorf("decode fiscal snapshot: %w", err)
	}
	return document, nil
}

func (snapshot Snapshot) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Hash     string          `json:"hash"`
		Document json.RawMessage `json:"document"`
	}{
		Hash:     snapshot.Hash(),
		Document: snapshot.canonical,
	})
}

func (snapshot *Snapshot) UnmarshalJSON(raw []byte) error {
	if snapshot == nil {
		return errors.New("cannot unmarshal snapshot into nil receiver")
	}
	var wire struct {
		Hash     string          `json:"hash"`
		Document json.RawMessage `json:"document"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	parsed, err := ParseSnapshot(wire.Document, wire.Hash)
	if err != nil {
		return err
	}
	*snapshot = parsed
	return nil
}
