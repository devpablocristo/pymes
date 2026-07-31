// Package repository contains the PostgreSQL adapter for commerce ports.
// and workers. It deliberately uses transaction-local org context before every
// tenant operation, matching the RLS policies in v3/db/migrations.
package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/domain"
	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIdempotencyConflict = errors.New("IDEMPOTENCY_KEY_REUSED")

func nullableText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

type Store struct {
	Pool *pgxpool.Pool
	Now  func() time.Time
}

func New(pool *pgxpool.Pool) *Store { return &Store{Pool: pool, Now: time.Now} }

func (s *Store) Clock() time.Time { return s.Now() }

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("database is not configured")
	}
	return s.Pool.Ping(ctx)
}

func (s *Store) CreateParty(ctx context.Context, party domain.Party) (domain.Party, error) {
	if party.ID == "" || party.OrganizationID == "" || party.DisplayName == "" || (party.Kind != "customer" && party.Kind != "supplier" && party.Kind != "both") {
		return domain.Party{}, fmt.Errorf("VALIDATION_ERROR")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Party{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", party.OrganizationID); err != nil {
		return domain.Party{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.parties (id,org_id,kind,display_name,tax_identifier) VALUES ($1,$2,$3,$4,$5)`, party.ID, party.OrganizationID, party.Kind, party.DisplayName, party.TaxIdentifier)
	if err != nil {
		return domain.Party{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Party{}, err
	}
	return party, nil
}

func (s *Store) GetParty(ctx context.Context, organizationID, partyID string) (domain.Party, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Party{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return domain.Party{}, err
	}
	var party domain.Party
	err = tx.QueryRow(ctx, `SELECT id,org_id,kind,display_name,COALESCE(tax_identifier,'') FROM app.parties WHERE id=$1`, partyID).Scan(&party.ID, &party.OrganizationID, &party.Kind, &party.DisplayName, &party.TaxIdentifier)
	if err != nil {
		return domain.Party{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Party{}, err
	}
	return party, nil
}

func (s *Store) CreatePurchaseAndQueue(ctx context.Context, p domain.Purchase) error {
	if p.ID == "" || p.OrganizationID == "" || p.SupplierRef == "" || p.ExternalDocumentRef == "" || !p.Total.Valid() {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", p.OrganizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	p.Status, p.CreatedAt = "confirmed", now
	if _, err = tx.Exec(ctx, `INSERT INTO app.purchases (id,org_id,supplier_ref,external_document_ref,amount,currency,status,snapshot_digest,correlation_id,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)`, p.ID, p.OrganizationID, p.SupplierRef, p.ExternalDocumentRef, p.Total.Amount, p.Total.Currency, p.Status, p.SnapshotDigest, p.CorrelationID, now); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"purchase_id": p.ID})
	digest := sha256.Sum256(payload)
	if _, err = tx.Exec(ctx, `INSERT INTO app.outbox(id,org_id,topic,payload,payload_hash,idempotency_key,correlation_id,available_at,created_at) VALUES($1,$2,'PurchasePostingRequested',$3,$4,$5,$6,$7,$7)`, uuid.New(), p.OrganizationID, payload, hex.EncodeToString(digest[:]), p.ID+":1", p.CorrelationID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CreatePaymentAndApplications(ctx context.Context, payment domain.Payment, applications []domain.OpenItemApplication) error {
	if payment.ID == "" || payment.OrganizationID == "" || payment.PartyRef == "" || (payment.Direction != "receipt" && payment.Direction != "disbursement") || !payment.Total.Valid() || payment.Total.Amount == "0" {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", payment.OrganizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	sum := new(big.Rat)
	for _, a := range applications {
		value, ok := new(big.Rat).SetString(a.Amount.Amount)
		if !ok || value.Sign() <= 0 || a.Amount.Currency != payment.Total.Currency || a.ID == "" || (a.DocumentKind != "sale" && a.DocumentKind != "purchase") || a.DocumentID == "" {
			return fmt.Errorf("VALIDATION_ERROR")
		}
		var documentAmount, documentCurrency, documentParty, documentStatus string
		switch a.DocumentKind {
		case "sale":
			if payment.Direction != "receipt" {
				return fmt.Errorf("VALIDATION_ERROR")
			}
			err = tx.QueryRow(ctx, `SELECT amount::text,currency,recipient_ref,status FROM app.sales WHERE id=$1 FOR UPDATE`, a.DocumentID).
				Scan(&documentAmount, &documentCurrency, &documentParty, &documentStatus)
		case "purchase":
			if payment.Direction != "disbursement" {
				return fmt.Errorf("VALIDATION_ERROR")
			}
			err = tx.QueryRow(ctx, `SELECT amount::text,currency,supplier_ref,status FROM app.purchases WHERE id=$1 FOR UPDATE`, a.DocumentID).
				Scan(&documentAmount, &documentCurrency, &documentParty, &documentStatus)
		}
		if err != nil || documentCurrency != payment.Total.Currency || documentParty != payment.PartyRef ||
			(documentStatus != "posted" && documentStatus != "partially_paid") {
			return fmt.Errorf("INVALID_APPLICATION_DOCUMENT")
		}
		var alreadyApplied string
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(sum(amount),0)::text
			FROM app.open_item_applications
			WHERE document_kind=$1 AND document_id=$2 AND status <> 'reversed'`,
			a.DocumentKind, a.DocumentID).Scan(&alreadyApplied); err != nil {
			return err
		}
		documentValue, ok := new(big.Rat).SetString(documentAmount)
		if !ok {
			return fmt.Errorf("VALIDATION_ERROR")
		}
		appliedValue, ok := new(big.Rat).SetString(alreadyApplied)
		if !ok || new(big.Rat).Sub(documentValue, appliedValue).Cmp(value) < 0 {
			return fmt.Errorf("OPEN_ITEM_AMOUNT_EXCEEDED")
		}
		sum.Add(sum, value)
	}
	total, _ := new(big.Rat).SetString(payment.Total.Amount)
	if sum.Cmp(total) > 0 {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO app.payments(id,org_id,direction,party_ref,amount,currency,status,correlation_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,'confirmed',$7,$8,$8)`, payment.ID, payment.OrganizationID, payment.Direction, payment.PartyRef, payment.Total.Amount, payment.Total.Currency, payment.CorrelationID, now); err != nil {
		return err
	}
	for _, a := range applications {
		if _, err = tx.Exec(ctx, `INSERT INTO app.open_item_applications(id,org_id,payment_id,document_kind,document_id,amount,currency) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.ID, payment.OrganizationID, payment.ID, a.DocumentKind, a.DocumentID, a.Amount.Amount, a.Amount.Currency); err != nil {
			return err
		}
	}
	payload, _ := json.Marshal(map[string]string{"payment_id": payment.ID})
	digest := sha256.Sum256(payload)
	if _, err = tx.Exec(ctx, `INSERT INTO app.outbox(id,org_id,topic,payload,payload_hash,idempotency_key,correlation_id,available_at,created_at) VALUES($1,$2,'PaymentPostingRequested',$3,$4,$5,$6,$7,$7)`, uuid.New(), payment.OrganizationID, payload, hex.EncodeToString(digest[:]), payment.ID+":1", payment.CorrelationID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CreateSaleAndQueueFiscal(ctx context.Context, sale domain.Sale, credentialRef string) (domain.Sale, error) {
	if sale.FiscalEnvironment == "" {
		sale.FiscalEnvironment = "homologation"
	}
	if sale.FiscalEnvironment != "homologation" && sale.FiscalEnvironment != "production" {
		return domain.Sale{}, fmt.Errorf("VALIDATION_ERROR")
	}
	now := s.Now().UTC()
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Sale{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.org_id', $1, true)", sale.OrganizationID); err != nil {
		return domain.Sale{}, err
	}
	var status string
	if err := tx.QueryRow(ctx, "SELECT status FROM app.organizations WHERE id=$1", sale.OrganizationID).Scan(&status); err != nil {
		return domain.Sale{}, fmt.Errorf("organization: %w", err)
	}
	if status != string(organizationdomain.Ready) {
		return domain.Sale{}, domain.ErrOrganizationNotReady
	}
	if strings.HasPrefix(sale.Voucher.DocumentType, "NC") || strings.HasPrefix(sale.Voucher.DocumentType, "ND") {
		var recipientRef, currency, sourceStatus string
		err = tx.QueryRow(ctx, `
			SELECT recipient_ref,currency,status
			FROM app.sales WHERE id=$1`, sale.SourceDocumentID).
			Scan(&recipientRef, &currency, &sourceStatus)
		if err != nil || recipientRef != sale.RecipientRef || currency != sale.Total.Currency ||
			(sourceStatus != "posted" && sourceStatus != "partially_paid" && sourceStatus != "paid") {
			return domain.Sale{}, fmt.Errorf("INVALID_SOURCE_DOCUMENT")
		}
	}
	if sale.Voucher.VoucherNumber < 1 {
		err = tx.QueryRow(ctx, `
			INSERT INTO app.fiscal_number_sequences
			  (org_id,fiscal_environment,point_of_sale,document_type,last_number,updated_at)
			VALUES ($1,$2,$3,$4,1,$5)
			ON CONFLICT (org_id,fiscal_environment,point_of_sale,document_type)
			DO UPDATE SET last_number=app.fiscal_number_sequences.last_number+1,updated_at=EXCLUDED.updated_at
			RETURNING last_number`,
			sale.OrganizationID, sale.FiscalEnvironment, sale.Voucher.PointOfSale,
			sale.Voucher.DocumentType, now).Scan(&sale.Voucher.VoucherNumber)
		if err != nil {
			return domain.Sale{}, err
		}
	} else {
		_, err = tx.Exec(ctx, `
			INSERT INTO app.fiscal_number_sequences
			  (org_id,fiscal_environment,point_of_sale,document_type,last_number,updated_at)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (org_id,fiscal_environment,point_of_sale,document_type)
			DO UPDATE SET last_number=GREATEST(app.fiscal_number_sequences.last_number,EXCLUDED.last_number),
			              updated_at=EXCLUDED.updated_at`,
			sale.OrganizationID, sale.FiscalEnvironment, sale.Voucher.PointOfSale,
			sale.Voucher.DocumentType, sale.Voucher.VoucherNumber, now)
		if err != nil {
			return domain.Sale{}, err
		}
	}
	frozenSnapshot, err := json.Marshal(map[string]any{
		"id": sale.ID, "recipient_ref": sale.RecipientRef, "voucher": sale.Voucher,
		"fiscal_environment": sale.FiscalEnvironment, "total": sale.Total,
		"source_document_id": sale.SourceDocumentID, "fiscal": json.RawMessage(sale.FiscalSnapshot),
	})
	if err != nil {
		return domain.Sale{}, err
	}
	snapshotDigest := sha256.Sum256(frozenSnapshot)
	sale.SnapshotDigest = hex.EncodeToString(snapshotDigest[:])
	_, err = tx.Exec(ctx, `
INSERT INTO app.sales (id,org_id,recipient_ref,point_of_sale,document_type,voucher_number,fiscal_environment,amount,currency,status,snapshot_digest,credential_ref,fiscal_snapshot,correlation_id,source_document_id,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$16)`, sale.ID, sale.OrganizationID, sale.RecipientRef, sale.Voucher.PointOfSale, sale.Voucher.DocumentType, sale.Voucher.VoucherNumber, sale.FiscalEnvironment, sale.Total.Amount, sale.Total.Currency, sale.Status, sale.SnapshotDigest, credentialRef, sale.FiscalSnapshot, sale.CorrelationID, nullableText(sale.SourceDocumentID), now)
	if err != nil {
		return domain.Sale{}, err
	}
	payload, err := json.Marshal(map[string]string{"sale_id": sale.ID, "credential_ref": credentialRef})
	if err != nil {
		return domain.Sale{}, err
	}
	digest := sha256.Sum256(payload)
	_, err = tx.Exec(ctx, `
INSERT INTO app.outbox (id,org_id,topic,payload,payload_hash,idempotency_key,correlation_id,available_at,created_at)
VALUES ($1,$2,'FiscalAuthorizationRequested',$3,$4,$5,$6,$7,$7)`, uuid.New(), sale.OrganizationID, payload, hex.EncodeToString(digest[:]), sale.ID+":1", sale.CorrelationID, now)
	if err != nil {
		return domain.Sale{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Sale{}, err
	}
	sale.CreatedAt, sale.UpdatedAt = now, now
	return sale, nil
}

func (s *Store) CreateAccountingReversal(ctx context.Context, value domain.AccountingReversal) (domain.AccountingReversal, error) {
	if value.ID == "" || value.OrganizationID == "" || value.DocumentID == "" ||
		value.EffectiveAt.IsZero() || strings.TrimSpace(value.Reason) == "" ||
		(value.DocumentKind != "purchase" && value.DocumentKind != "payment") {
		return domain.AccountingReversal{}, fmt.Errorf("VALIDATION_ERROR")
	}
	value.EffectiveAt = value.EffectiveAt.UTC().Truncate(time.Microsecond)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AccountingReversal{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", value.OrganizationID); err != nil {
		return domain.AccountingReversal{}, err
	}
	var status, journalEntryID string
	switch value.DocumentKind {
	case "purchase":
		err = tx.QueryRow(ctx, `SELECT status,COALESCE(journal_entry_id,'') FROM app.purchases WHERE id=$1 FOR UPDATE`, value.DocumentID).Scan(&status, &journalEntryID)
	case "payment":
		err = tx.QueryRow(ctx, `SELECT status,COALESCE(journal_entry_id,'') FROM app.payments WHERE id=$1 FOR UPDATE`, value.DocumentID).Scan(&status, &journalEntryID)
	}
	if err != nil || journalEntryID == "" || (status != "posted" && status != "partially_paid" && status != "paid") {
		return domain.AccountingReversal{}, fmt.Errorf("DOCUMENT_NOT_REVERSIBLE")
	}
	value.OriginalJournalEntryID, value.Status = journalEntryID, "requested"
	now := s.Now().UTC()
	tag, err := tx.Exec(ctx, `
		INSERT INTO app.accounting_reversals
			(id,org_id,document_kind,document_id,original_journal_entry_id,effective_at,reason,status,correlation_id,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'requested',$8,$9,$9)
		ON CONFLICT (org_id,document_kind,document_id) DO NOTHING`,
		value.ID, value.OrganizationID, value.DocumentKind, value.DocumentID, value.OriginalJournalEntryID,
		value.EffectiveAt, value.Reason, value.CorrelationID, now)
	if err != nil {
		return domain.AccountingReversal{}, err
	}
	if tag.RowsAffected() == 0 {
		var existing domain.AccountingReversal
		err = tx.QueryRow(ctx, `
			SELECT id,org_id,document_kind,document_id,original_journal_entry_id,effective_at,
			       reason,status,COALESCE(reversal_journal_entry_id,''),correlation_id
			FROM app.accounting_reversals
			WHERE org_id=$1 AND document_kind=$2 AND document_id=$3`,
			value.OrganizationID, value.DocumentKind, value.DocumentID).
			Scan(&existing.ID, &existing.OrganizationID, &existing.DocumentKind, &existing.DocumentID,
				&existing.OriginalJournalEntryID, &existing.EffectiveAt, &existing.Reason, &existing.Status,
				&existing.ReversalJournalEntryID, &existing.CorrelationID)
		if err != nil {
			return domain.AccountingReversal{}, err
		}
		if existing.ID != value.ID || !existing.EffectiveAt.Equal(value.EffectiveAt) || existing.Reason != value.Reason {
			return domain.AccountingReversal{}, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.AccountingReversal{}, err
		}
		return existing, nil
	}
	payload, _ := json.Marshal(map[string]string{"reversal_id": value.ID})
	digest := sha256.Sum256(payload)
	if _, err = tx.Exec(ctx, `
		INSERT INTO app.outbox
			(id,org_id,topic,payload,payload_hash,idempotency_key,correlation_id,available_at,created_at)
		VALUES ($1,$2,'AccountingReversalRequested',$3,$4,$5,$6,$7,$7)
		ON CONFLICT (org_id,topic,idempotency_key) DO NOTHING`,
		uuid.New(), value.OrganizationID, payload, hex.EncodeToString(digest[:]), value.ID+":1", value.CorrelationID, now); err != nil {
		return domain.AccountingReversal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AccountingReversal{}, err
	}
	return value, nil
}

func (s *Store) Lease(ctx context.Context, limit int, duration time.Duration) ([]domain.Event, error) {
	if limit < 1 || duration <= 0 {
		return nil, nil
	}
	now := s.Now().UTC()
	token := uuid.NewString()
	rows, err := s.Pool.Query(ctx, `
WITH candidates AS (
 SELECT id FROM app.outbox
 WHERE published_at IS NULL AND available_at <= $1 AND (lease_expires_at IS NULL OR lease_expires_at <= $1)
 ORDER BY available_at, created_at FOR UPDATE SKIP LOCKED LIMIT $2
)
UPDATE app.outbox o SET lease_token=$3, lease_expires_at=$4, attempts=o.attempts+1
FROM candidates c WHERE o.id=c.id
RETURNING o.id,o.org_id,o.topic,o.payload,o.payload_hash,o.idempotency_key,o.correlation_id,o.available_at,o.attempts,o.lease_token,o.lease_expires_at`, now, limit, token, now.Add(duration))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []domain.Event{}
	for rows.Next() {
		var event domain.Event
		if err := rows.Scan(&event.ID, &event.OrganizationID, &event.Topic, &event.Payload, &event.PayloadHash, &event.IdempotencyKey, &event.CorrelationID, &event.AvailableAt, &event.Attempts, &event.LeaseToken, &event.LeaseExpiresAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) MarkPublished(ctx context.Context, event domain.Event) error {
	result, err := s.Pool.Exec(ctx, `UPDATE app.outbox SET published_at=$1, lease_token=NULL, lease_expires_at=NULL WHERE id=$2 AND lease_token=$3`, s.Now().UTC(), event.ID, event.LeaseToken)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrLeaseLost
	}
	return nil
}

func (s *Store) Retry(ctx context.Context, event domain.Event) error {
	attempt := event.Attempts
	if attempt < 1 {
		attempt = 1
	}
	backoff := time.Second * time.Duration(1<<min(attempt-1, 6))
	if backoff > time.Minute {
		backoff = time.Minute
	}
	jitterDigest := sha256.Sum256([]byte(event.ID))
	jitterMillis := (int(jitterDigest[0])<<8 | int(jitterDigest[1])) % 1000
	result, err := s.Pool.Exec(ctx, `UPDATE app.outbox SET available_at=$1, lease_token=NULL, lease_expires_at=NULL WHERE id=$2 AND lease_token=$3`, s.Now().UTC().Add(backoff+time.Duration(jitterMillis)*time.Millisecond), event.ID, event.LeaseToken)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrLeaseLost
	}
	return nil
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
