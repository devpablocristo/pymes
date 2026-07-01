package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	corepostgres "github.com/devpablocristo/platform/databases/postgres/go"
	"github.com/devpablocristo/platform/http/go/pagination"
	"github.com/devpablocristo/pymes/core/backend/internal/ledger/repository/models"
	ledgerdomain "github.com/devpablocristo/pymes/core/backend/internal/ledger/usecases/domain"
)

var (
	ErrNotFound           = errors.New("ledger: not found")
	ErrAlreadyExists      = errors.New("ledger: already exists")
	ErrAccountLinkMissing = errors.New("ledger: account link missing")
	ErrUnbalanced         = errors.New("ledger: entry not balanced")
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// --- Plan de cuentas ---

func (r *Repository) ListAccounts(ctx context.Context, orgID uuid.UUID, includeArchived bool) ([]ledgerdomain.Account, error) {
	q := r.db.WithContext(ctx).Model(&models.AccountModel{}).Where("org_id = ?", orgID)
	if !includeArchived {
		q = q.Where("archived_at IS NULL")
	}
	var rows []models.AccountModel
	if err := q.Order("code ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ledgerdomain.Account, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAccountDomain(row))
	}
	return out, nil
}

func (r *Repository) GetAccount(ctx context.Context, orgID, id uuid.UUID) (ledgerdomain.Account, error) {
	var row models.AccountModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ledgerdomain.Account{}, ErrNotFound
	}
	if err != nil {
		return ledgerdomain.Account{}, err
	}
	return toAccountDomain(row), nil
}

func (r *Repository) CreateAccount(ctx context.Context, in ledgerdomain.Account) (ledgerdomain.Account, error) {
	row := models.AccountModel{
		ID:         uuid.New(),
		OrgID:      in.OrgID,
		Code:       in.Code,
		Name:       in.Name,
		Type:       in.Type,
		ParentID:   in.ParentID,
		IsPostable: in.IsPostable,
	}
	err := r.db.WithContext(ctx).Create(&row).Error
	if isUniqueViolation(err) {
		return ledgerdomain.Account{}, ErrAlreadyExists
	}
	if err != nil {
		return ledgerdomain.Account{}, err
	}
	return r.GetAccount(ctx, in.OrgID, row.ID)
}

func (r *Repository) UpdateAccount(ctx context.Context, in ledgerdomain.Account) (ledgerdomain.Account, error) {
	res := r.db.WithContext(ctx).Model(&models.AccountModel{}).
		Where("org_id = ? AND id = ? AND archived_at IS NULL", in.OrgID, in.ID).
		Updates(map[string]any{
			"name":        in.Name,
			"type":        in.Type,
			"parent_id":   in.ParentID,
			"is_postable": in.IsPostable,
		})
	if res.Error != nil {
		return ledgerdomain.Account{}, res.Error
	}
	if res.RowsAffected == 0 {
		return ledgerdomain.Account{}, ErrNotFound
	}
	return r.GetAccount(ctx, in.OrgID, in.ID)
}

func (r *Repository) ArchiveAccount(ctx context.Context, orgID, id uuid.UUID) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&models.AccountModel{}).
		Where("org_id = ? AND id = ? AND archived_at IS NULL", orgID, id).
		Update("archived_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) RestoreAccount(ctx context.Context, orgID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Model(&models.AccountModel{}).
		Where("org_id = ? AND id = ? AND archived_at IS NOT NULL", orgID, id).
		Update("archived_at", nil)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Account links (rol -> cuenta) ---

func (r *Repository) ListLinks(ctx context.Context, orgID uuid.UUID) ([]ledgerdomain.AccountLink, error) {
	type row struct {
		ID          uuid.UUID
		OrgID       uuid.UUID
		Role        string
		AccountID   uuid.UUID
		AccountCode string
		AccountName string
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("ledger_account_links AS l").
		Select("l.id, l.org_id, l.role, l.account_id, a.code AS account_code, a.name AS account_name").
		Joins("JOIN ledger_accounts a ON a.id = l.account_id").
		Where("l.org_id = ?", orgID).
		Order("l.role ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]ledgerdomain.AccountLink, 0, len(rows))
	for _, row := range rows {
		out = append(out, ledgerdomain.AccountLink{ID: row.ID, OrgID: row.OrgID, Role: row.Role, AccountID: row.AccountID, AccountCode: row.AccountCode, AccountName: row.AccountName})
	}
	return out, nil
}

func (r *Repository) SetLink(ctx context.Context, orgID uuid.UUID, role string, accountID uuid.UUID) (ledgerdomain.AccountLink, error) {
	// La cuenta debe existir, ser de la org y estar activa.
	if _, err := r.GetAccount(ctx, orgID, accountID); err != nil {
		return ledgerdomain.AccountLink{}, err
	}
	row := models.AccountLinkModel{ID: uuid.New(), OrgID: orgID, Role: role, AccountID: accountID}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "org_id"}, {Name: "role"}},
		DoUpdates: clause.AssignmentColumns([]string{"account_id", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return ledgerdomain.AccountLink{}, err
	}
	links, err := r.ListLinks(ctx, orgID)
	if err != nil {
		return ledgerdomain.AccountLink{}, err
	}
	for _, l := range links {
		if l.Role == role {
			return l, nil
		}
	}
	return ledgerdomain.AccountLink{OrgID: orgID, Role: role, AccountID: accountID}, nil
}

// --- Posteo de asientos ---

// PostEntry persiste un asiento balanceado asignándole número correlativo gapless
// dentro de una única transacción (SELECT FOR UPDATE sobre ledger_sequences).
// El balanceo se valida en el usecase antes de llegar acá.
func (r *Repository) PostEntry(ctx context.Context, in ledgerdomain.JournalEntry) (ledgerdomain.JournalEntry, error) {
	var entryID uuid.UUID
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		id, err := postEntryTx(tx, in)
		entryID = id
		return err
	})
	if isUniqueViolation(err) {
		return ledgerdomain.JournalEntry{}, ErrAlreadyExists
	}
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	return r.GetEntry(ctx, in.OrgID, entryID)
}

