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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ListUncertainSales(ctx context.Context, limit int) ([]domain.PendingFiscal, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,org_id,recipient_ref,point_of_sale,document_type,voucher_number,amount::text,currency,snapshot_digest,credential_ref,COALESCE(fiscal_snapshot,'{}'::jsonb),COALESCE(source_document_id,''),correlation_id,created_at,updated_at FROM app.sales WHERE status='fiscal_uncertain' ORDER BY updated_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.PendingFiscal
	for rows.Next() {
		var value domain.PendingFiscal
		if err := rows.Scan(&value.Sale.ID, &value.Sale.OrganizationID, &value.Sale.RecipientRef, &value.Sale.Voucher.PointOfSale, &value.Sale.Voucher.DocumentType, &value.Sale.Voucher.VoucherNumber, &value.Sale.Total.Amount, &value.Sale.Total.Currency, &value.Sale.SnapshotDigest, &value.CredentialRef, &value.Sale.FiscalSnapshot, &value.Sale.SourceDocumentID, &value.Sale.CorrelationID, &value.Sale.CreatedAt, &value.Sale.UpdatedAt); err != nil {
			return nil, err
		}
		value.Sale.Status = domain.SaleFiscalUncertain
		result = append(result, value)
	}
	return result, rows.Err()
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
	err = tx.QueryRow(ctx, `SELECT id,org_id,supplier_ref,external_document_ref,amount::text,currency,status,COALESCE(source_document_id,''),COALESCE(journal_entry_id,''),COALESCE(open_item_id,''),snapshot_digest,correlation_id,created_at FROM app.purchases WHERE id=$1`, purchaseID).Scan(&p.ID, &p.OrganizationID, &p.SupplierRef, &p.ExternalDocumentRef, &p.Total.Amount, &p.Total.Currency, &p.Status, &p.SourceDocumentID, &p.JournalEntryID, &p.OpenItemID, &p.SnapshotDigest, &p.CorrelationID, &p.CreatedAt)
	if err != nil {
		return domain.Purchase{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Purchase{}, err
	}
	return p, nil
}
func (s *Store) MarkPurchasePosted(ctx context.Context, organizationID, purchaseID, journalEntryID, openItemID string) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE app.purchases SET status='posted',journal_entry_id=$1,open_item_id=$2,updated_at=$3 WHERE id=$4 AND status='confirmed'`, journalEntryID, openItemID, s.Now().UTC(), purchaseID)
	if err != nil {
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
	err = tx.QueryRow(ctx, `SELECT id,org_id,direction,party_ref,amount::text,currency,status,COALESCE(journal_entry_id,''),COALESCE(open_item_id,''),correlation_id,created_at FROM app.payments WHERE id=$1`, id).Scan(&p.ID, &p.OrganizationID, &p.Direction, &p.PartyRef, &p.Total.Amount, &p.Total.Currency, &p.Status, &p.JournalEntryID, &p.OpenItemID, &p.CorrelationID, &p.CreatedAt)
	if err != nil {
		return domain.Payment{}, err
	}
	return p, tx.Commit(ctx)
}
func (s *Store) MarkPaymentPosted(ctx context.Context, org, id, journal, openItemID string) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", org); err != nil {
		return err
	}
	now := s.Now().UTC()
	updated, err := tx.Exec(ctx, `UPDATE app.payments SET status='posted',journal_entry_id=$1,open_item_id=$2,updated_at=$3 WHERE id=$4 AND status='confirmed'`, journal, openItemID, now, id)
	if err != nil {
		return err
	}
	if updated.RowsAffected() == 1 {
		rows, err := tx.Query(ctx, `
			SELECT id,document_kind,document_id,amount::text,currency
			FROM app.open_item_applications
			WHERE payment_id=$1 AND status='pending'
			ORDER BY id`, id)
		if err != nil {
			return err
		}
		type application struct{ id, kind, documentID, amount, currency string }
		var values []application
		for rows.Next() {
			var value application
			if err := rows.Scan(&value.id, &value.kind, &value.documentID, &value.amount, &value.currency); err != nil {
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
			switch value.kind {
			case "sale":
				err = tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.sales WHERE id=$1`, value.documentID).Scan(&documentOpenItemID)
			case "purchase":
				err = tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.purchases WHERE id=$1`, value.documentID).Scan(&documentOpenItemID)
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
			if value.kind == "purchase" {
				debitID, creditID = openItemID, documentOpenItemID
			}
			commandID := commandUUID("payment-application", value.id)
			if err := insertApplicationCommand(ctx, tx, domain.PendingAccountingApplication{
				ID: commandID, OrganizationID: org, SourceKind: "payment_application", SourceID: value.id,
				DebitOpenItemID: debitID, CreditOpenItemID: creditID,
				Amount: domain.Money{Amount: value.amount, Currency: value.currency}, Status: "pending",
				CorrelationID: "payment-application:" + value.id,
			}, now); err != nil {
				return err
			}
		}
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
	err = tx.QueryRow(ctx, `SELECT id,org_id,recipient_ref,point_of_sale,document_type,voucher_number,fiscal_environment,amount::text,currency,status,snapshot_digest,COALESCE(cae,''),COALESCE(journal_entry_id,''),COALESCE(open_item_id,''),COALESCE(source_document_id,''),COALESCE(fiscal_snapshot,'{}'::jsonb),correlation_id,created_at,updated_at FROM app.sales WHERE id=$1`, saleID).Scan(&sale.ID, &sale.OrganizationID, &sale.RecipientRef, &sale.Voucher.PointOfSale, &sale.Voucher.DocumentType, &sale.Voucher.VoucherNumber, &sale.FiscalEnvironment, &sale.Total.Amount, &sale.Total.Currency, &sale.Status, &sale.SnapshotDigest, &sale.CAE, &sale.JournalEntryID, &sale.OpenItemID, &sale.SourceDocumentID, &sale.FiscalSnapshot, &sale.CorrelationID, &sale.CreatedAt, &sale.UpdatedAt)
	if err != nil {
		return domain.Sale{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Sale{}, err
	}
	return sale, nil
}

func (s *Store) ApplyFiscalResult(ctx context.Context, sale domain.Sale, result domain.FiscalResult) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", sale.OrganizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	switch result.Status {
	case "authorized":
		updated, err := tx.Exec(ctx, `UPDATE app.sales SET status=$1,cae=$2,updated_at=$3 WHERE id=$4 AND status IN ('fiscal_pending','fiscal_uncertain')`, domain.SaleAuthorizedPendingPosting, result.CAE, now, sale.ID)
		if err != nil {
			return err
		}
		if updated.RowsAffected() == 1 {
			payload, _ := json.Marshal(map[string]string{"sale_id": sale.ID})
			if _, err = tx.Exec(ctx, `INSERT INTO app.outbox (id,org_id,topic,payload,payload_hash,idempotency_key,correlation_id,available_at,created_at)
