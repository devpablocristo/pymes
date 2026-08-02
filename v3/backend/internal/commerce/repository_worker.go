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

	repositoryhelpers "github.com/devpablocristo/pymes/v3/backend/internal/commerce/repository/helpers"
	repositorymodels "github.com/devpablocristo/pymes/v3/backend/internal/commerce/repository/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/commerce/usecases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ListUncertainSales(ctx context.Context, limit int) ([]domain.PendingFiscal, error) {
	if limit < 1 {
		return nil, nil
	}
	organizations, err := s.organizationIDs(ctx)
	if err != nil {
		return nil, err
	}
	var result []domain.PendingFiscal
	for _, organizationID := range organizations {
		if len(result) >= limit {
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
		rows, err := tx.Query(ctx, `
				SELECT id,org_id,recipient_ref,point_of_sale,document_type,voucher_number,
				       amount::text,currency,snapshot_digest,credential_ref,
				       COALESCE(fiscal_snapshot,'{}'::jsonb),COALESCE(source_document_id,''),
				       request_id,actor_ref,source_version,correlation_id,created_at,updated_at
				FROM app.sales
				WHERE status='fiscal_uncertain'
				  AND EXISTS (
				    SELECT 1 FROM app.organization_feature_flags features
				    WHERE features.org_id=$2 AND features.fiscal_real_enabled
				  )
				ORDER BY updated_at
				LIMIT $1`,
			limit-len(result),
			organizationID,
		)
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
		for rows.Next() {
			var value domain.PendingFiscal
			if err := rows.Scan(
				&value.Sale.ID, &value.Sale.OrganizationID, &value.Sale.RecipientRef,
				&value.Sale.Voucher.PointOfSale, &value.Sale.Voucher.DocumentType,
				&value.Sale.Voucher.VoucherNumber, &value.Sale.Total.Amount,
				&value.Sale.Total.Currency, &value.Sale.SnapshotDigest,
				&value.CredentialRef, &value.Sale.FiscalSnapshot,
				&value.Sale.SourceDocumentID, &value.Sale.Origin.RequestID,
				&value.Sale.Origin.ActorRef, &value.Sale.Origin.SourceVersion,
				&value.Sale.CorrelationID, &value.Sale.CreatedAt, &value.Sale.UpdatedAt,
			); err != nil {
				rows.Close()
				_ = tx.Rollback(ctx)
				return nil, err
			}
			value.Sale.Origin.CorrelationID = value.Sale.CorrelationID
			value.Sale.Status = domain.SaleFiscalUncertain
			result = append(result, value)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			_ = tx.Rollback(ctx)
			return nil, err
		}
		rows.Close()
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Store) ReserveFiscalConsultAttempt(ctx context.Context, organizationID, saleID string) (int, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return 0, err
	}
	var attempt int
	if err = tx.QueryRow(ctx, `
		UPDATE app.sales
		SET fiscal_consult_attempts=fiscal_consult_attempts+1
		WHERE org_id=$1 AND id=$2 AND status='fiscal_uncertain'
		RETURNING fiscal_consult_attempts`,
		organizationID, saleID,
	).Scan(&attempt); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return attempt, nil
}