// postEntryTx asume estar dentro de una transacción. Reutilizable por el worker
// del outbox (M2+) para postear en la misma tx del drenado.
func postEntryTx(tx *gorm.DB, in ledgerdomain.JournalEntry) (uuid.UUID, error) {
	var seq models.SequenceModel
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("org_id = ?", in.OrgID).Take(&seq).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, err
		}
		seq = models.SequenceModel{OrgID: in.OrgID, NextEntryNumber: 1}
		if err := tx.Create(&seq).Error; err != nil {
			return uuid.Nil, err
		}
	}

	var prefix string
	if err := tx.Table("org_settings").Select("COALESCE(journal_prefix, 'ASTO')").Where("org_id = ?", in.OrgID).Scan(&prefix).Error; err != nil {
		return uuid.Nil, err
	}
	if prefix == "" {
		prefix = "ASTO"
	}

	rate := in.ExchangeRate
	if rate == 0 {
		rate = 1
	}
	currency := in.Currency
	if currency == "" {
		currency = "ARS"
	}
	entryDate := in.EntryDate
	if entryDate.IsZero() {
		entryDate = time.Now().UTC()
	}

	entry := models.JournalEntryModel{
		ID:           uuid.New(),
		OrgID:        in.OrgID,
		EntryNumber:  fmt.Sprintf("%s-%08d", prefix, seq.NextEntryNumber),
		EntryDate:    entryDate,
		Currency:     currency,
		ExchangeRate: rate,
		SourceType:   defaultStr(in.SourceType, "manual"),
		SourceID:     in.SourceID,
		SourceEvent:  defaultStr(in.SourceEvent, "manual"),
		Description:  in.Description,
		Status:       "posted",
		CreatedBy:    in.CreatedBy,
	}
	if err := tx.Create(&entry).Error; err != nil {
		return uuid.Nil, err
	}

	lines := make([]models.JournalLineModel, 0, len(in.Lines))
	for i, l := range in.Lines {
		lines = append(lines, models.JournalLineModel{
			ID:         uuid.New(),
			OrgID:      in.OrgID,
			EntryID:    entry.ID,
			AccountID:  l.AccountID,
			Debit:      l.Debit,
			Credit:     l.Credit,
			BaseAmount: (l.Debit + l.Credit) * rate,
			PartyID:    l.PartyID,
			Memo:       l.Memo,
			LineNo:     i + 1,
		})
	}
	if err := tx.Create(&lines).Error; err != nil {
		return uuid.Nil, err
	}

	if err := tx.Model(&models.SequenceModel{}).Where("org_id = ?", in.OrgID).
		Update("next_entry_number", seq.NextEntryNumber+1).Error; err != nil {
		return uuid.Nil, err
	}
	return entry.ID, nil
}

func (r *Repository) GetEntry(ctx context.Context, orgID, id uuid.UUID) (ledgerdomain.JournalEntry, error) {
	var head models.JournalEntryModel
	err := r.db.WithContext(ctx).Where("org_id = ? AND id = ?", orgID, id).Take(&head).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ledgerdomain.JournalEntry{}, ErrNotFound
	}
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	lines, err := r.listLines(ctx, orgID, []uuid.UUID{id})
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	return toEntryDomain(head, lines[id]), nil
}

// --- Outbox / worker contable ---