VALUES ($1,$2,'AccountingPostingRequested',$3,$4,$5,$6,$7,$7)
ON CONFLICT (org_id,topic,idempotency_key) DO NOTHING`, uuid.New(), sale.OrganizationID, payload, sale.SnapshotDigest, sale.ID+":1", sale.CorrelationID, now); err != nil {
				return err
			}
		}
	case "uncertain":
		_, err = tx.Exec(ctx, `UPDATE app.sales SET status='fiscal_uncertain',updated_at=$1 WHERE id=$2 AND status='fiscal_pending'`, now, sale.ID)
		if err != nil {
			return err
		}
	case "rejected", "not_found":
		_, err = tx.Exec(ctx, `UPDATE app.sales SET status='fiscal_rejected',updated_at=$1 WHERE id=$2 AND status IN ('fiscal_pending','fiscal_uncertain')`, now, sale.ID)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unexpected fiscal status %q", result.Status)
	}
	return tx.Commit(ctx)
}

func (s *Store) MarkSalePosted(ctx context.Context, organizationID, saleID, journalEntryID, openItemID string) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	updated, err := tx.Exec(ctx, `UPDATE app.sales SET status='posted',journal_entry_id=$1,open_item_id=$2,updated_at=$3 WHERE id=$4 AND status='authorized_pending_posting'`, journalEntryID, openItemID, now, saleID)
	if err != nil {
		return err
	}
	if updated.RowsAffected() == 1 {
		var documentType, sourceDocumentID, amount, currency, correlationID string
		if err := tx.QueryRow(ctx, `SELECT document_type,COALESCE(source_document_id,''),amount::text,currency,correlation_id FROM app.sales WHERE id=$1`, saleID).
			Scan(&documentType, &sourceDocumentID, &amount, &currency, &correlationID); err != nil {
			return err
		}
		if strings.HasPrefix(documentType, "NC") {
			var sourceOpenItemID string
			if err := tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.sales WHERE id=$1`, sourceDocumentID).Scan(&sourceOpenItemID); err != nil {
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
			}, now); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func insertApplicationCommand(ctx context.Context, tx pgx.Tx, value domain.PendingAccountingApplication, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO app.accounting_application_commands
			(id,org_id,source_kind,source_id,debit_open_item_id,credit_open_item_id,amount,currency,status,correlation_id,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending',$9,$10,$10)
		ON CONFLICT (org_id,source_kind,source_id) DO NOTHING`,
		value.ID, value.OrganizationID, value.SourceKind, value.SourceID, value.DebitOpenItemID,
		value.CreditOpenItemID, value.Amount.Amount, value.Amount.Currency, value.CorrelationID, now)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"application_id": value.ID})
	digest := sha256.Sum256(payload)
	_, err = tx.Exec(ctx, `
		INSERT INTO app.outbox
			(id,org_id,topic,payload,payload_hash,idempotency_key,correlation_id,available_at,created_at)
		VALUES ($1,$2,'OpenItemApplicationRequested',$3,$4,$5,$6,$7,$7)
		ON CONFLICT (org_id,topic,idempotency_key) DO NOTHING`,
		uuid.New(), value.OrganizationID, payload, hex.EncodeToString(digest[:]), value.ID+":1", value.CorrelationID, now)
	return err
}

func commandUUID(operation, sourceID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("pymes-v3:"+operation+":"+sourceID)).String()
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
		       amount::text,currency,status,COALESCE(accounting_application_id,''),correlation_id
		FROM app.accounting_application_commands WHERE id=$1`, id).
		Scan(&value.ID, &value.OrganizationID, &value.SourceKind, &value.SourceID,
			&value.DebitOpenItemID, &value.CreditOpenItemID, &value.Amount.Amount,
			&value.Amount.Currency, &value.Status, &value.ApplicationID, &value.CorrelationID)
	if err != nil {
		return domain.PendingAccountingApplication{}, err
	}
	return value, tx.Commit(ctx)
}