func (s *Store) GetPurchase(ctx context.Context, organizationID, purchaseID string) (domain.Purchase, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Purchase{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return domain.Purchase{}, err
	}
	var p domain.Purchase
	var vatBreakdown []byte
	err = tx.QueryRow(ctx, `
		SELECT id,org_id,supplier_ref,external_document_ref,issue_date::text,amount::text,currency,
		       net_amount::text,exempt_amount::text,vat_breakdown,
		       COALESCE(exchange_rate::text,''),status,COALESCE(source_document_id,''),
		       COALESCE(journal_entry_id,''),COALESCE(open_item_id,''),
		       COALESCE(accounting_failure_id::text,''),COALESCE(accounting_failure_code,''),
		       snapshot_digest,request_id,actor_ref,source_version,correlation_id,created_at
			FROM app.purchases WHERE org_id=$1 AND id=$2`,
		organizationID, purchaseID,
	).Scan(
		&p.ID, &p.OrganizationID, &p.SupplierRef, &p.ExternalDocumentRef,
		&p.IssueDate, &p.Total.Amount, &p.Total.Currency, &p.NetAmount, &p.ExemptAmount,
		&vatBreakdown, &p.ExchangeRate, &p.Status, &p.SourceDocumentID,
		&p.JournalEntryID, &p.OpenItemID,
		&p.AccountingFailureID, &p.AccountingFailureCode, &p.SnapshotDigest,
		&p.Origin.RequestID, &p.Origin.ActorRef, &p.Origin.SourceVersion,
		&p.CorrelationID, &p.CreatedAt,
	)
	if err != nil {
		return domain.Purchase{}, err
	}
	if err := json.Unmarshal(vatBreakdown, &p.VATBreakdown); err != nil {
		return domain.Purchase{}, fmt.Errorf("decode VAT breakdown: %w", err)
	}
	p.Origin.CorrelationID = p.CorrelationID
	if err = tx.Commit(ctx); err != nil {
		return domain.Purchase{}, err
	}
	return p, nil
}
func (s *Store) MarkPurchasePosted(ctx context.Context, purchase domain.Purchase, result domain.AccountingEvent) error {
	if result.OrganizationID != purchase.OrganizationID ||
		result.SourceVersion != repositoryhelpers.OriginSourceVersion(purchase.Origin) ||
		result.SnapshotDigest != purchase.SnapshotDigest ||
		result.CorrelationID != purchase.CorrelationID ||
		(result.Status != "posted" && result.Status != "duplicate") ||
		result.JournalEntryID == "" ||
		len(result.OpenItemIDs) != 1 {
		return fmt.Errorf("INVALID_SERVICE_RESPONSE: purchase posting")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", purchase.OrganizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	if err := recordServiceResponse(
		ctx, tx, accountingResponseMetadata("post-purchase", result), result, now,
	); err != nil {
		return err
	}
	updated, err := tx.Exec(ctx, `
		UPDATE app.purchases
		SET status='posted',journal_entry_id=$1,open_item_id=$2,
		    accounting_failure_id=NULL,accounting_failure_code=NULL,updated_at=$3
		WHERE org_id=$4 AND id=$5
		  AND status IN ('confirmed','accounting_adjustment_pending')`,
		result.JournalEntryID, result.OpenItemIDs[0], now,
		purchase.OrganizationID, purchase.ID,
	)
	if err != nil {
		return err
	}
	if updated.RowsAffected() == 0 {
		var status, journalEntryID, openItemID string
		if err := tx.QueryRow(ctx, `
			SELECT status,COALESCE(journal_entry_id,''),COALESCE(open_item_id,'')
			FROM app.purchases WHERE org_id=$1 AND id=$2`,
			purchase.OrganizationID, purchase.ID,
		).Scan(&status, &journalEntryID, &openItemID); err != nil {
			return err
		}
		if (status != "posted" && status != "partially_paid" && status != "paid") ||
			journalEntryID != result.JournalEntryID ||
			openItemID != result.OpenItemIDs[0] {
			return fmt.Errorf("STATE_TRANSITION_REJECTED: purchase posting")
		}
	}
	if err := resolveAccountingFailureTx(
		ctx, tx, purchase.OrganizationID, purchase.AccountingFailureID, now,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetPayment(ctx context.Context, org, id string) (domain.Payment, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Payment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", org); err != nil {
		return domain.Payment{}, err
	}
	var p domain.Payment
	err = tx.QueryRow(ctx, `
		SELECT id,org_id,direction,party_ref,amount::text,currency,status,
		       COALESCE(journal_entry_id,''),COALESCE(open_item_id,''),
		       COALESCE(accounting_failure_id::text,''),COALESCE(accounting_failure_code,''),
		       snapshot_digest,request_id,actor_ref,source_version,correlation_id,created_at
		FROM app.payments
		WHERE org_id=$1 AND id=$2`,
		org, id,
	).Scan(
		&p.ID, &p.OrganizationID, &p.Direction, &p.PartyRef, &p.Total.Amount,
		&p.Total.Currency, &p.Status, &p.JournalEntryID, &p.OpenItemID,
		&p.AccountingFailureID, &p.AccountingFailureCode,
		&p.SnapshotDigest, &p.Origin.RequestID, &p.Origin.ActorRef,
		&p.Origin.SourceVersion, &p.CorrelationID, &p.CreatedAt,
	)
	if err != nil {
		return domain.Payment{}, err
	}
	p.Origin.CorrelationID = p.CorrelationID
	return p, tx.Commit(ctx)
}
func (s *Store) MarkPaymentPosted(ctx context.Context, payment domain.Payment, result domain.AccountingEvent) error {
	snapshotDigest := payment.SnapshotDigest
	if snapshotDigest == "" {
		snapshotDigest = paymentSnapshotDigest(payment)
	}
	if result.OrganizationID != payment.OrganizationID ||
		result.SourceVersion != repositoryhelpers.OriginSourceVersion(payment.Origin) ||
		result.SnapshotDigest != snapshotDigest ||
		result.CorrelationID != payment.CorrelationID ||
		(result.Status != "posted" && result.Status != "duplicate") ||
		result.JournalEntryID == "" ||
		len(result.OpenItemIDs) != 1 {
		return fmt.Errorf("INVALID_SERVICE_RESPONSE: payment posting")
	}
	org, id := payment.OrganizationID, payment.ID
	openItemID := result.OpenItemIDs[0]
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", org); err != nil {
		return err
	}
	now := s.Now().UTC()
	if err := recordServiceResponse(
		ctx, tx, accountingResponseMetadata("post-payment", result), result, now,
	); err != nil {
		return err
	}
	updated, err := tx.Exec(ctx, `
		UPDATE app.payments
		SET status='posted',journal_entry_id=$1,open_item_id=$2,
		    accounting_failure_id=NULL,accounting_failure_code=NULL,updated_at=$3
		WHERE org_id=$4 AND id=$5
		  AND status IN ('confirmed','accounting_adjustment_pending')`,
		result.JournalEntryID, openItemID, now, org, id)
	if err != nil {
		return err
	}
	if updated.RowsAffected() == 1 {
		rows, err := tx.Query(ctx, `
			SELECT id,document_kind,document_id,amount::text,currency
			FROM app.open_item_applications
			WHERE org_id=$1 AND payment_id=$2 AND status='pending'
			ORDER BY id`, org, id)
		if err != nil {
			return err
		}
		var values []repositorymodels.PendingApplication
		for rows.Next() {
			var value repositorymodels.PendingApplication
			if err := rows.Scan(
				&value.ID,
				&value.Kind,
				&value.DocumentID,
				&value.Amount,
				&value.Currency,
			); err != nil {
				rows.Close()
				return err
			}
			values = append(values, value)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, value := range values {
			var documentOpenItemID string
			switch value.Kind {
			case "sale":
				err = tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.sales WHERE org_id=$1 AND id=$2`, org, value.DocumentID).Scan(&documentOpenItemID)
			case "purchase":
				err = tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.purchases WHERE org_id=$1 AND id=$2`, org, value.DocumentID).Scan(&documentOpenItemID)
			default:
				err = fmt.Errorf("unsupported application document kind")
			}
			if err != nil || documentOpenItemID == "" || openItemID == "" {
				if err != nil {
					return err
				}
				return fmt.Errorf("OPEN_ITEM_NOT_READY")
			}
			debitID, creditID := documentOpenItemID, openItemID
			if value.Kind == "purchase" {
				debitID, creditID = openItemID, documentOpenItemID
			}
			commandID := commandUUID("payment-application", value.ID)
			if err := insertApplicationCommand(ctx, tx, domain.PendingAccountingApplication{
				ID: commandID, OrganizationID: org, SourceKind: "payment_application", SourceID: value.ID,
				DebitOpenItemID: debitID, CreditOpenItemID: creditID,
				Amount: domain.Money{Amount: value.Amount, Currency: value.Currency}, Status: "pending",
				Origin: payment.Origin, CorrelationID: payment.CorrelationID,
			}, now); err != nil {
				return err
			}
		}
	} else {
		var status, journalEntryID, storedOpenItemID string
		if err := tx.QueryRow(ctx, `
			SELECT status,COALESCE(journal_entry_id,''),COALESCE(open_item_id,'')
			FROM app.payments WHERE org_id=$1 AND id=$2`,
			org, id,
		).Scan(&status, &journalEntryID, &storedOpenItemID); err != nil {
			return err
		}
		if status != "posted" ||
			journalEntryID != result.JournalEntryID ||
			storedOpenItemID != openItemID {
			return fmt.Errorf("STATE_TRANSITION_REJECTED: payment posting")
		}
	}
	if err := resolveAccountingFailureTx(
		ctx, tx, payment.OrganizationID, payment.AccountingFailureID, now,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetSale(ctx context.Context, organizationID, saleID string) (domain.Sale, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Sale{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return domain.Sale{}, err
	}
	var sale domain.Sale
	err = tx.QueryRow(ctx, `
		SELECT id,org_id,recipient_ref,point_of_sale,document_type,voucher_number,
		       fiscal_environment,amount::text,currency,status,snapshot_digest,
		       COALESCE(cae,''),COALESCE(journal_entry_id,''),COALESCE(open_item_id,''),
		       COALESCE(accounting_failure_id::text,''),COALESCE(accounting_failure_code,''),
		       COALESCE(source_document_id,''),COALESCE(fiscal_snapshot,'{}'::jsonb),
		       request_id,actor_ref,source_version,correlation_id,created_at,updated_at
		FROM app.sales
		WHERE org_id=$1 AND id=$2`,
		organizationID, saleID,
	).Scan(
		&sale.ID, &sale.OrganizationID, &sale.RecipientRef, &sale.Voucher.PointOfSale,
		&sale.Voucher.DocumentType, &sale.Voucher.VoucherNumber, &sale.FiscalEnvironment,
		&sale.Total.Amount, &sale.Total.Currency, &sale.Status, &sale.SnapshotDigest,
		&sale.CAE, &sale.JournalEntryID, &sale.OpenItemID,
		&sale.AccountingFailureID, &sale.AccountingFailureCode, &sale.SourceDocumentID,
		&sale.FiscalSnapshot, &sale.Origin.RequestID, &sale.Origin.ActorRef,
		&sale.Origin.SourceVersion, &sale.CorrelationID, &sale.CreatedAt, &sale.UpdatedAt,
	)
	if err != nil {
		return domain.Sale{}, err
	}
	sale.Origin.CorrelationID = sale.CorrelationID
	if err = tx.Commit(ctx); err != nil {
		return domain.Sale{}, err
	}
	return sale, nil
}

func (s *Store) ApplyFiscalResult(ctx context.Context, sale domain.Sale, result domain.FiscalResult) error {
	if result.OrganizationID != sale.OrganizationID ||
		result.SourceVersion != repositoryhelpers.OriginSourceVersion(sale.Origin) ||
		result.SnapshotDigest != sale.SnapshotDigest ||
		result.CorrelationID != sale.CorrelationID {
		return fmt.Errorf("INVALID_SERVICE_RESPONSE: fiscal metadata")
	}
	switch result.Status {
	case "authorized", "uncertain", "rejected", "not_found":
	default:
		return fmt.Errorf("unexpected fiscal status %q", result.Status)
	}
	if result.Status == "authorized" && result.CAE == "" {
		return fmt.Errorf("INVALID_SERVICE_RESPONSE: authorized fiscal result without CAE")
	}
	metadata, err := fiscalResponseMetadata(result)
	if err != nil {
		return err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", sale.OrganizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	if err := recordServiceResponse(ctx, tx, metadata, result, now); err != nil {
		return err
	}
	switch result.Status {
	case "authorized":
		updated, err := tx.Exec(ctx, `UPDATE app.sales SET status=$1,cae=$2,updated_at=$3 WHERE org_id=$4 AND id=$5 AND status IN ('fiscal_pending','fiscal_uncertain')`, domain.SaleAuthorizedPendingPosting, result.CAE, now, sale.OrganizationID, sale.ID)
		if err != nil {
			return err
		}
		if updated.RowsAffected() == 1 {
			payload, _ := json.Marshal(map[string]string{"sale_id": sale.ID})
			payloadDigest := sha256.Sum256(payload)
			sourceVersion := repositoryhelpers.OriginSourceVersion(sale.Origin)
			idempotencyKey := repositoryhelpers.IdempotencyKey(
				sale.OrganizationID, "accounting.post", sale.ID, sourceVersion,
			)
			origin := repositoryhelpers.NormalizeOrigin(
				sale.Origin, sale.CorrelationID, "accounting.post", sale.ID,
			)
			if _, err = tx.Exec(ctx, `
					INSERT INTO app.outbox (
						id,org_id,topic,payload,payload_hash,idempotency_key,
						request_id,actor_ref,source_version,snapshot_digest,
						correlation_id,available_at,created_at
					) VALUES (
						$1,$2,'AccountingPostingRequested',$3,$4,$5,$6,$7,$8,$9,$10,$11,$11
					)
					ON CONFLICT (org_id,topic,idempotency_key) DO NOTHING`,
				uuid.New(), sale.OrganizationID, payload,
				hex.EncodeToString(payloadDigest[:]), idempotencyKey,
				origin.RequestID, origin.ActorRef, origin.SourceVersion,
				sale.SnapshotDigest, origin.CorrelationID, now,
			); err != nil {
				return err
			}
		} else {
			var status, cae string
			if err := tx.QueryRow(ctx, `
				SELECT status,COALESCE(cae,'') FROM app.sales
				WHERE org_id=$1 AND id=$2`,
				sale.OrganizationID, sale.ID,
			).Scan(&status, &cae); err != nil {
				return err
			}
			if (status != string(domain.SaleAuthorizedPendingPosting) &&
				status != string(domain.SalePosted) &&
				status != "partially_paid" && status != "paid") ||
				cae != result.CAE {
				return fmt.Errorf("STATE_TRANSITION_REJECTED: fiscal authorization")
			}
		}
	case "uncertain":
		updated, updateErr := tx.Exec(ctx, `UPDATE app.sales SET status='fiscal_uncertain',updated_at=$1 WHERE org_id=$2 AND id=$3 AND status='fiscal_pending'`, now, sale.OrganizationID, sale.ID)
		if updateErr != nil {
			return updateErr
		}
		if updated.RowsAffected() == 0 {
			var status string
			if err := tx.QueryRow(ctx, `SELECT status FROM app.sales WHERE org_id=$1 AND id=$2`, sale.OrganizationID, sale.ID).Scan(&status); err != nil {
				return err
			}
			if status != string(domain.SaleFiscalUncertain) {
				return fmt.Errorf("STATE_TRANSITION_REJECTED: fiscal uncertainty")
			}
		}
	case "rejected", "not_found":
		updated, updateErr := tx.Exec(ctx, `UPDATE app.sales SET status='fiscal_rejected',updated_at=$1 WHERE org_id=$2 AND id=$3 AND status IN ('fiscal_pending','fiscal_uncertain')`, now, sale.OrganizationID, sale.ID)
		if updateErr != nil {
			return updateErr
		}
		if updated.RowsAffected() == 0 {
			var status string
			if err := tx.QueryRow(ctx, `SELECT status FROM app.sales WHERE org_id=$1 AND id=$2`, sale.OrganizationID, sale.ID).Scan(&status); err != nil {
				return err
			}
			if status != string(domain.SaleFiscalRejected) {
				return fmt.Errorf("STATE_TRANSITION_REJECTED: fiscal rejection")
			}
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) MarkSalePosted(ctx context.Context, sale domain.Sale, result domain.AccountingEvent) error {
	if result.OrganizationID != sale.OrganizationID ||
		result.SourceVersion != repositoryhelpers.OriginSourceVersion(sale.Origin) ||
		result.SnapshotDigest != sale.SnapshotDigest ||
		result.CorrelationID != sale.CorrelationID ||
		(result.Status != "posted" && result.Status != "duplicate") ||
		result.JournalEntryID == "" ||
		len(result.OpenItemIDs) != 1 {
		return fmt.Errorf("INVALID_SERVICE_RESPONSE: sale posting")
	}
	organizationID, saleID := sale.OrganizationID, sale.ID
	openItemID := result.OpenItemIDs[0]
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	if err := recordServiceResponse(
		ctx, tx, accountingResponseMetadata("post-sale", result), result, now,
	); err != nil {
		return err
	}
	updated, err := tx.Exec(ctx, `
		UPDATE app.sales
		SET status='posted',journal_entry_id=$1,open_item_id=$2,
		    accounting_failure_id=NULL,accounting_failure_code=NULL,updated_at=$3
		WHERE org_id=$4 AND id=$5
		  AND status IN ('authorized_pending_posting','accounting_adjustment_pending')`,
		result.JournalEntryID, openItemID, now, organizationID, saleID)
	if err != nil {
		return err
	}
	if updated.RowsAffected() == 1 {
		var documentType, sourceDocumentID, amount, currency, correlationID string
		if err := tx.QueryRow(ctx, `SELECT document_type,COALESCE(source_document_id,''),amount::text,currency,correlation_id FROM app.sales WHERE org_id=$1 AND id=$2`, organizationID, saleID).
			Scan(&documentType, &sourceDocumentID, &amount, &currency, &correlationID); err != nil {
			return err
		}
		if strings.HasPrefix(documentType, "NC") {
			var sourceOpenItemID string
			if err := tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.sales WHERE org_id=$1 AND id=$2`, organizationID, sourceDocumentID).Scan(&sourceOpenItemID); err != nil {
				return err
			}
			if sourceOpenItemID == "" || openItemID == "" {
				return fmt.Errorf("OPEN_ITEM_NOT_READY")
			}
			commandID := commandUUID("credit-note-application", saleID)
			if err := insertApplicationCommand(ctx, tx, domain.PendingAccountingApplication{
				ID: commandID, OrganizationID: organizationID, SourceKind: "credit_note", SourceID: saleID,
				DebitOpenItemID: sourceOpenItemID, CreditOpenItemID: openItemID,
				Amount: domain.Money{Amount: amount, Currency: currency}, Status: "pending", CorrelationID: correlationID,
				Origin: sale.Origin,
			}, now); err != nil {
				return err
			}
		}
	} else {
		var status, journalEntryID, storedOpenItemID string
		if err := tx.QueryRow(ctx, `
			SELECT status,COALESCE(journal_entry_id,''),COALESCE(open_item_id,'')
			FROM app.sales WHERE org_id=$1 AND id=$2`,
			organizationID, saleID,
		).Scan(&status, &journalEntryID, &storedOpenItemID); err != nil {
			return err
		}
		if (status != string(domain.SalePosted) && status != "partially_paid" && status != "paid") ||
			journalEntryID != result.JournalEntryID ||
			storedOpenItemID != openItemID {
			return fmt.Errorf("STATE_TRANSITION_REJECTED: sale posting")
		}
	}
	if err := resolveAccountingFailureTx(
		ctx, tx, sale.OrganizationID, sale.AccountingFailureID, now,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertApplicationCommand(ctx context.Context, tx pgx.Tx, value domain.PendingAccountingApplication, now time.Time) error {
	value.Origin = repositoryhelpers.NormalizeOrigin(
		value.Origin, value.CorrelationID, "accounting.apply", value.ID,
	)
	value.CorrelationID = value.Origin.CorrelationID
	if value.SnapshotDigest == "" {
		value.SnapshotDigest = repositoryhelpers.AccountingApplicationSnapshotDigest(value)
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO app.accounting_application_commands
			(id,org_id,source_kind,source_id,debit_open_item_id,credit_open_item_id,
			 amount,currency,status,snapshot_digest,request_id,actor_ref,source_version,
			 correlation_id,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending',$9,$10,$11,$12,$13,$14,$14)
		ON CONFLICT (org_id,source_kind,source_id) DO NOTHING`,
		value.ID, value.OrganizationID, value.SourceKind, value.SourceID, value.DebitOpenItemID,
		value.CreditOpenItemID, value.Amount.Amount, value.Amount.Currency,
		value.SnapshotDigest, value.Origin.RequestID, value.Origin.ActorRef,
		value.Origin.SourceVersion, value.CorrelationID, now)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"application_id": value.ID})
	digest := sha256.Sum256(payload)
	idempotencyKey := repositoryhelpers.IdempotencyKey(
		value.OrganizationID, "accounting.apply", value.ID, value.Origin.SourceVersion,
	)
	_, err = tx.Exec(ctx, `
		INSERT INTO app.outbox
			(id,org_id,topic,payload,payload_hash,idempotency_key,
			 request_id,actor_ref,source_version,snapshot_digest,
			 correlation_id,available_at,created_at)
		VALUES ($1,$2,'OpenItemApplicationRequested',$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)
		ON CONFLICT (org_id,topic,idempotency_key) DO NOTHING`,
		uuid.New(), value.OrganizationID, payload, hex.EncodeToString(digest[:]), idempotencyKey,
		value.Origin.RequestID, value.Origin.ActorRef, value.Origin.SourceVersion,
		value.SnapshotDigest, value.CorrelationID, now)
	return err
}

func commandUUID(operation, sourceID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("pymes-v3:"+operation+":"+sourceID)).String()
}

func paymentSnapshotDigest(payment domain.Payment) string {
	digest := sha256.Sum256([]byte(
		payment.ID + ":" + payment.Total.Amount + ":" + payment.Total.Currency + ":" + payment.Direction,
	))
	return hex.EncodeToString(digest[:])
}

func (s *Store) GetAccountingApplication(ctx context.Context, organizationID, id string) (domain.PendingAccountingApplication, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.PendingAccountingApplication{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return domain.PendingAccountingApplication{}, err
	}
	var value domain.PendingAccountingApplication
	err = tx.QueryRow(ctx, `
		SELECT id,org_id,source_kind,source_id,debit_open_item_id,credit_open_item_id,
		       amount::text,currency,status,COALESCE(accounting_application_id,''),
		       COALESCE(accounting_failure_id::text,''),COALESCE(accounting_failure_code,''),
		       snapshot_digest,request_id,actor_ref,source_version,correlation_id
		FROM app.accounting_application_commands WHERE org_id=$1 AND id=$2`, organizationID, id).
		Scan(&value.ID, &value.OrganizationID, &value.SourceKind, &value.SourceID,
			&value.DebitOpenItemID, &value.CreditOpenItemID, &value.Amount.Amount,
			&value.Amount.Currency, &value.Status, &value.ApplicationID,
			&value.AccountingFailureID, &value.AccountingFailureCode,
			&value.SnapshotDigest, &value.Origin.RequestID, &value.Origin.ActorRef,
			&value.Origin.SourceVersion, &value.CorrelationID)
	if err != nil {
		return domain.PendingAccountingApplication{}, err
	}
	value.Origin.CorrelationID = value.CorrelationID
	return value, tx.Commit(ctx)
}

func (s *Store) MarkAccountingApplicationApplied(ctx context.Context, value domain.PendingAccountingApplication, result domain.AccountingEvent) error {
	if result.OrganizationID != value.OrganizationID ||
		result.SourceVersion != repositoryhelpers.OriginSourceVersion(value.Origin) ||
		result.SnapshotDigest != value.SnapshotDigest ||
		result.CorrelationID != value.CorrelationID ||
		(result.Status != "applied" && result.Status != "duplicate") ||
		result.ApplicationID == "" {
		return fmt.Errorf("INVALID_SERVICE_RESPONSE: open-item application")
	}
	organizationID, id := value.OrganizationID, value.ID
	accountingApplicationID := result.ApplicationID
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	if err := recordServiceResponse(
		ctx, tx, accountingResponseMetadata("apply-open-item", result), result, now,
	); err != nil {
		return err
	}
	var sourceKind, sourceID, debitOpenItemID, creditOpenItemID string
	err = tx.QueryRow(ctx, `
		UPDATE app.accounting_application_commands
		SET status='applied',accounting_application_id=$2,
		    accounting_failure_id=NULL,accounting_failure_code=NULL,updated_at=$3
		WHERE org_id=$1 AND id=$4
		  AND (
		    status='pending'
		    OR status='accounting_adjustment_pending'
		    OR (status='applied' AND accounting_application_id=$2)
		  )
		RETURNING source_kind,source_id,debit_open_item_id,credit_open_item_id`, organizationID, accountingApplicationID, now, id).
		Scan(&sourceKind, &sourceID, &debitOpenItemID, &creditOpenItemID)
	if err != nil {
		return err
	}
	if sourceKind == "payment_application" {
		if _, err := tx.Exec(ctx, `
			UPDATE app.open_item_applications
			SET status='applied',accounting_application_id=$2
			WHERE org_id=$1 AND id=$3 AND status IN ('pending','applied')`, organizationID, accountingApplicationID, sourceID); err != nil {
			return err
		}
	}
	if err := updateDocumentSettlement(ctx, tx, organizationID, debitOpenItemID, creditOpenItemID, now); err != nil {
		return err
	}
	if err := resolveAccountingFailureTx(
		ctx, tx, value.OrganizationID, value.AccountingFailureID, now,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func updateDocumentSettlement(ctx context.Context, tx pgx.Tx, organizationID, debitOpenItemID, creditOpenItemID string, now time.Time) error {
	var documentKind, documentID, amount, documentOpenItemID string
	err := tx.QueryRow(ctx, `
		SELECT document_kind,id,amount,open_item_id FROM (
			SELECT 'sale' AS document_kind,id,amount::text AS amount,open_item_id
			FROM app.sales WHERE org_id=$1 AND open_item_id IN ($2,$3)
			UNION ALL
			SELECT 'purchase',id,amount::text,open_item_id
			FROM app.purchases WHERE org_id=$1 AND open_item_id IN ($2,$3)
		) documents
		ORDER BY CASE WHEN open_item_id=$2 THEN 0 ELSE 1 END
		LIMIT 1`, organizationID, debitOpenItemID, creditOpenItemID).Scan(&documentKind, &documentID, &amount, &documentOpenItemID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var applied string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(sum(amount),0)::text
		FROM app.accounting_application_commands
		WHERE org_id=$1 AND (debit_open_item_id=$2 OR credit_open_item_id=$2)
		  AND status='applied'`, organizationID, documentOpenItemID).Scan(&applied); err != nil {
		return err
	}
	appliedValue, amountValue := new(big.Rat), new(big.Rat)
	if _, ok := appliedValue.SetString(applied); !ok {
		return fmt.Errorf("invalid applied amount")
	}
	if _, ok := amountValue.SetString(amount); !ok {
		return fmt.Errorf("invalid document amount")
	}
	status := "posted"
	if appliedValue.Sign() > 0 {
		status = "partially_paid"
	}
	if appliedValue.Cmp(amountValue) >= 0 {
		status = "paid"
	}
	table := "app.sales"
	if documentKind == "purchase" {
		table = "app.purchases"
	}
	_, err = tx.Exec(ctx, `UPDATE `+table+` SET status=$1,updated_at=$2 WHERE org_id=$3 AND id=$4 AND status <> 'reversed'`, status, now, organizationID, documentID)
	return err
}

func (s *Store) ListAppliedAccountingApplications(ctx context.Context, organizationID, documentKind, documentID string) ([]domain.PendingAccountingApplication, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return nil, err
	}
	var openItemID string
	switch documentKind {
	case "sale":
		err = tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.sales WHERE org_id=$1 AND id=$2`, organizationID, documentID).Scan(&openItemID)
	case "purchase":
		err = tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.purchases WHERE org_id=$1 AND id=$2`, organizationID, documentID).Scan(&openItemID)
	case "payment":
		err = tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.payments WHERE org_id=$1 AND id=$2`, organizationID, documentID).Scan(&openItemID)
	default:
		err = fmt.Errorf("VALIDATION_ERROR")
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT id,org_id,source_kind,source_id,debit_open_item_id,credit_open_item_id,
		       amount::text,currency,status,COALESCE(accounting_application_id,''),
		       snapshot_digest,request_id,actor_ref,source_version,correlation_id
		FROM app.accounting_application_commands
		WHERE org_id=$1 AND status='applied' AND (debit_open_item_id=$2 OR credit_open_item_id=$2)
		ORDER BY id`, organizationID, openItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.PendingAccountingApplication
	for rows.Next() {
		var value domain.PendingAccountingApplication
		if err := rows.Scan(&value.ID, &value.OrganizationID, &value.SourceKind, &value.SourceID,
			&value.DebitOpenItemID, &value.CreditOpenItemID, &value.Amount.Amount,
			&value.Amount.Currency, &value.Status, &value.ApplicationID,
			&value.SnapshotDigest, &value.Origin.RequestID, &value.Origin.ActorRef,
			&value.Origin.SourceVersion, &value.CorrelationID); err != nil {
			return nil, err
		}
		value.Origin.CorrelationID = value.CorrelationID
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) MarkAccountingApplicationReversed(ctx context.Context, value domain.PendingAccountingApplication, result domain.AccountingEvent) error {
	if result.OrganizationID != value.OrganizationID ||
		result.SourceVersion != repositoryhelpers.OriginSourceVersion(value.Origin) ||
		result.IdempotencyKey == "" ||
		result.SnapshotDigest == "" ||
		result.CorrelationID == "" ||
		(result.Status != "reversed" && result.Status != "duplicate") ||
		result.ApplicationID != value.ApplicationID {
		return fmt.Errorf("INVALID_SERVICE_RESPONSE: open-item application reversal")
	}
	organizationID, id := value.OrganizationID, value.ID
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	if err := recordServiceResponse(
		ctx, tx, accountingResponseMetadata("reverse-open-item-application", result), result, now,
	); err != nil {
		return err
	}
	var sourceKind, sourceID, debitOpenItemID, creditOpenItemID string
	err = tx.QueryRow(ctx, `
		UPDATE app.accounting_application_commands
		SET status='reversed',updated_at=$2
		WHERE org_id=$1 AND id=$3 AND status IN ('applied','reversed')
		RETURNING source_kind,source_id,debit_open_item_id,credit_open_item_id`, organizationID, now, id).
		Scan(&sourceKind, &sourceID, &debitOpenItemID, &creditOpenItemID)
	if err != nil {
		return err
	}
	if sourceKind == "payment_application" {
		if _, err := tx.Exec(ctx, `
			UPDATE app.open_item_applications
			SET status='reversed',reversed_at=$2
			WHERE org_id=$1 AND id=$3 AND status IN ('applied','reversed')`, organizationID, now, sourceID); err != nil {
			return err
		}
	}
	if err := updateDocumentSettlement(ctx, tx, organizationID, debitOpenItemID, creditOpenItemID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetAccountingReversal(ctx context.Context, organizationID, id string) (domain.AccountingReversal, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.AccountingReversal{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return domain.AccountingReversal{}, err
	}
	var value domain.AccountingReversal
	err = tx.QueryRow(ctx, `
		SELECT id,org_id,document_kind,document_id,original_journal_entry_id,effective_at,
		       reason,status,COALESCE(reversal_journal_entry_id,''),snapshot_digest,
		       COALESCE(accounting_failure_id::text,''),COALESCE(accounting_failure_code,''),
		       request_id,actor_ref,source_version,correlation_id
		FROM app.accounting_reversals WHERE org_id=$1 AND id=$2`, organizationID, id).
		Scan(&value.ID, &value.OrganizationID, &value.DocumentKind, &value.DocumentID,
			&value.OriginalJournalEntryID, &value.EffectiveAt, &value.Reason, &value.Status,
			&value.ReversalJournalEntryID, &value.SnapshotDigest,
			&value.AccountingFailureID, &value.AccountingFailureCode,
			&value.Origin.RequestID, &value.Origin.ActorRef,
			&value.Origin.SourceVersion, &value.CorrelationID)
	if err != nil {
		return domain.AccountingReversal{}, err
	}
	value.Origin.CorrelationID = value.CorrelationID
	return value, tx.Commit(ctx)
}

func (s *Store) MarkAccountingReversalCompleted(ctx context.Context, value domain.AccountingReversal, result domain.AccountingEvent) error {
	if result.OrganizationID != value.OrganizationID ||
		result.SourceVersion != repositoryhelpers.OriginSourceVersion(value.Origin) ||
		result.SnapshotDigest != value.SnapshotDigest ||
		result.CorrelationID != value.CorrelationID ||
		(result.Status != "reversed" && result.Status != "duplicate") ||
		result.JournalEntryID == "" {
		return fmt.Errorf("INVALID_SERVICE_RESPONSE: journal reversal")
	}
	reversalJournalEntryID := result.JournalEntryID
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", value.OrganizationID); err != nil {
		return err
	}
	var (
		storedDocumentKind, storedDocumentID      string
		storedStatus, storedJournalEntryID        string
		storedSnapshotDigest, storedCorrelationID string
	)
	if err := tx.QueryRow(ctx, `
		SELECT document_kind,document_id,status,COALESCE(reversal_journal_entry_id,''),
		       snapshot_digest,correlation_id
		FROM app.accounting_reversals
		WHERE org_id=$1 AND id=$2
		FOR UPDATE`,
		value.OrganizationID, value.ID,
	).Scan(
		&storedDocumentKind, &storedDocumentID, &storedStatus,
		&storedJournalEntryID, &storedSnapshotDigest, &storedCorrelationID,
	); err != nil {
		return err
	}
	if storedDocumentKind != value.DocumentKind ||
		storedDocumentID != value.DocumentID ||
		storedSnapshotDigest != value.SnapshotDigest ||
		storedCorrelationID != value.CorrelationID ||
		(storedStatus != "requested" &&
			storedStatus != domain.AccountingAdjustmentPending &&
			storedStatus != "reversed") ||
		(storedStatus == "reversed" && storedJournalEntryID != reversalJournalEntryID) {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: journal reversal")
	}
	now := s.Now().UTC()
	if err := recordServiceResponse(
		ctx, tx, accountingResponseMetadata("reverse-journal", result), result, now,
	); err != nil {
		return err
	}
	table := map[string]string{"sale": "app.sales", "purchase": "app.purchases", "payment": "app.payments"}[value.DocumentKind]
	if table == "" {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	documentUpdated, err := tx.Exec(ctx, `
		UPDATE `+table+`
		SET status='reversed',updated_at=$1
		WHERE org_id=$2 AND id=$3
		  AND status IN ('posted','partially_paid','paid','reversed')`,
		now, value.OrganizationID, value.DocumentID,
	)
	if err != nil {
		return err
	}
	if documentUpdated.RowsAffected() != 1 {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: reversed document")
	}
	reversalUpdated, err := tx.Exec(ctx, `
		UPDATE app.accounting_reversals
		SET status='reversed',reversal_journal_entry_id=$2,
		    accounting_failure_id=NULL,accounting_failure_code=NULL,updated_at=$3
		WHERE org_id=$1 AND id=$4
		  AND (
		    status='requested'
		    OR status='accounting_adjustment_pending'
		    OR (status='reversed' AND reversal_journal_entry_id=$2)
		  )`,
		value.OrganizationID, reversalJournalEntryID, now, value.ID,
	)
	if err != nil {
		return err
	}
	if reversalUpdated.RowsAffected() != 1 {
		return fmt.Errorf("STATE_TRANSITION_REJECTED: journal reversal")
	}
	if err := resolveAccountingFailureTx(
		ctx, tx, value.OrganizationID, value.AccountingFailureID, now,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