// EnqueueOutbox inserta un evento contable. Con reArm=false es idempotente
// (DoNothing en conflicto: ventas/cobros, once-only). Con reArm=true reactiva la
// fila a 'pending' con payload fresco (compras: el evento de sync se re-dispara
// en cada cambio de estado para reconciliar el mayor).
func (r *Repository) EnqueueOutbox(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID, sourceEvent string, payload []byte, reArm bool) error {
	row := models.OutboxModel{
		ID: uuid.New(), OrgID: orgID, ReferenceType: refType, ReferenceID: refID,
		SourceEvent: sourceEvent, Payload: payload, Status: "pending", MaxAttempts: 10,
	}
	conflict := clause.OnConflict{
		Columns:   []clause.Column{{Name: "org_id"}, {Name: "reference_type"}, {Name: "reference_id"}, {Name: "source_event"}},
		DoNothing: true,
	}
	if reArm {
		conflict.DoNothing = false
		conflict.DoUpdates = clause.Assignments(map[string]any{
			"status": "pending", "attempts": 0, "next_retry": nil, "last_error": "", "payload": payload,
		})
	}
	return r.db.WithContext(ctx).Clauses(conflict).Create(&row).Error
}

// ProcessDueOutbox drena hasta `limit` eventos vencidos. Si orgFilter != nil sólo
// procesa esa org (posteo best-effort tras un alta). Cada evento corre en su
// propia transacción con FOR UPDATE SKIP LOCKED, de modo que múltiples workers
// no se pisan y nunca se duplica un asiento.
func (r *Repository) ProcessDueOutbox(ctx context.Context, orgFilter *uuid.UUID, limit int) (posted, failed, skipped int, err error) {
	if limit <= 0 {
		limit = 100
	}
	for i := 0; i < limit; i++ {
		found, st, e := r.processOneOutbox(ctx, orgFilter)
		if e != nil {
			return posted, failed, skipped, e
		}
		if !found {
			break
		}
		switch st {
		case "posted":
			posted++
		case "failed":
			failed++
		case "skipped":
			skipped++
		}
	}
	return posted, failed, skipped, nil
}

func (r *Repository) processOneOutbox(ctx context.Context, orgFilter *uuid.UUID) (found bool, status string, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status IN ? AND (next_retry IS NULL OR next_retry <= now())", []string{"pending", "failed"})
		if orgFilter != nil {
			q = q.Where("org_id = ?", *orgFilter)
		}
		var row models.OutboxModel
		e := q.Order("created_at ASC").Take(&row).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return nil
		}
		if e != nil {
			return e
		}
		found = true

		enabled, e := ledgerEnabledTx(tx, row.OrgID)
		if e != nil {
			return e
		}
		if !enabled {
			status = "skipped"
			return finalizeOutboxTx(tx, &row, "skipped", "ledger disabled for org")
		}

		links, e := resolveLinksTx(tx, row.OrgID)
		if e != nil {
			return e
		}
		act, planErr := planOutbox(tx, row, links)
		if planErr != nil {
			status = "failed"
			return failOutboxTx(tx, &row, planErr)
		}
		switch act.kind {
		case actionSkip:
			status = "skipped"
			return finalizeOutboxTx(tx, &row, "skipped", act.note)
		case actionStorno:
			if err := reverseEntryTx(tx, row.OrgID, act.targetEntryID); err != nil {
				status = "failed"
				return failOutboxTx(tx, &row, err)
			}
			status = "posted"
			return finalizeOutboxTx(tx, &row, "posted", "storno")
		default: // actionPost
			if _, postErr := postEntryTx(tx, act.entry); postErr != nil {
				if isUniqueViolation(postErr) {
					// Ya posteado (re-drenado): idempotente, no es error.
					status = "posted"
					return finalizeOutboxTx(tx, &row, "posted", "")
				}
				status = "failed"
				return failOutboxTx(tx, &row, postErr)
			}
			status = "posted"
			return finalizeOutboxTx(tx, &row, "posted", "")
		}
	})
	if err != nil {
		return false, "", err
	}
	return found, status, nil
}

const (
	actionPost   = "post"
	actionStorno = "storno"
	actionSkip   = "skip"
)

// outboxAction es lo que el worker debe hacer con un evento: postear un asiento
// nuevo, stornear uno existente, o no hacer nada.
type outboxAction struct {
	kind          string
	entry         ledgerdomain.JournalEntry
	targetEntryID uuid.UUID
	note          string
}

