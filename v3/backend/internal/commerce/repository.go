// Package repository contains the PostgreSQL adapter for commerce ports.
// and workers. It deliberately uses transaction-local org context before every
// tenant operation, matching the RLS policies in v3/db/migrations.
// architecture:adapter repository
package commerce

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

	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	organizationdomain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func nullableText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func normalizeOrigin(
	origin domain.OriginMetadata,
	fallbackCorrelationID, operation, sourceID string,
) domain.OriginMetadata {
	origin.RequestID = strings.TrimSpace(origin.RequestID)
	origin.CorrelationID = strings.TrimSpace(origin.CorrelationID)
	origin.ActorRef = strings.TrimSpace(origin.ActorRef)
	if origin.SourceVersion < 1 {
		origin.SourceVersion = 1
	}
	if origin.RequestID == "" {
		identity := fmt.Sprintf("%s\x00%s", strings.TrimSpace(operation), strings.TrimSpace(sourceID))
		digest := sha256.Sum256([]byte(identity))
		origin.RequestID = "internal:" + hex.EncodeToString(digest[:16])
	}
	if origin.CorrelationID == "" {
		origin.CorrelationID = strings.TrimSpace(fallbackCorrelationID)
	}
	if origin.CorrelationID == "" {
		origin.CorrelationID = origin.RequestID
	}
	if origin.ActorRef == "" {
		origin.ActorRef = "system:internal"
	}
	return origin
}

func originFromIdempotencyCommand(
	current domain.OriginMetadata,
	command domain.IdempotencyCommand,
) domain.OriginMetadata {
	if strings.TrimSpace(command.RequestID) != "" {
		current.RequestID = command.RequestID
	}
	if strings.TrimSpace(command.CorrelationID) != "" {
		current.CorrelationID = command.CorrelationID
	}
	if strings.TrimSpace(command.ActorRef) != "" {
		current.ActorRef = command.ActorRef
	}
	if command.SourceVersion > 0 {
		current.SourceVersion = command.SourceVersion
	}
	return normalizeOrigin(
		current,
		command.CorrelationID,
		command.Operation,
		command.SourceID,
	)
}

func originSourceVersion(origin domain.OriginMetadata) int {
	if origin.SourceVersion > 0 {
		return origin.SourceVersion
	}
	return 1
}

