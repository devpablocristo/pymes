package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/platform/errors/go/domainerr"
	ledgerdomain "github.com/devpablocristo/pymes/core/backend/internal/ledger/usecases/domain"
)

type RepositoryPort interface {
	ListAccounts(ctx context.Context, orgID uuid.UUID, includeArchived bool) ([]ledgerdomain.Account, error)
	GetAccount(ctx context.Context, orgID, id uuid.UUID) (ledgerdomain.Account, error)
	CreateAccount(ctx context.Context, in ledgerdomain.Account) (ledgerdomain.Account, error)
	UpdateAccount(ctx context.Context, in ledgerdomain.Account) (ledgerdomain.Account, error)
	ArchiveAccount(ctx context.Context, orgID, id uuid.UUID) error
	RestoreAccount(ctx context.Context, orgID, id uuid.UUID) error

	ListLinks(ctx context.Context, orgID uuid.UUID) ([]ledgerdomain.AccountLink, error)
	SetLink(ctx context.Context, orgID uuid.UUID, role string, accountID uuid.UUID) (ledgerdomain.AccountLink, error)

	PostEntry(ctx context.Context, in ledgerdomain.JournalEntry) (ledgerdomain.JournalEntry, error)
	Journal(ctx context.Context, orgID uuid.UUID, from, to time.Time, limit int) ([]ledgerdomain.JournalEntry, error)
	AccountLedger(ctx context.Context, orgID, accountID uuid.UUID, from, to time.Time, limit int) (ledgerdomain.AccountLedger, error)
	TrialBalance(ctx context.Context, orgID uuid.UUID, asOf time.Time) (ledgerdomain.TrialBalance, error)
	OutboxHealth(ctx context.Context, orgID uuid.UUID) (ledgerdomain.OutboxHealth, error)

	SeedChart(ctx context.Context, orgID uuid.UUID, seed []SeedAccount) error

	EnqueueOutbox(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID, sourceEvent string, payload []byte, reArm bool) error
	ProcessDueOutbox(ctx context.Context, orgFilter *uuid.UUID, limit int) (posted, failed, skipped int, err error)
}

type Usecases struct{ repo RepositoryPort }

func NewUsecases(repo RepositoryPort) *Usecases { return &Usecases{repo: repo} }

var validAccountTypes = map[string]bool{"A": true, "L": true, "Q": true, "I": true, "E": true}

// --- Plan de cuentas ---

func (u *Usecases) ListAccounts(ctx context.Context, orgID uuid.UUID, includeArchived bool) ([]ledgerdomain.Account, error) {
	return u.repo.ListAccounts(ctx, orgID, includeArchived)
}

func (u *Usecases) GetAccount(ctx context.Context, orgID, id uuid.UUID) (ledgerdomain.Account, error) {
	out, err := u.repo.GetAccount(ctx, orgID, id)
	return out, mapRepoErr(err)
}

func (u *Usecases) CreateAccount(ctx context.Context, in ledgerdomain.Account) (ledgerdomain.Account, error) {
	if in.OrgID == uuid.Nil {
		return ledgerdomain.Account{}, domainerr.Validation("org_id is required")
	}
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	in.Type = strings.ToUpper(strings.TrimSpace(in.Type))
	if in.Code == "" {
		return ledgerdomain.Account{}, domainerr.Validation("code is required")
	}
	if in.Name == "" {
		return ledgerdomain.Account{}, domainerr.Validation("name is required")
	}
	if !validAccountTypes[in.Type] {
		return ledgerdomain.Account{}, domainerr.Validation("invalid type (expected A, L, Q, I or E)")
	}
	out, err := u.repo.CreateAccount(ctx, in)
	return out, mapRepoErr(err)
}