// planOutbox decide la acción para un evento. Necesita tx porque algunas
// decisiones dependen del estado actual del mayor o del documento.
func planOutbox(tx *gorm.DB, row models.OutboxModel, links map[string]uuid.UUID) (outboxAction, error) {
	switch {
	case row.ReferenceType == "sale" && row.SourceEvent == "sale.completed":
		var evt SaleEvent
		if err := json.Unmarshal(row.Payload, &evt); err != nil {
			return outboxAction{}, fmt.Errorf("decode sale event: %w", err)
		}
		entry, err := buildSaleEntry(evt, links)
		if err != nil {
			return outboxAction{}, err
		}
		return outboxAction{kind: actionPost, entry: entry}, nil

	case row.ReferenceType == "payment" && row.SourceEvent == "payment.created":
		var evt PaymentEvent
		if err := json.Unmarshal(row.Payload, &evt); err != nil {
			return outboxAction{}, fmt.Errorf("decode payment event: %w", err)
		}
		recvAcc, ok := links["receivable"]
		if !ok || recvAcc == uuid.Nil {
			return outboxAction{kind: actionSkip, note: "contado sale: cash already recognized"}, nil
		}
		credit, err := saleHasReceivableDebitTx(tx, evt.OrgID, evt.SaleID, recvAcc)
		if err != nil {
			return outboxAction{}, err
		}
		if !credit {
			return outboxAction{kind: actionSkip, note: "contado sale: cash already recognized"}, nil
		}
		partyID, currency, err := saleHeaderTx(tx, evt.OrgID, evt.SaleID)
		if err != nil {
			return outboxAction{}, err
		}
		if strings.TrimSpace(evt.Currency) == "" {
			evt.Currency = currency
		}
		entry, err := buildPaymentEntry(evt, links, partyID)
		if err != nil {
			return outboxAction{}, err
		}
		return outboxAction{kind: actionPost, entry: entry}, nil

	case row.ReferenceType == "payment" && row.SourceEvent == "supplier_payment.created":
		var evt PurchasePaymentEvent
		if err := json.Unmarshal(row.Payload, &evt); err != nil {
			return outboxAction{}, fmt.Errorf("decode supplier payment event: %w", err)
		}
		partyID, currency, err := purchaseHeaderTx(tx, evt.OrgID, evt.PurchaseID)
		if err != nil {
			return outboxAction{}, err
		}
		if strings.TrimSpace(evt.Currency) == "" {
			evt.Currency = currency
		}
		entry, err := buildPurchasePaymentEntry(evt, links, partyID)
		if err != nil {
			return outboxAction{}, err
		}
		return outboxAction{kind: actionPost, entry: entry}, nil

	case row.ReferenceType == "purchase" && row.SourceEvent == "purchase.sync":
		var evt PurchaseEvent
		if err := json.Unmarshal(row.Payload, &evt); err != nil {
			return outboxAction{}, fmt.Errorf("decode purchase event: %w", err)
		}
		return planPurchaseSync(tx, evt, links)

	case row.ReferenceType == "return" && row.SourceEvent == "return.created":
		var evt ReturnEvent
		if err := json.Unmarshal(row.Payload, &evt); err != nil {
			return outboxAction{}, fmt.Errorf("decode return event: %w", err)
		}
		entry, err := buildReturnEntry(evt, links)
		if err != nil {
			return outboxAction{}, err
		}
		return outboxAction{kind: actionPost, entry: entry}, nil

	case row.SourceEvent == "reversal":
		var evt ReversalEvent
		if err := json.Unmarshal(row.Payload, &evt); err != nil {
			return outboxAction{}, fmt.Errorf("decode reversal event: %w", err)
		}
		entryID, found, err := findPostedEntryTx(tx, evt.OrgID, evt.RefType, evt.RefID, evt.TargetEvent)
		if err != nil {
			return outboxAction{}, err
		}
		if !found {
			// El documento no tenía asiento (p. ej. cobro de venta contado): nada que reversar.
			return outboxAction{kind: actionSkip, note: "no posted entry to reverse"}, nil
		}
		return outboxAction{kind: actionStorno, targetEntryID: entryID, note: "reversal"}, nil

	default:
		return outboxAction{}, fmt.Errorf("unsupported outbox event %s/%s", row.ReferenceType, row.SourceEvent)
	}
}

// planPurchaseSync reconcilia el mayor al estado actual de la compra: alta si
// está 'received' y no hay asiento vigente; storno si dejó de estar received.
func planPurchaseSync(tx *gorm.DB, evt PurchaseEvent, links map[string]uuid.UUID) (outboxAction, error) {
	data, statusStr, found, err := readPurchaseTx(tx, evt.OrgID, evt.PurchaseID)
	if err != nil {
		return outboxAction{}, err
	}
	if !found {
		return outboxAction{kind: actionSkip, note: "purchase not found"}, nil
	}
	vigenteID, hasVigente, err := vigentePurchaseAltaTx(tx, evt.OrgID, evt.PurchaseID)
	if err != nil {
		return outboxAction{}, err
	}
	received := statusStr == "received"
	switch {
	case received && !hasVigente:
		entry, err := buildPurchaseEntry(data, links)
		if err != nil {
			return outboxAction{}, err
		}
		return outboxAction{kind: actionPost, entry: entry}, nil
	case !received && hasVigente:
		return outboxAction{kind: actionStorno, targetEntryID: vigenteID, note: "purchase no longer received"}, nil
	default:
		return outboxAction{kind: actionSkip, note: "ledger already consistent with purchase"}, nil
	}
}