func repositoryIdempotencyKey(organizationID, operation, sourceID string, sourceVersion int) string {
	identity := fmt.Sprintf("%s\x00%s\x00%s\x00%d", organizationID, operation, sourceID, sourceVersion)
	digest := sha256.Sum256([]byte(identity))
	return "pymes-v3:" + hex.EncodeToString(digest[:])
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
	tx, err := beginTenantTransaction(ctx, s, party.OrganizationID)
	if err != nil {
		return domain.Party{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := createPartyTx(ctx, tx, party)
	if err != nil {
		return domain.Party{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Party{}, err
	}
	return result, nil
}

func (s *Store) CreatePartyIdempotent(ctx context.Context, command domain.IdempotencyCommand, party domain.Party) (domain.Party, error) {
	return executeIdempotent(ctx, s, command, func(tx pgx.Tx) (domain.Party, error) {
		return createPartyTx(ctx, tx, party)
	})
}

func createPartyTx(ctx context.Context, tx pgx.Tx, party domain.Party) (domain.Party, error) {
	if party.ID == "" || party.OrganizationID == "" || party.DisplayName == "" ||
		(party.Kind != "customer" && party.Kind != "supplier" && party.Kind != "both") {
		return domain.Party{}, fmt.Errorf("VALIDATION_ERROR")
	}
	_, err := tx.Exec(ctx, `INSERT INTO app.parties (id,org_id,kind,display_name,tax_identifier) VALUES ($1,$2,$3,$4,$5)`, party.ID, party.OrganizationID, party.Kind, party.DisplayName, party.TaxIdentifier)
	if err != nil {
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
	err = tx.QueryRow(ctx, `SELECT id,org_id,kind,display_name,COALESCE(tax_identifier,'') FROM app.parties WHERE org_id=$1 AND id=$2`, organizationID, partyID).Scan(&party.ID, &party.OrganizationID, &party.Kind, &party.DisplayName, &party.TaxIdentifier)
	if err != nil {
		return domain.Party{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Party{}, err
	}
	return party, nil
}

func (s *Store) CreatePurchaseAndQueue(ctx context.Context, p domain.Purchase) error {
	if p.ID == "" || p.OrganizationID == "" || p.SupplierRef == "" ||
		p.ExternalDocumentRef == "" || p.ValidateAccountingAmounts() != nil {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	tx, err := beginTenantTransaction(ctx, s, p.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = s.createPurchaseAndQueueTx(ctx, tx, p); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CreatePurchaseAndQueueIdempotent(ctx context.Context, command domain.IdempotencyCommand, purchase domain.Purchase) (domain.Purchase, error) {
	purchase.Origin = originFromIdempotencyCommand(purchase.Origin, command)
	purchase.CorrelationID = purchase.Origin.CorrelationID
	return executeIdempotent(ctx, s, command, func(tx pgx.Tx) (domain.Purchase, error) {
		return s.createPurchaseAndQueueTx(ctx, tx, purchase)
	})
}

func (s *Store) createPurchaseAndQueueTx(ctx context.Context, tx pgx.Tx, p domain.Purchase) (domain.Purchase, error) {
	if p.ID == "" || p.OrganizationID == "" || p.SupplierRef == "" ||
		p.ExternalDocumentRef == "" || p.ValidateAccountingAmounts() != nil {
		return domain.Purchase{}, fmt.Errorf("VALIDATION_ERROR")
	}
	p.Origin = normalizeOrigin(p.Origin, p.CorrelationID, domain.OperationCreatePurchase, p.ID)
	p.CorrelationID = p.Origin.CorrelationID
	now := s.Now().UTC()
	p.Status, p.CreatedAt = "confirmed", now
	if p.VATBreakdown == nil {
		p.VATBreakdown = []domain.VATBreakdownItem{}
	}
	vatBreakdown, err := json.Marshal(p.VATBreakdown)
	if err != nil {
		return domain.Purchase{}, fmt.Errorf("encode VAT breakdown: %w", err)
	}
	var exchangeRate any
	if p.ExchangeRate != "" {
		exchangeRate = p.ExchangeRate
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.purchases (
			id,org_id,supplier_ref,external_document_ref,issue_date,amount,currency,
			net_amount,exempt_amount,vat_breakdown,exchange_rate,
			status,snapshot_digest,request_id,actor_ref,source_version,
			correlation_id,created_at,updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$18
		)`,
		p.ID, p.OrganizationID, p.SupplierRef, p.ExternalDocumentRef,
		p.IssueDate, p.Total.Amount, p.Total.Currency, p.NetAmount, p.ExemptAmount,
		vatBreakdown, exchangeRate, p.Status, p.SnapshotDigest,
		p.Origin.RequestID, p.Origin.ActorRef, p.Origin.SourceVersion,
		p.CorrelationID, now,
	); err != nil {
		return domain.Purchase{}, err
	}
	payload, _ := json.Marshal(map[string]string{"purchase_id": p.ID})
	digest := sha256.Sum256(payload)
	idempotencyKey := repositoryIdempotencyKey(
		p.OrganizationID, "accounting.post", p.ID, p.Origin.SourceVersion,
	)
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.outbox (
			id,org_id,topic,payload,payload_hash,idempotency_key,
			request_id,actor_ref,source_version,snapshot_digest,
			correlation_id,available_at,created_at
		) VALUES (
			$1,$2,'PurchasePostingRequested',$3,$4,$5,$6,$7,$8,$9,$10,$11,$11
		)`,
		uuid.New(), p.OrganizationID, payload, hex.EncodeToString(digest[:]), idempotencyKey,
		p.Origin.RequestID, p.Origin.ActorRef, p.Origin.SourceVersion, p.SnapshotDigest,
		p.CorrelationID, now,
	); err != nil {
		return domain.Purchase{}, err
	}
	return p, nil
}

func (s *Store) CreatePaymentAndApplications(ctx context.Context, payment domain.Payment, applications []domain.OpenItemApplication) error {
	if payment.ID == "" || payment.OrganizationID == "" || payment.PartyRef == "" || (payment.Direction != "receipt" && payment.Direction != "disbursement") || !payment.Total.Valid() || payment.Total.Amount == "0" {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	tx, err := beginTenantTransaction(ctx, s, payment.OrganizationID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = s.createPaymentAndApplicationsTx(ctx, tx, payment, applications); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CreatePaymentAndApplicationsIdempotent(ctx context.Context, command domain.IdempotencyCommand, payment domain.Payment, applications []domain.OpenItemApplication) (domain.Payment, error) {
	payment.Origin = originFromIdempotencyCommand(payment.Origin, command)
	payment.CorrelationID = payment.Origin.CorrelationID
	return executeIdempotent(ctx, s, command, func(tx pgx.Tx) (domain.Payment, error) {
		return s.createPaymentAndApplicationsTx(ctx, tx, payment, applications)
	})
}

func (s *Store) createPaymentAndApplicationsTx(ctx context.Context, tx pgx.Tx, payment domain.Payment, applications []domain.OpenItemApplication) (domain.Payment, error) {
	if payment.ID == "" || payment.OrganizationID == "" || payment.PartyRef == "" ||
		(payment.Direction != "receipt" && payment.Direction != "disbursement") ||
		!payment.Total.Valid() || payment.Total.Amount == "0" {
		return domain.Payment{}, fmt.Errorf("VALIDATION_ERROR")
	}
	payment.Origin = normalizeOrigin(
		payment.Origin, payment.CorrelationID, domain.OperationCreatePayment, payment.ID,
	)
	payment.CorrelationID = payment.Origin.CorrelationID
	if payment.SnapshotDigest == "" {
		payment.SnapshotDigest = paymentSnapshotDigest(payment)
	}
	now := s.Now().UTC()
	sum := new(big.Rat)
	for _, a := range applications {
		value, ok := new(big.Rat).SetString(a.Amount.Amount)
		if !ok || value.Sign() <= 0 || a.Amount.Currency != payment.Total.Currency || a.ID == "" || (a.DocumentKind != "sale" && a.DocumentKind != "purchase") || a.DocumentID == "" {
			return domain.Payment{}, fmt.Errorf("VALIDATION_ERROR")
		}
		var documentAmount, documentCurrency, documentParty, documentStatus string
		switch a.DocumentKind {
		case "sale":
			if payment.Direction != "receipt" {
				return domain.Payment{}, fmt.Errorf("VALIDATION_ERROR")
			}
			err := tx.QueryRow(ctx, `SELECT amount::text,currency,recipient_ref,status FROM app.sales WHERE org_id=$1 AND id=$2 FOR UPDATE`, payment.OrganizationID, a.DocumentID).
				Scan(&documentAmount, &documentCurrency, &documentParty, &documentStatus)
			if err != nil {
				return domain.Payment{}, fmt.Errorf("INVALID_APPLICATION_DOCUMENT")
			}
		case "purchase":
			if payment.Direction != "disbursement" {
				return domain.Payment{}, fmt.Errorf("VALIDATION_ERROR")
			}
			err := tx.QueryRow(ctx, `SELECT amount::text,currency,supplier_ref,status FROM app.purchases WHERE org_id=$1 AND id=$2 FOR UPDATE`, payment.OrganizationID, a.DocumentID).
				Scan(&documentAmount, &documentCurrency, &documentParty, &documentStatus)
			if err != nil {
				return domain.Payment{}, fmt.Errorf("INVALID_APPLICATION_DOCUMENT")
			}
		}
		if documentCurrency != payment.Total.Currency || documentParty != payment.PartyRef ||
			(documentStatus != "posted" && documentStatus != "partially_paid") {
			return domain.Payment{}, fmt.Errorf("INVALID_APPLICATION_DOCUMENT")
		}
		var alreadyApplied string
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(sum(amount),0)::text
			FROM app.open_item_applications
			WHERE org_id=$1 AND document_kind=$2 AND document_id=$3 AND status <> 'reversed'`,
			payment.OrganizationID, a.DocumentKind, a.DocumentID).Scan(&alreadyApplied); err != nil {
			return domain.Payment{}, err
		}
		documentValue, ok := new(big.Rat).SetString(documentAmount)
		if !ok {
			return domain.Payment{}, fmt.Errorf("VALIDATION_ERROR")
		}
		appliedValue, ok := new(big.Rat).SetString(alreadyApplied)
		if !ok || new(big.Rat).Sub(documentValue, appliedValue).Cmp(value) < 0 {
			return domain.Payment{}, fmt.Errorf("OPEN_ITEM_AMOUNT_EXCEEDED")
		}
		sum.Add(sum, value)
	}
	total, _ := new(big.Rat).SetString(payment.Total.Amount)
	if sum.Cmp(total) > 0 {
		return domain.Payment{}, fmt.Errorf("VALIDATION_ERROR")
	}
	payment.Status, payment.CreatedAt = "confirmed", now
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.payments (
			id,org_id,direction,party_ref,amount,currency,status,snapshot_digest,
			request_id,actor_ref,source_version,correlation_id,created_at,updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,'confirmed',$7,$8,$9,$10,$11,$12,$12
		)`,
		payment.ID, payment.OrganizationID, payment.Direction, payment.PartyRef,
		payment.Total.Amount, payment.Total.Currency, payment.SnapshotDigest,
		payment.Origin.RequestID, payment.Origin.ActorRef, payment.Origin.SourceVersion,
		payment.CorrelationID, now,
	); err != nil {
		return domain.Payment{}, err
	}
	for _, a := range applications {
		if _, err := tx.Exec(ctx, `INSERT INTO app.open_item_applications(id,org_id,payment_id,document_kind,document_id,amount,currency) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.ID, payment.OrganizationID, payment.ID, a.DocumentKind, a.DocumentID, a.Amount.Amount, a.Amount.Currency); err != nil {
			return domain.Payment{}, err
		}
	}
	payload, _ := json.Marshal(map[string]string{"payment_id": payment.ID})
	digest := sha256.Sum256(payload)
	idempotencyKey := repositoryIdempotencyKey(
		payment.OrganizationID, "accounting.post", payment.ID, payment.Origin.SourceVersion,
	)
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.outbox (
			id,org_id,topic,payload,payload_hash,idempotency_key,
			request_id,actor_ref,source_version,snapshot_digest,
			correlation_id,available_at,created_at
		) VALUES (
			$1,$2,'PaymentPostingRequested',$3,$4,$5,$6,$7,$8,$9,$10,$11,$11
		)`,
		uuid.New(), payment.OrganizationID, payload, hex.EncodeToString(digest[:]), idempotencyKey,
		payment.Origin.RequestID, payment.Origin.ActorRef, payment.Origin.SourceVersion,
		payment.SnapshotDigest, payment.CorrelationID, now,
	); err != nil {
		return domain.Payment{}, err
	}
	return payment, nil
}

func (s *Store) CreateSaleAndQueueFiscal(ctx context.Context, sale domain.Sale, credentialRef string) (domain.Sale, error) {
	tx, err := beginTenantTransaction(ctx, s, sale.OrganizationID)
	if err != nil {
		return domain.Sale{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := s.createSaleAndQueueFiscalTx(ctx, tx, sale, credentialRef)
	if err != nil {
		return domain.Sale{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Sale{}, err
	}
	return result, nil
}

func (s *Store) CreateSaleAndQueueFiscalIdempotent(ctx context.Context, command domain.IdempotencyCommand, sale domain.Sale, credentialRef string) (domain.Sale, error) {
	sale.Origin = originFromIdempotencyCommand(sale.Origin, command)
	sale.CorrelationID = sale.Origin.CorrelationID
	return executeIdempotent(ctx, s, command, func(tx pgx.Tx) (domain.Sale, error) {
		return s.createSaleAndQueueFiscalTx(ctx, tx, sale, credentialRef)
	})
}

func (s *Store) createSaleAndQueueFiscalTx(ctx context.Context, tx pgx.Tx, sale domain.Sale, credentialRef string) (domain.Sale, error) {
	if sale.ID == "" || sale.OrganizationID == "" || credentialRef == "" || !sale.Total.Valid() {
		return domain.Sale{}, fmt.Errorf("VALIDATION_ERROR")
	}
	if sale.FiscalEnvironment == "" {
		sale.FiscalEnvironment = "homologation"
	}
	if sale.FiscalEnvironment != "homologation" && sale.FiscalEnvironment != "production" {
		return domain.Sale{}, fmt.Errorf("VALIDATION_ERROR")
	}
	sale.Origin = normalizeOrigin(sale.Origin, sale.CorrelationID, domain.OperationCreateSale, sale.ID)
	sale.CorrelationID = sale.Origin.CorrelationID
	now := s.Now().UTC()
	var status string
	if err := tx.QueryRow(ctx, "SELECT status FROM app.organizations WHERE id=$1", sale.OrganizationID).Scan(&status); err != nil {
		return domain.Sale{}, fmt.Errorf("organization: %w", err)
	}
	if status != string(organizationdomain.Ready) {
		return domain.Sale{}, domain.ErrOrganizationNotReady
	}
	if strings.HasPrefix(sale.Voucher.DocumentType, "NC") || strings.HasPrefix(sale.Voucher.DocumentType, "ND") {
		var recipientRef, currency, sourceStatus, sourceDocumentType string
		var sourcePointOfSale, sourceVoucherNumber int
		var sourceFiscalSnapshot []byte
		err := tx.QueryRow(ctx, `
			SELECT recipient_ref,currency,status,point_of_sale,document_type,voucher_number,fiscal_snapshot
			FROM app.sales WHERE org_id=$1 AND id=$2`, sale.OrganizationID, sale.SourceDocumentID).
			Scan(
				&recipientRef, &currency, &sourceStatus, &sourcePointOfSale,
				&sourceDocumentType, &sourceVoucherNumber, &sourceFiscalSnapshot,
			)
		if err != nil || recipientRef != sale.RecipientRef || currency != sale.Total.Currency ||
			(sourceStatus != "posted" && sourceStatus != "partially_paid" && sourceStatus != "paid") ||
			(sourceDocumentType != "FA" && sourceDocumentType != "FB" && sourceDocumentType != "FC") {
			return domain.Sale{}, fmt.Errorf("INVALID_SOURCE_DOCUMENT")
		}
		sale.FiscalSnapshot, err = attachAssociatedVoucher(
			sale.FiscalSnapshot,
			domain.VoucherReference{
				PointOfSale: sourcePointOfSale, DocumentType: sourceDocumentType,
				VoucherNumber: sourceVoucherNumber,
			},
			sourceFiscalSnapshot,
		)
		if err != nil {
			return domain.Sale{}, fmt.Errorf("INVALID_SOURCE_DOCUMENT")
		}
	}
	if sale.Voucher.VoucherNumber < 1 {
		err := tx.QueryRow(ctx, `
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
		_, err := tx.Exec(ctx, `
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
INSERT INTO app.sales (
	id,org_id,recipient_ref,point_of_sale,document_type,voucher_number,
	fiscal_environment,amount,currency,status,snapshot_digest,credential_ref,
	fiscal_snapshot,request_id,actor_ref,source_version,correlation_id,
	source_document_id,created_at,updated_at
) VALUES (
	$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$19
)`,
		sale.ID, sale.OrganizationID, sale.RecipientRef, sale.Voucher.PointOfSale,
		sale.Voucher.DocumentType, sale.Voucher.VoucherNumber, sale.FiscalEnvironment,
		sale.Total.Amount, sale.Total.Currency, sale.Status, sale.SnapshotDigest,
		credentialRef, sale.FiscalSnapshot, sale.Origin.RequestID, sale.Origin.ActorRef,
		sale.Origin.SourceVersion, sale.CorrelationID, nullableText(sale.SourceDocumentID), now,
	)
	if err != nil {
		return domain.Sale{}, err
	}
	payload, err := json.Marshal(map[string]string{"sale_id": sale.ID, "credential_ref": credentialRef})
	if err != nil {
		return domain.Sale{}, err
	}
	digest := sha256.Sum256(payload)
	idempotencyKey := repositoryIdempotencyKey(
		sale.OrganizationID, "fiscal.authorize", sale.ID, sale.Origin.SourceVersion,
	)
	_, err = tx.Exec(ctx, `
INSERT INTO app.outbox (
	id,org_id,topic,payload,payload_hash,idempotency_key,
	request_id,actor_ref,source_version,snapshot_digest,
	correlation_id,available_at,created_at
) VALUES (
	$1,$2,'FiscalAuthorizationRequested',$3,$4,$5,$6,$7,$8,$9,$10,$11,$11
)`,
		uuid.New(), sale.OrganizationID, payload, hex.EncodeToString(digest[:]),
		idempotencyKey, sale.Origin.RequestID, sale.Origin.ActorRef,
		sale.Origin.SourceVersion, sale.SnapshotDigest, sale.CorrelationID, now,
	)
	if err != nil {
		return domain.Sale{}, err
	}
	sale.CreatedAt, sale.UpdatedAt = now, now
	return sale, nil
}

func attachAssociatedVoucher(
	noteSnapshot []byte,
	sourceVoucher domain.VoucherReference,
	sourceSnapshot []byte,
) ([]byte, error) {
	var note map[string]any
	if len(noteSnapshot) == 0 || json.Unmarshal(noteSnapshot, &note) != nil {
		return nil, fmt.Errorf("invalid note fiscal snapshot")
	}
	var source struct {
		IssueDate string `json:"issue_date"`
	}
	if len(sourceSnapshot) == 0 || json.Unmarshal(sourceSnapshot, &source) != nil ||
		source.IssueDate == "" || sourceVoucher.PointOfSale < 1 ||
		sourceVoucher.DocumentType == "" || sourceVoucher.VoucherNumber < 1 {
		return nil, fmt.Errorf("invalid source fiscal snapshot")
	}
	note["associated_voucher"] = map[string]any{
		"point_of_sale":  sourceVoucher.PointOfSale,
		"document_type":  sourceVoucher.DocumentType,
		"voucher_number": sourceVoucher.VoucherNumber,
		"issue_date":     source.IssueDate,
	}
	return json.Marshal(note)
}

func (s *Store) CreateAccountingReversal(ctx context.Context, value domain.AccountingReversal) (domain.AccountingReversal, error) {
	tx, err := beginTenantTransaction(ctx, s, value.OrganizationID)
	if err != nil {
		return domain.AccountingReversal{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := s.createAccountingReversalTx(ctx, tx, value)
	if err != nil {
		return domain.AccountingReversal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AccountingReversal{}, err
	}
	return result, nil
}

func (s *Store) CreateAccountingReversalIdempotent(ctx context.Context, command domain.IdempotencyCommand, value domain.AccountingReversal) (domain.AccountingReversal, error) {
	value.Origin = originFromIdempotencyCommand(value.Origin, command)
	value.CorrelationID = value.Origin.CorrelationID
	return executeIdempotent(ctx, s, command, func(tx pgx.Tx) (domain.AccountingReversal, error) {
		return s.createAccountingReversalTx(ctx, tx, value)
	})
}

func (s *Store) createAccountingReversalTx(ctx context.Context, tx pgx.Tx, value domain.AccountingReversal) (domain.AccountingReversal, error) {
	if value.ID == "" || value.OrganizationID == "" || value.DocumentID == "" ||
		value.EffectiveAt.IsZero() || strings.TrimSpace(value.Reason) == "" ||
		(value.DocumentKind != "purchase" && value.DocumentKind != "payment") {
		return domain.AccountingReversal{}, fmt.Errorf("VALIDATION_ERROR")
	}
	value.Origin = normalizeOrigin(
		value.Origin, value.CorrelationID, domain.OperationCreateAccountingReversal, value.ID,
	)
	value.CorrelationID = value.Origin.CorrelationID
	value.EffectiveAt = value.EffectiveAt.UTC().Truncate(time.Microsecond)
	if value.SnapshotDigest == "" {
		value.SnapshotDigest = reversalSnapshotDigest(value)
	}
	var status, journalEntryID string
	var err error
	switch value.DocumentKind {
	case "purchase":
		err = tx.QueryRow(ctx, `SELECT status,COALESCE(journal_entry_id,'') FROM app.purchases WHERE org_id=$1 AND id=$2 FOR UPDATE`, value.OrganizationID, value.DocumentID).Scan(&status, &journalEntryID)
	case "payment":
		err = tx.QueryRow(ctx, `SELECT status,COALESCE(journal_entry_id,'') FROM app.payments WHERE org_id=$1 AND id=$2 FOR UPDATE`, value.OrganizationID, value.DocumentID).Scan(&status, &journalEntryID)
	}
	if err != nil || journalEntryID == "" || (status != "posted" && status != "partially_paid" && status != "paid") {
		return domain.AccountingReversal{}, fmt.Errorf("DOCUMENT_NOT_REVERSIBLE")
	}
	value.OriginalJournalEntryID, value.Status = journalEntryID, "requested"
	now := s.Now().UTC()
	tag, err := tx.Exec(ctx, `
		INSERT INTO app.accounting_reversals
			(id,org_id,document_kind,document_id,original_journal_entry_id,effective_at,
			 reason,status,snapshot_digest,request_id,actor_ref,source_version,
			 correlation_id,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'requested',$8,$9,$10,$11,$12,$13,$13)
		ON CONFLICT (org_id,document_kind,document_id) DO NOTHING`,
		value.ID, value.OrganizationID, value.DocumentKind, value.DocumentID, value.OriginalJournalEntryID,
		value.EffectiveAt, value.Reason, value.SnapshotDigest,
		value.Origin.RequestID, value.Origin.ActorRef, value.Origin.SourceVersion,
		value.CorrelationID, now)
	if err != nil {
		return domain.AccountingReversal{}, err
	}
	if tag.RowsAffected() == 0 {
		var existing domain.AccountingReversal
		err = tx.QueryRow(ctx, `
			SELECT id,org_id,document_kind,document_id,original_journal_entry_id,effective_at,
			       reason,status,COALESCE(reversal_journal_entry_id,''),snapshot_digest,
			       request_id,actor_ref,source_version,correlation_id
			FROM app.accounting_reversals
			WHERE org_id=$1 AND document_kind=$2 AND document_id=$3`,
			value.OrganizationID, value.DocumentKind, value.DocumentID).
			Scan(&existing.ID, &existing.OrganizationID, &existing.DocumentKind, &existing.DocumentID,
				&existing.OriginalJournalEntryID, &existing.EffectiveAt, &existing.Reason, &existing.Status,
				&existing.ReversalJournalEntryID, &existing.SnapshotDigest,
				&existing.Origin.RequestID, &existing.Origin.ActorRef,
				&existing.Origin.SourceVersion, &existing.CorrelationID)
		if err != nil {
			return domain.AccountingReversal{}, err
		}
		existing.Origin.CorrelationID = existing.CorrelationID
		if existing.ID != value.ID || !existing.EffectiveAt.Equal(value.EffectiveAt) || existing.Reason != value.Reason {
			return domain.AccountingReversal{}, ErrIdempotencyConflict
		}
		return existing, nil
	}
	payload, _ := json.Marshal(map[string]string{"reversal_id": value.ID})
	digest := sha256.Sum256(payload)
	idempotencyKey := repositoryIdempotencyKey(
		value.OrganizationID, "accounting.reverse", value.ID, value.Origin.SourceVersion,
	)
	if _, err = tx.Exec(ctx, `
		INSERT INTO app.outbox
			(id,org_id,topic,payload,payload_hash,idempotency_key,
			 request_id,actor_ref,source_version,snapshot_digest,
			 correlation_id,available_at,created_at)
		VALUES ($1,$2,'AccountingReversalRequested',$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)
		ON CONFLICT (org_id,topic,idempotency_key) DO NOTHING`,
		uuid.New(), value.OrganizationID, payload, hex.EncodeToString(digest[:]), idempotencyKey,
		value.Origin.RequestID, value.Origin.ActorRef, value.Origin.SourceVersion,
		value.SnapshotDigest, value.CorrelationID, now); err != nil {
		return domain.AccountingReversal{}, err
	}
	return value, nil
}

func reversalSnapshotDigest(value domain.AccountingReversal) string {
	body, _ := json.Marshal(struct {
		ID, DocumentKind, DocumentID, Reason string
		EffectiveAt                          time.Time
	}{
		value.ID, value.DocumentKind, value.DocumentID, value.Reason, value.EffectiveAt,
	})
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func (s *Store) Lease(ctx context.Context, limit int, duration time.Duration) ([]domain.Event, error) {
	return s.lease(ctx, nil, true, limit, duration)
}

func (s *Store) LeaseTopics(
	ctx context.Context,
	topics []string,
	limit int,
	duration time.Duration,
) ([]domain.Event, error) {
	if len(topics) == 0 {
		return nil, nil
	}
	return s.lease(ctx, topics, false, limit, duration)
}

func (s *Store) lease(
	ctx context.Context,
	topics []string,
	allTopics bool,
	limit int,
	duration time.Duration,
) ([]domain.Event, error) {
	if limit < 1 || duration <= 0 {
		return nil, nil
	}
	organizations, err := s.organizationIDs(ctx)
	if err != nil {
		return nil, err
	}
	now := s.Now().UTC()
	token := uuid.NewString()
	events := []domain.Event{}
	for len(events) < limit {
		leasedThisRound := 0
		for _, organizationID := range organizations {
			if len(events) >= limit {
				break
			}
			tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
			if err != nil {
				return nil, err
			}
			if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
				_ = tx.Rollback(ctx)
				return nil, err
			}
			var event domain.Event
			err = tx.QueryRow(ctx, `
WITH candidate AS (
 SELECT id FROM app.outbox
 WHERE org_id=$1 AND published_at IS NULL AND available_at <= $2
   AND (lease_expires_at IS NULL OR lease_expires_at <= $2)
   AND ($5 OR topic=ANY($6::text[]))
   AND (
     topic <> 'FiscalAuthorizationRequested'
     OR EXISTS (
       SELECT 1 FROM app.organization_feature_flags features
       WHERE features.org_id=$1 AND features.fiscal_real_enabled
     )
   )
 ORDER BY available_at, created_at FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE app.outbox o SET lease_token=$3, lease_expires_at=$4, attempts=o.attempts+1
FROM candidate c WHERE o.id=c.id
RETURNING o.id,o.org_id,o.topic,o.payload,o.payload_hash,o.idempotency_key,
          o.request_id,o.actor_ref,o.source_version,o.snapshot_digest,
          o.correlation_id,o.available_at,o.attempts,o.lease_token,o.lease_expires_at`,
				organizationID, now, token, now.Add(duration), allTopics, topics).
				Scan(
					&event.ID, &event.OrganizationID, &event.Topic, &event.Payload,
					&event.PayloadHash, &event.IdempotencyKey, &event.RequestID,
					&event.ActorRef, &event.SourceVersion, &event.SnapshotDigest,
					&event.CorrelationID, &event.AvailableAt, &event.Attempts,
					&event.LeaseToken, &event.LeaseExpiresAt,
				)
			if errors.Is(err, pgx.ErrNoRows) {
				_ = tx.Rollback(ctx)
				continue
			}
			if err != nil {
				_ = tx.Rollback(ctx)
				return nil, err
			}
			if err = tx.Commit(ctx); err != nil {
				return nil, err
			}
			events = append(events, event)
			leasedThisRound++
		}
		if leasedThisRound == 0 {
			break
		}
	}
	return events, nil
}

func (s *Store) organizationIDs(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id FROM app.organizations WHERE status <> 'suspended' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var organizationID string
		if err := rows.Scan(&organizationID); err != nil {
			return nil, err
		}
		result = append(result, organizationID)
	}
	return result, rows.Err()
}

func (s *Store) MarkPublished(ctx context.Context, event domain.Event) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", event.OrganizationID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE app.outbox SET published_at=$1, lease_token=NULL, lease_expires_at=NULL WHERE id=$2 AND lease_token=$3`, s.Now().UTC(), event.ID, event.LeaseToken)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrLeaseLost
	}
	return tx.Commit(ctx)
}

// DeadLetter atomically preserves an exhausted event for operator replay and
// removes it from the active relay. It stores a stable code, never the raw
// downstream error, so operational data cannot leak tokens or PII.
func (s *Store) DeadLetter(ctx context.Context, event domain.Event, failureCode string) error {
	if strings.TrimSpace(failureCode) == "" {
		failureCode = "DELIVERY_FAILED"
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", event.OrganizationID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `INSERT INTO app.outbox_dead_letters(
	id,org_id,topic,payload,payload_hash,idempotency_key,
	request_id,actor_ref,source_version,snapshot_digest,
	correlation_id,attempts,failure_code,failed_at
)
SELECT id,org_id,topic,payload,payload_hash,idempotency_key,
       request_id,actor_ref,source_version,snapshot_digest,
       correlation_id,attempts,$1,$2
FROM app.outbox WHERE id=$3 AND lease_token=$4`,
		failureCode, s.Now().UTC(), event.ID, event.LeaseToken)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrLeaseLost
	}
	result, err = tx.Exec(ctx, `DELETE FROM app.outbox WHERE id=$1 AND lease_token=$2`, event.ID, event.LeaseToken)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrLeaseLost
	}
	return tx.Commit(ctx)
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
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", event.OrganizationID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE app.outbox SET available_at=$1, lease_token=NULL, lease_expires_at=NULL WHERE id=$2 AND lease_token=$3`, s.Now().UTC().Add(backoff+time.Duration(jitterMillis)*time.Millisecond), event.ID, event.LeaseToken)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrLeaseLost
	}
	return tx.Commit(ctx)
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