func (u *Usecases) UpdateAccount(ctx context.Context, in ledgerdomain.Account) (ledgerdomain.Account, error) {
	if in.OrgID == uuid.Nil || in.ID == uuid.Nil {
		return ledgerdomain.Account{}, domainerr.Validation("org_id and id are required")
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Type = strings.ToUpper(strings.TrimSpace(in.Type))
	if in.Name == "" {
		return ledgerdomain.Account{}, domainerr.Validation("name is required")
	}
	if !validAccountTypes[in.Type] {
		return ledgerdomain.Account{}, domainerr.Validation("invalid type (expected A, L, Q, I or E)")
	}
	out, err := u.repo.UpdateAccount(ctx, in)
	return out, mapRepoErr(err)
}

func (u *Usecases) ArchiveAccount(ctx context.Context, orgID, id uuid.UUID) error {
	return mapRepoErr(u.repo.ArchiveAccount(ctx, orgID, id))
}

func (u *Usecases) RestoreAccount(ctx context.Context, orgID, id uuid.UUID) error {
	return mapRepoErr(u.repo.RestoreAccount(ctx, orgID, id))
}

// --- Account links ---

func (u *Usecases) ListLinks(ctx context.Context, orgID uuid.UUID) ([]ledgerdomain.AccountLink, error) {
	return u.repo.ListLinks(ctx, orgID)
}

func (u *Usecases) SetLink(ctx context.Context, orgID uuid.UUID, role string, accountID uuid.UUID) (ledgerdomain.AccountLink, error) {
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		return ledgerdomain.AccountLink{}, domainerr.Validation("role is required")
	}
	if accountID == uuid.Nil {
		return ledgerdomain.AccountLink{}, domainerr.Validation("account_id is required")
	}
	out, err := u.repo.SetLink(ctx, orgID, role, accountID)
	return out, mapRepoErr(err)
}

// --- Asiento manual ---

// PostManual valida y postea un asiento manual (ajuste). Verifica partida doble:
// cada línea es débito XOR crédito > 0, todas las cuentas son posteables y de la
// org, y Σdébito = Σcrédito dentro de la tolerancia de un centavo.
func (u *Usecases) PostManual(ctx context.Context, in ledgerdomain.JournalEntry) (ledgerdomain.JournalEntry, error) {
	if in.OrgID == uuid.Nil {
		return ledgerdomain.JournalEntry{}, domainerr.Validation("org_id is required")
	}
	if len(in.Lines) < 2 {
		return ledgerdomain.JournalEntry{}, domainerr.Validation("an entry needs at least two lines")
	}

	accounts, err := u.repo.ListAccounts(ctx, in.OrgID, false)
	if err != nil {
		return ledgerdomain.JournalEntry{}, err
	}
	byID := make(map[uuid.UUID]ledgerdomain.Account, len(accounts))
	for _, a := range accounts {
		byID[a.ID] = a
	}

	var totalDebit, totalCredit float64
	for i := range in.Lines {
		l := &in.Lines[i]
		l.Debit = round2(l.Debit)
		l.Credit = round2(l.Credit)
		if l.Debit < 0 || l.Credit < 0 {
			return ledgerdomain.JournalEntry{}, domainerr.Validation("debit and credit must be non-negative")
		}
		if (l.Debit > 0) == (l.Credit > 0) {
			return ledgerdomain.JournalEntry{}, domainerr.Validation("each line must be either a debit or a credit")
		}
		acc, ok := byID[l.AccountID]
		if !ok {
			return ledgerdomain.JournalEntry{}, domainerr.Validation("unknown or archived account in line")
		}
		if !acc.IsPostable {
			return ledgerdomain.JournalEntry{}, domainerr.Validation("account " + acc.Code + " is not postable")
		}
		totalDebit += l.Debit
		totalCredit += l.Credit
	}
	// Comparación en centavos enteros para evitar falsos negativos por error de
	// representación float. Un asiento manual debe balancear exacto.
	if int64(math.Round((totalDebit-totalCredit)*100)) != 0 {
		return ledgerdomain.JournalEntry{}, domainerr.Validation("entry is not balanced: debit and credit totals differ")
	}

	in.SourceType = "manual"
	in.SourceEvent = "manual"
	in.SourceID = nil
	if in.EntryDate.IsZero() {
		in.EntryDate = time.Now().UTC()
	}
	out, err := u.repo.PostEntry(ctx, in)
	return out, mapRepoErr(err)
}

// --- Reportes ---

func (u *Usecases) Journal(ctx context.Context, orgID uuid.UUID, from, to time.Time, limit int) ([]ledgerdomain.JournalEntry, error) {
	return u.repo.Journal(ctx, orgID, from, to, limit)
}

func (u *Usecases) AccountLedger(ctx context.Context, orgID, accountID uuid.UUID, from, to time.Time, limit int) (ledgerdomain.AccountLedger, error) {
	out, err := u.repo.AccountLedger(ctx, orgID, accountID, from, to, limit)
	return out, mapRepoErr(err)
}

func (u *Usecases) TrialBalance(ctx context.Context, orgID uuid.UUID, asOf time.Time) (ledgerdomain.TrialBalance, error) {
	return u.repo.TrialBalance(ctx, orgID, asOf)
}

func (u *Usecases) Health(ctx context.Context, orgID uuid.UUID) (ledgerdomain.OutboxHealth, error) {
	return u.repo.OutboxHealth(ctx, orgID)
}