func readPurchaseTx(tx *gorm.DB, orgID, purchaseID uuid.UUID) (purchaseData, string, bool, error) {
	var h struct {
		Status     string
		Currency   string
		PartyID    *uuid.UUID
		Subtotal   float64
		TaxTotal   float64
		Total      float64
		ReceivedAt *time.Time
	}
	err := tx.Table("purchases").
		Select("status, currency, party_id, subtotal, tax_total, total, received_at").
		Where("org_id = ? AND id = ?", orgID, purchaseID).Take(&h).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return purchaseData{}, "", false, nil
	}
	if err != nil {
		return purchaseData{}, "", false, err
	}
	var items []struct {
		ProductID *uuid.UUID
		TaxRate   float64
		Subtotal  float64
	}
	if err := tx.Table("purchase_items").Select("product_id, tax_rate, subtotal").Where("purchase_id = ?", purchaseID).Scan(&items).Error; err != nil {
		return purchaseData{}, "", false, err
	}
	occurred := time.Now().UTC()
	if h.ReceivedAt != nil {
		occurred = *h.ReceivedAt
	}
	data := purchaseData{
		OrgID: orgID, PurchaseID: purchaseID, OccurredAt: occurred, Currency: h.Currency,
		PartyID: h.PartyID, Subtotal: h.Subtotal, TaxTotal: h.TaxTotal, Total: h.Total,
	}
	for _, it := range items {
		data.Items = append(data.Items, purchaseLine{IsProduct: it.ProductID != nil, TaxRate: it.TaxRate, Subtotal: it.Subtotal})
	}
	return data, h.Status, true, nil
}

// purchaseHeaderTx lee party (proveedor) y moneda de la compra para el asiento de pago.
func purchaseHeaderTx(tx *gorm.DB, orgID, purchaseID uuid.UUID) (*uuid.UUID, string, error) {
	var row struct {
		PartyID  *uuid.UUID
		Currency string
	}
	err := tx.Table("purchases").Select("party_id, currency").Where("org_id = ? AND id = ?", orgID, purchaseID).Scan(&row).Error
	return row.PartyID, row.Currency, err
}

func vigentePurchaseAltaTx(tx *gorm.DB, orgID, purchaseID uuid.UUID) (uuid.UUID, bool, error) {
	var idStr string
	err := tx.Table("journal_entries").Select("id::text").
		Where("org_id = ? AND source_type = 'purchase' AND source_id = ? AND source_event = 'purchase.received' AND status = 'posted'", orgID, purchaseID).
		Limit(1).Scan(&idStr).Error
	if err != nil {
		return uuid.Nil, false, err
	}
	if strings.TrimSpace(idStr) == "" {
		return uuid.Nil, false, nil
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, false, err
	}
	return id, true, nil
}

// findPostedEntryTx ubica el asiento posteado (no reversado) de un documento por
// (source_type, source_id, source_event).
func findPostedEntryTx(tx *gorm.DB, orgID uuid.UUID, sourceType string, sourceID uuid.UUID, sourceEvent string) (uuid.UUID, bool, error) {
	q := tx.Table("journal_entries").Select("id::text").
		Where("org_id = ? AND source_type = ? AND source_id = ? AND status = 'posted'", orgID, sourceType, sourceID)
	// targetEvent vacío = cualquier asiento del documento (un pago tiene ≤1).
	if strings.TrimSpace(sourceEvent) != "" {
		q = q.Where("source_event = ?", sourceEvent)
	}
	var idStr string
	err := q.Limit(1).Scan(&idStr).Error
	if err != nil {
		return uuid.Nil, false, err
	}
	if strings.TrimSpace(idStr) == "" {
		return uuid.Nil, false, nil
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, false, err
	}
	return id, true, nil
}