func (s *Store) MarkAccountingApplicationApplied(ctx context.Context, organizationID, id, accountingApplicationID string) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	var sourceKind, sourceID, debitOpenItemID, creditOpenItemID string
	err = tx.QueryRow(ctx, `
		UPDATE app.accounting_application_commands
		SET status='applied',accounting_application_id=$2,updated_at=$3
		WHERE id=$1 AND status IN ('pending','applied')
		RETURNING source_kind,source_id,debit_open_item_id,credit_open_item_id`, id, accountingApplicationID, now).
		Scan(&sourceKind, &sourceID, &debitOpenItemID, &creditOpenItemID)
	if err != nil {
		return err
	}
	if sourceKind == "payment_application" {
		if _, err := tx.Exec(ctx, `
			UPDATE app.open_item_applications
			SET status='applied',accounting_application_id=$2
			WHERE id=$1 AND status IN ('pending','applied')`, sourceID, accountingApplicationID); err != nil {
			return err
		}
	}
	if err := updateDocumentSettlement(ctx, tx, debitOpenItemID, creditOpenItemID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func updateDocumentSettlement(ctx context.Context, tx pgx.Tx, debitOpenItemID, creditOpenItemID string, now time.Time) error {
	var documentKind, documentID, amount, documentOpenItemID string
	err := tx.QueryRow(ctx, `
		SELECT document_kind,id,amount,open_item_id FROM (
			SELECT 'sale' AS document_kind,id,amount::text AS amount,open_item_id
			FROM app.sales WHERE open_item_id IN ($1,$2)
			UNION ALL
			SELECT 'purchase',id,amount::text,open_item_id
			FROM app.purchases WHERE open_item_id IN ($1,$2)
		) documents
		ORDER BY CASE WHEN open_item_id=$1 THEN 0 ELSE 1 END
		LIMIT 1`, debitOpenItemID, creditOpenItemID).Scan(&documentKind, &documentID, &amount, &documentOpenItemID)
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
		WHERE (debit_open_item_id=$1 OR credit_open_item_id=$1)
		  AND status='applied'`, documentOpenItemID).Scan(&applied); err != nil {
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
	_, err = tx.Exec(ctx, `UPDATE `+table+` SET status=$1,updated_at=$2 WHERE id=$3 AND status <> 'reversed'`, status, now, documentID)
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
		err = tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.sales WHERE id=$1`, documentID).Scan(&openItemID)
	case "purchase":
		err = tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.purchases WHERE id=$1`, documentID).Scan(&openItemID)
	case "payment":
		err = tx.QueryRow(ctx, `SELECT COALESCE(open_item_id,'') FROM app.payments WHERE id=$1`, documentID).Scan(&openItemID)
	default:
		err = fmt.Errorf("VALIDATION_ERROR")
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT id,org_id,source_kind,source_id,debit_open_item_id,credit_open_item_id,
		       amount::text,currency,status,COALESCE(accounting_application_id,''),correlation_id
		FROM app.accounting_application_commands
		WHERE status='applied' AND (debit_open_item_id=$1 OR credit_open_item_id=$1)
		ORDER BY id`, openItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.PendingAccountingApplication
	for rows.Next() {
		var value domain.PendingAccountingApplication
		if err := rows.Scan(&value.ID, &value.OrganizationID, &value.SourceKind, &value.SourceID,
			&value.DebitOpenItemID, &value.CreditOpenItemID, &value.Amount.Amount,
			&value.Amount.Currency, &value.Status, &value.ApplicationID, &value.CorrelationID); err != nil {
			return nil, err
		}
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

func (s *Store) MarkAccountingApplicationReversed(ctx context.Context, organizationID, id string) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", organizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	var sourceKind, sourceID, debitOpenItemID, creditOpenItemID string
	err = tx.QueryRow(ctx, `
		UPDATE app.accounting_application_commands
		SET status='reversed',updated_at=$2
		WHERE id=$1 AND status IN ('applied','reversed')
		RETURNING source_kind,source_id,debit_open_item_id,credit_open_item_id`, id, now).
		Scan(&sourceKind, &sourceID, &debitOpenItemID, &creditOpenItemID)
	if err != nil {
		return err
	}
	if sourceKind == "payment_application" {
		if _, err := tx.Exec(ctx, `
			UPDATE app.open_item_applications
			SET status='reversed',reversed_at=$2
			WHERE id=$1 AND status IN ('applied','reversed')`, sourceID, now); err != nil {
			return err
		}
	}
	if err := updateDocumentSettlement(ctx, tx, debitOpenItemID, creditOpenItemID, now); err != nil {
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
		       reason,status,COALESCE(reversal_journal_entry_id,''),correlation_id
		FROM app.accounting_reversals WHERE id=$1`, id).
		Scan(&value.ID, &value.OrganizationID, &value.DocumentKind, &value.DocumentID,
			&value.OriginalJournalEntryID, &value.EffectiveAt, &value.Reason, &value.Status,
			&value.ReversalJournalEntryID, &value.CorrelationID)
	if err != nil {
		return domain.AccountingReversal{}, err
	}
	return value, tx.Commit(ctx)
}

func (s *Store) MarkAccountingReversalCompleted(ctx context.Context, value domain.AccountingReversal, reversalJournalEntryID string) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.org_id',$1,true)", value.OrganizationID); err != nil {
		return err
	}
	now := s.Now().UTC()
	table := map[string]string{"sale": "app.sales", "purchase": "app.purchases", "payment": "app.payments"}[value.DocumentKind]
	if table == "" {
		return fmt.Errorf("VALIDATION_ERROR")
	}
	if _, err := tx.Exec(ctx, `UPDATE `+table+` SET status='reversed',updated_at=$1 WHERE id=$2`, now, value.DocumentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.accounting_reversals
		SET status='reversed',reversal_journal_entry_id=$2,updated_at=$3
		WHERE id=$1 AND status IN ('requested','reversed')`, value.ID, reversalJournalEntryID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func normalizeDocumentKind(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