// --- Outbox / posteo automático ---

// EnqueueSale registra el evento de venta en el outbox contable y dispara un
// posteo best-effort inmediato. Nunca bloquea la venta: si el posteo inmediato
// falla, el evento queda 'pending'/'failed' y lo levanta el cron (ledger_post).
func (u *Usecases) EnqueueSale(ctx context.Context, evt SaleEvent) error {
	if evt.OrgID == uuid.Nil || evt.SaleID == uuid.Nil {
		return domainerr.Validation("org_id and sale_id are required")
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	if err := u.repo.EnqueueOutbox(ctx, evt.OrgID, "sale", evt.SaleID, "sale.completed", payload, false); err != nil {
		return err
	}
	orgID := evt.OrgID
	if _, _, _, err := u.repo.ProcessDueOutbox(ctx, &orgID, 10); err != nil {
		slog.WarnContext(ctx, "ledger best-effort post failed; left for scheduler", "error", err, "sale_id", evt.SaleID)
	}
	return nil
}

// EnqueuePurchasePayment encola el evento de un pago a proveedor y dispara un
// posteo best-effort. Idempotente por payment_id.
func (u *Usecases) EnqueuePurchasePayment(ctx context.Context, evt PurchasePaymentEvent) error {
	if evt.OrgID == uuid.Nil || evt.PaymentID == uuid.Nil {
		return domainerr.Validation("org_id and payment_id are required")
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	if err := u.repo.EnqueueOutbox(ctx, evt.OrgID, "payment", evt.PaymentID, "supplier_payment.created", payload, false); err != nil {
		return err
	}
	orgID := evt.OrgID
	if _, _, _, err := u.repo.ProcessDueOutbox(ctx, &orgID, 10); err != nil {
		slog.WarnContext(ctx, "ledger best-effort post failed; left for scheduler", "error", err, "payment_id", evt.PaymentID)
	}
	return nil
}

// EnqueueReturn encola el evento de una devolución (storno parcial) y dispara un
// posteo best-effort. Idempotente por return_id.
func (u *Usecases) EnqueueReturn(ctx context.Context, evt ReturnEvent) error {
	if evt.OrgID == uuid.Nil || evt.ReturnID == uuid.Nil {
		return domainerr.Validation("org_id and return_id are required")
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	if err := u.repo.EnqueueOutbox(ctx, evt.OrgID, "return", evt.ReturnID, "return.created", payload, false); err != nil {
		return err
	}
	orgID := evt.OrgID
	if _, _, _, err := u.repo.ProcessDueOutbox(ctx, &orgID, 10); err != nil {
		slog.WarnContext(ctx, "ledger best-effort post failed; left for scheduler", "error", err, "return_id", evt.ReturnID)
	}
	return nil
}

// EnqueueReversal encola el storno del asiento posteado de un documento (void de
// venta, SoftDelete de cobro, void de devolución). Si el documento no tenía
// asiento, el worker no hace nada. Idempotente por (refType, refID).
func (u *Usecases) EnqueueReversal(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID, targetEvent, actor string) error {
	if orgID == uuid.Nil || refID == uuid.Nil {
		return domainerr.Validation("org_id and ref_id are required")
	}
	payload, err := json.Marshal(ReversalEvent{OrgID: orgID, RefType: refType, RefID: refID, TargetEvent: targetEvent, Actor: actor})
	if err != nil {
		return err
	}
	if err := u.repo.EnqueueOutbox(ctx, orgID, refType, refID, "reversal", payload, false); err != nil {
		return err
	}
	org := orgID
	if _, _, _, err := u.repo.ProcessDueOutbox(ctx, &org, 10); err != nil {
		slog.WarnContext(ctx, "ledger best-effort reversal failed; left for scheduler", "error", err, "ref_id", refID)
	}
	return nil
}

// EnqueuePurchaseSync encola (re-armable) la reconciliación contable de una
// compra y dispara un posteo best-effort. Se llama en cada cambio de estado de
// la compra; el worker lleva el mayor al estado actual (alta/storno).
func (u *Usecases) EnqueuePurchaseSync(ctx context.Context, evt PurchaseEvent) error {
	if evt.OrgID == uuid.Nil || evt.PurchaseID == uuid.Nil {
		return domainerr.Validation("org_id and purchase_id are required")
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	if err := u.repo.EnqueueOutbox(ctx, evt.OrgID, "purchase", evt.PurchaseID, "purchase.sync", payload, true); err != nil {
		return err
	}
	orgID := evt.OrgID
	if _, _, _, err := u.repo.ProcessDueOutbox(ctx, &orgID, 10); err != nil {
		slog.WarnContext(ctx, "ledger best-effort post failed; left for scheduler", "error", err, "purchase_id", evt.PurchaseID)
	}
	return nil
}

// EnqueuePayment registra el evento de cobro en el outbox y dispara un posteo
// best-effort. Idempotente por payment_id. Nunca bloquea el cobro.
func (u *Usecases) EnqueuePayment(ctx context.Context, evt PaymentEvent) error {
	if evt.OrgID == uuid.Nil || evt.PaymentID == uuid.Nil {
		return domainerr.Validation("org_id and payment_id are required")
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	if err := u.repo.EnqueueOutbox(ctx, evt.OrgID, "payment", evt.PaymentID, "payment.created", payload, false); err != nil {
		return err
	}
	orgID := evt.OrgID
	if _, _, _, err := u.repo.ProcessDueOutbox(ctx, &orgID, 10); err != nil {
		slog.WarnContext(ctx, "ledger best-effort post failed; left for scheduler", "error", err, "payment_id", evt.PaymentID)
	}
	return nil
}

// ProcessDueOutbox drena el outbox contable (lo invoca el scheduler).
func (u *Usecases) ProcessDueOutbox(ctx context.Context, orgFilter *uuid.UUID, limit int) (int, int, int, error) {
	return u.repo.ProcessDueOutbox(ctx, orgFilter, limit)
}

// --- Setup / seed ---

// Setup siembra (idempotente) la plantilla de plan de cuentas por defecto y sus
// account_links. Pensado para llamarse al activar contabilidad en una org.
func (u *Usecases) Setup(ctx context.Context, orgID uuid.UUID) ([]ledgerdomain.Account, error) {
	if orgID == uuid.Nil {
		return nil, domainerr.Validation("org_id is required")
	}
	if err := u.repo.SeedChart(ctx, orgID, defaultChartTemplate()); err != nil {
		return nil, err
	}
	return u.repo.ListAccounts(ctx, orgID, false)
}

// defaultChartTemplate es un plan de cuentas genérico LATAM (editable por la org)
// con los account_links críticos precargados para que el posteo automático (M2+)
// funcione sin configuración manual.
func defaultChartTemplate() []SeedAccount {
	return []SeedAccount{
		{Code: "1.1.01", Name: "Caja", Type: "A", Role: "cash"},
		{Code: "1.1.02", Name: "Banco", Type: "A", Role: "bank"},
		{Code: "1.1.03", Name: "Deudores por ventas", Type: "A", Role: "receivable"},
		{Code: "1.1.04", Name: "IVA Crédito Fiscal 21%", Type: "A", Role: "vat_credit_21"},
		{Code: "1.1.05", Name: "IVA Crédito Fiscal 10.5%", Type: "A", Role: "vat_credit_105"},
		{Code: "1.1.06", Name: "Tarjetas a liquidar", Type: "A", Role: "card_clearing"},
		{Code: "1.1.07", Name: "MercadoPago a liquidar", Type: "A", Role: "mp_clearing"},
		{Code: "1.2.01", Name: "Mercaderías", Type: "A", Role: "inventory"},
		{Code: "2.1.01", Name: "Proveedores", Type: "L", Role: "payable"},
		{Code: "2.1.02", Name: "IVA Débito Fiscal 21%", Type: "L", Role: "vat_payable_21"},
		{Code: "2.1.03", Name: "IVA Débito Fiscal 10.5%", Type: "L", Role: "vat_payable_105"},
		{Code: "2.1.04", Name: "Notas de crédito a clientes", Type: "L", Role: "credit_note_payable"},
		{Code: "3.1.01", Name: "Resultado del ejercicio", Type: "Q", Role: "retained_earnings"},
		{Code: "4.1.01", Name: "Ventas", Type: "I", Role: "revenue"},
		{Code: "5.1.01", Name: "Costo de mercadería vendida", Type: "E", Role: "cogs"},
		{Code: "5.2.01", Name: "Gastos / Compras de servicios", Type: "E", Role: "purchase_expense"},
	}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// mapRepoErr traduce los sentinels del repository a errores de dominio que el
// adapter HTTP sabe mapear a códigos de estado.
func mapRepoErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNotFound):
		return domainerr.NotFoundf("ledger", "resource")
	case errors.Is(err, ErrAlreadyExists):
		return domainerr.Conflict("ledger resource already exists")
	default:
		return err
	}
}