// reverseEntryTx postea un asiento de storno (debe/haber invertidos) del asiento
// original y lo marca 'reversed'. Reutilizable para void/devoluciones.
func reverseEntryTx(tx *gorm.DB, orgID, entryID uuid.UUID) error {
	var head models.JournalEntryModel
	if err := tx.Where("org_id = ? AND id = ?", orgID, entryID).Take(&head).Error; err != nil {
		return err
	}
	if head.Status != "posted" {
		return nil // ya reversado
	}
	var lines []models.JournalLineModel
	if err := tx.Where("entry_id = ?", entryID).Order("line_no ASC").Find(&lines).Error; err != nil {
		return err
	}
	rev := ledgerdomain.JournalEntry{
		OrgID:        orgID,
		EntryDate:    head.EntryDate,
		Currency:     head.Currency,
		ExchangeRate: head.ExchangeRate,
		SourceType:   head.SourceType,
		SourceID:     head.SourceID,
		SourceEvent:  head.SourceEvent + ".reversal",
		Description:  "Reversa " + head.EntryNumber,
		CreatedBy:    head.CreatedBy,
	}
	for _, l := range lines {
		rev.Lines = append(rev.Lines, ledgerdomain.JournalLine{
			OrgID: orgID, AccountID: l.AccountID, Debit: l.Credit, Credit: l.Debit,
			PartyID: l.PartyID, Memo: "Reversa: " + l.Memo,
		})
	}
	newID, err := postEntryTx(tx, rev)
	if err != nil {
		return err
	}
	return tx.Model(&models.JournalEntryModel{}).Where("id = ?", entryID).
		Updates(map[string]any{"status": "reversed", "reversed_by_entry_id": newID}).Error
}

// saleHasReceivableDebitTx indica si el asiento posteado de la venta debitó la
// cuenta de deudores (es decir, fue a crédito).
func saleHasReceivableDebitTx(tx *gorm.DB, orgID, saleID, receivableAccID uuid.UUID) (bool, error) {
	var exists bool
	err := tx.Raw(`SELECT EXISTS(
		SELECT 1 FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.entry_id
		WHERE je.org_id = ? AND je.source_type = 'sale' AND je.source_id = ?
		  AND je.status = 'posted' AND jl.account_id = ? AND jl.debit > 0)`,
		orgID, saleID, receivableAccID).Scan(&exists).Error
	return exists, err
}

// saleHeaderTx lee party (customer) y moneda de la venta para el asiento de cobro.
func saleHeaderTx(tx *gorm.DB, orgID, saleID uuid.UUID) (*uuid.UUID, string, error) {
	var row struct {
		PartyID  *uuid.UUID
		Currency string
	}
	err := tx.Table("sales").Select("party_id, currency").Where("org_id = ? AND id = ?", orgID, saleID).Scan(&row).Error
	return row.PartyID, row.Currency, err
}

func finalizeOutboxTx(tx *gorm.DB, row *models.OutboxModel, status, note string) error {
	if err := tx.Model(&models.OutboxModel{}).Where("id = ?", row.ID).Updates(map[string]any{
		"status": status, "last_error": note, "next_retry": nil,
	}).Error; err != nil {
		return err
	}
	return updateDocPostingStatusTx(tx, row.OrgID, row.ReferenceType, row.ReferenceID, status)
}

func failOutboxTx(tx *gorm.DB, row *models.OutboxModel, cause error) error {
	attempts := row.Attempts + 1
	status := "failed"
	if attempts >= row.MaxAttempts {
		status = "dead"
	}
	backoff := time.Duration(attempts*attempts) * time.Minute
	if backoff > time.Hour {
		backoff = time.Hour
	}
	next := time.Now().UTC().Add(backoff)
	if err := tx.Model(&models.OutboxModel{}).Where("id = ?", row.ID).Updates(map[string]any{
		"status": status, "attempts": attempts, "last_error": cause.Error(), "next_retry": next,
	}).Error; err != nil {
		return err
	}
	return updateDocPostingStatusTx(tx, row.OrgID, row.ReferenceType, row.ReferenceID, "failed")
}

func updateDocPostingStatusTx(tx *gorm.DB, orgID uuid.UUID, refType string, refID uuid.UUID, status string) error {
	table := postingStatusTable(refType)
	if table == "" {
		return nil
	}
	return tx.Table(table).Where("org_id = ? AND id = ?", orgID, refID).Update("posting_status", status).Error
}

func postingStatusTable(refType string) string {
	switch refType {
	case "sale":
		return "sales"
	case "purchase":
		return "purchases"
	case "payment":
		return "payments"
	case "return":
		return "returns"
	default:
		return ""
	}
}

func ledgerEnabledTx(tx *gorm.DB, orgID uuid.UUID) (bool, error) {
	var enabled bool
	err := tx.Table("org_settings").Select("COALESCE(ledger_enabled, false)").Where("org_id = ?", orgID).Scan(&enabled).Error
	return enabled, err
}

func resolveLinksTx(tx *gorm.DB, orgID uuid.UUID) (map[string]uuid.UUID, error) {
	type lr struct {
		Role      string
		AccountID uuid.UUID
	}
	var rows []lr
	if err := tx.Table("ledger_account_links").Select("role, account_id").Where("org_id = ?", orgID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]uuid.UUID, len(rows))
	for _, row := range rows {
		m[row.Role] = row.AccountID
	}
	return m, nil
}

// --- Reportes ---

func (r *Repository) Journal(ctx context.Context, orgID uuid.UUID, from, to time.Time, limit int) ([]ledgerdomain.JournalEntry, error) {
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 50, MaxLimit: 200})
	q := r.db.WithContext(ctx).Model(&models.JournalEntryModel{}).Where("org_id = ?", orgID)
	if !from.IsZero() {
		q = q.Where("entry_date >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("entry_date <= ?", to)
	}
	var heads []models.JournalEntryModel
	if err := q.Order("entry_date DESC").Order("entry_number DESC").Limit(limit).Find(&heads).Error; err != nil {
		return nil, err
	}
	if len(heads) == 0 {
		return []ledgerdomain.JournalEntry{}, nil
	}
	ids := make([]uuid.UUID, 0, len(heads))
	for _, h := range heads {
		ids = append(ids, h.ID)
	}
	linesByEntry, err := r.listLines(ctx, orgID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]ledgerdomain.JournalEntry, 0, len(heads))
	for _, h := range heads {
		out = append(out, toEntryDomain(h, linesByEntry[h.ID]))
	}
	return out, nil
}

func (r *Repository) AccountLedger(ctx context.Context, orgID, accountID uuid.UUID, from, to time.Time, limit int) (ledgerdomain.AccountLedger, error) {
	acc, err := r.GetAccount(ctx, orgID, accountID)
	if err != nil {
		return ledgerdomain.AccountLedger{}, err
	}
	limit = pagination.NormalizeLimit(limit, pagination.Config{DefaultLimit: 100, MaxLimit: 500})

	// Saldo de apertura: Σ(debit-credit) anterior a `from`.
	opening := 0.0
	if !from.IsZero() {
		if err := r.db.WithContext(ctx).
			Table("journal_lines AS jl").
			Joins("JOIN journal_entries je ON je.id = jl.entry_id").
			Where("jl.org_id = ? AND jl.account_id = ? AND je.entry_date < ?", orgID, accountID, from).
			Select("COALESCE(SUM(jl.debit - jl.credit), 0)").
			Scan(&opening).Error; err != nil {
			return ledgerdomain.AccountLedger{}, err
		}
	}

	type lineRow struct {
		EntryID     uuid.UUID
		EntryNumber string
		EntryDate   time.Time
		Description string
		Debit       float64
		Credit      float64
	}
	q := r.db.WithContext(ctx).
		Table("journal_lines AS jl").
		Joins("JOIN journal_entries je ON je.id = jl.entry_id").
		Where("jl.org_id = ? AND jl.account_id = ?", orgID, accountID)
	if !from.IsZero() {
		q = q.Where("je.entry_date >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("je.entry_date <= ?", to)
	}
	var rows []lineRow
	if err := q.
		Select("je.id AS entry_id, je.entry_number, je.entry_date, je.description, jl.debit, jl.credit").
		Order("je.entry_date ASC").Order("je.entry_number ASC").
		Limit(limit).Scan(&rows).Error; err != nil {
		return ledgerdomain.AccountLedger{}, err
	}

	balance := opening
	lines := make([]ledgerdomain.LedgerLine, 0, len(rows))
	for _, row := range rows {
		balance += row.Debit - row.Credit
		lines = append(lines, ledgerdomain.LedgerLine{
			EntryID:     row.EntryID,
			EntryNumber: row.EntryNumber,
			EntryDate:   row.EntryDate,
			Description: row.Description,
			Debit:       row.Debit,
			Credit:      row.Credit,
			Balance:     balance,
		})
	}
	return ledgerdomain.AccountLedger{Account: acc, Opening: opening, Closing: balance, Lines: lines}, nil
}

func (r *Repository) TrialBalance(ctx context.Context, orgID uuid.UUID, asOf time.Time) (ledgerdomain.TrialBalance, error) {
	type row struct {
		AccountID uuid.UUID
		Code      string
		Name      string
		Type      string
		Debit     float64
		Credit    float64
	}
	q := r.db.WithContext(ctx).
		Table("journal_lines AS jl").
		Joins("JOIN journal_entries je ON je.id = jl.entry_id").
		Joins("JOIN ledger_accounts a ON a.id = jl.account_id").
		Where("jl.org_id = ?", orgID)
	if !asOf.IsZero() {
		q = q.Where("je.entry_date <= ?", asOf)
	}
	var rows []row
	if err := q.
		Select("a.id AS account_id, a.code, a.name, a.type, COALESCE(SUM(jl.debit),0) AS debit, COALESCE(SUM(jl.credit),0) AS credit").
		Group("a.id, a.code, a.name, a.type").
		Order("a.code ASC").
		Scan(&rows).Error; err != nil {
		return ledgerdomain.TrialBalance{}, err
	}
	out := ledgerdomain.TrialBalance{AsOf: asOf, Rows: make([]ledgerdomain.TrialBalanceRow, 0, len(rows))}
	for _, row := range rows {
		out.Rows = append(out.Rows, ledgerdomain.TrialBalanceRow{
			AccountID: row.AccountID, Code: row.Code, Name: row.Name, Type: row.Type,
			Debit: row.Debit, Credit: row.Credit, Balance: row.Debit - row.Credit,
		})
		out.TotalDebit += row.Debit
		out.TotalCredit += row.Credit
	}
	return out, nil
}

func (r *Repository) OutboxHealth(ctx context.Context, orgID uuid.UUID) (ledgerdomain.OutboxHealth, error) {
	type row struct {
		Status string
		Count  int
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("ledger_outbox").
		Select("status, COUNT(*) AS count").
		Where("org_id = ?", orgID).
		Group("status").
		Scan(&rows).Error; err != nil {
		return ledgerdomain.OutboxHealth{}, err
	}
	out := ledgerdomain.OutboxHealth{OrgID: orgID}
	for _, row := range rows {
		switch row.Status {
		case "pending":
			out.Pending = row.Count
		case "failed":
			out.Failed = row.Count
		case "posted":
			out.Posted = row.Count
		case "skipped":
			out.Skipped = row.Count
		case "dead":
			out.Dead = row.Count
		}
	}
	return out, nil
}

// --- Seed plantilla de plan de cuentas ---

// SeedAccount describe una cuenta de la plantilla por defecto y, opcionalmente,
// el rol funcional al que se la enlaza.
type SeedAccount struct {
	Code string
	Name string
	Type string
	Role string
}

// SeedChart inserta (idempotente, por code) las cuentas de la plantilla y enlaza
// sus roles. Si una cuenta ya existe se respeta; los links se upsertan por rol.
func (r *Repository) SeedChart(ctx context.Context, orgID uuid.UUID, seed []SeedAccount) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, s := range seed {
			var existing models.AccountModel
			err := tx.Where("org_id = ? AND code = ? AND archived_at IS NULL", orgID, s.Code).Take(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				existing = models.AccountModel{ID: uuid.New(), OrgID: orgID, Code: s.Code, Name: s.Name, Type: s.Type, IsPostable: true}
				if err := tx.Create(&existing).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			if s.Role == "" {
				continue
			}
			link := models.AccountLinkModel{ID: uuid.New(), OrgID: orgID, Role: s.Role, AccountID: existing.ID}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "org_id"}, {Name: "role"}},
				DoUpdates: clause.AssignmentColumns([]string{"account_id", "updated_at"}),
			}).Create(&link).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// --- mappers / helpers ---

func toAccountDomain(row models.AccountModel) ledgerdomain.Account {
	return ledgerdomain.Account{
		ID: row.ID, OrgID: row.OrgID, Code: row.Code, Name: row.Name, Type: row.Type,
		ParentID: row.ParentID, IsPostable: row.IsPostable, ArchivedAt: row.ArchivedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func (r *Repository) listLines(ctx context.Context, orgID uuid.UUID, entryIDs []uuid.UUID) (map[uuid.UUID][]ledgerdomain.JournalLine, error) {
	type lineRow struct {
		models.JournalLineModel
		AccountCode string
		AccountName string
	}
	var rows []lineRow
	err := r.db.WithContext(ctx).
		Table("journal_lines AS jl").
		Select("jl.*, a.code AS account_code, a.name AS account_name").
		Joins("JOIN ledger_accounts a ON a.id = jl.account_id").
		Where("jl.org_id = ? AND jl.entry_id IN ?", orgID, entryIDs).
		Order("jl.line_no ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]ledgerdomain.JournalLine, len(entryIDs))
	for _, row := range rows {
		out[row.EntryID] = append(out[row.EntryID], ledgerdomain.JournalLine{
			ID: row.ID, OrgID: row.OrgID, EntryID: row.EntryID, AccountID: row.AccountID,
			AccountCode: row.AccountCode, AccountName: row.AccountName,
			Debit: row.Debit, Credit: row.Credit, BaseAmount: row.BaseAmount,
			PartyID: row.PartyID, Memo: row.Memo, LineNo: row.LineNo,
		})
	}
	return out, nil
}

func toEntryDomain(head models.JournalEntryModel, lines []ledgerdomain.JournalLine) ledgerdomain.JournalEntry {
	return ledgerdomain.JournalEntry{
		ID: head.ID, OrgID: head.OrgID, EntryNumber: head.EntryNumber, EntryDate: head.EntryDate,
		Currency: head.Currency, ExchangeRate: head.ExchangeRate, SourceType: head.SourceType,
		SourceID: head.SourceID, SourceEvent: head.SourceEvent, Description: head.Description,
		Status: head.Status, CreatedBy: head.CreatedBy, CreatedAt: head.CreatedAt, Lines: lines,
	}
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func isUniqueViolation(err error) bool {
	return err != nil && corepostgres.IsUniqueViolation(err)
}
