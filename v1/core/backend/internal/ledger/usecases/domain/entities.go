package domain

import (
	"time"

	"github.com/google/uuid"
)

// Account es una cuenta del plan de cuentas (chart of accounts).
// Type sigue las categorías de LedgerSMB: A=Activo, L=Pasivo, Q=Patrimonio,
// I=Ingreso, E=Egreso.
type Account struct {
	ID         uuid.UUID  `json:"id"`
	OrgID      uuid.UUID  `json:"org_id"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	ParentID   *uuid.UUID `json:"parent_id,omitempty"`
	IsPostable bool       `json:"is_postable"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// AccountLink mapea un rol funcional (revenue, cash, receivable, vat_payable_21…)
// a una cuenta concreta del plan. Es el patrón account_link de LedgerSMB: las
// posting rules nunca hardcodean cuentas, resuelven por rol.
type AccountLink struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	Role      string    `json:"role"`
	AccountID uuid.UUID `json:"account_id"`
	// Code/Name de la cuenta enlazada, para display (no persistido aquí).
	AccountCode string `json:"account_code,omitempty"`
	AccountName string `json:"account_name,omitempty"`
}

// JournalLine es una línea de asiento: débito XOR crédito.
type JournalLine struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	EntryID     uuid.UUID  `json:"entry_id"`
	AccountID   uuid.UUID  `json:"account_id"`
	AccountCode string     `json:"account_code,omitempty"`
	AccountName string     `json:"account_name,omitempty"`
	Debit       float64    `json:"debit"`
	Credit      float64    `json:"credit"`
	BaseAmount  float64    `json:"base_amount"`
	PartyID     *uuid.UUID `json:"party_id,omitempty"`
	Memo        string     `json:"memo,omitempty"`
	LineNo      int        `json:"line_no"`
}

// JournalEntry es un asiento (cabecera + líneas). Inmutable una vez posteado:
// las correcciones se hacen con asientos de reversa (storno).
type JournalEntry struct {
	ID           uuid.UUID     `json:"id"`
	OrgID        uuid.UUID     `json:"org_id"`
	EntryNumber  string        `json:"entry_number"`
	EntryDate    time.Time     `json:"entry_date"`
	Currency     string        `json:"currency"`
	ExchangeRate float64       `json:"exchange_rate"`
	SourceType   string        `json:"source_type"`
	SourceID     *uuid.UUID    `json:"source_id,omitempty"`
	SourceEvent  string        `json:"source_event"`
	Description  string        `json:"description"`
	Status       string        `json:"status"`
	CreatedBy    string        `json:"created_by,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	Lines        []JournalLine `json:"lines,omitempty"`
}

// LedgerLine es una fila del Libro Mayor de una cuenta: movimiento con saldo
// acumulado (running balance = Σ débito − Σ crédito).
type LedgerLine struct {
	EntryID     uuid.UUID `json:"entry_id"`
	EntryNumber string    `json:"entry_number"`
	EntryDate   time.Time `json:"entry_date"`
	Description string    `json:"description"`
	Debit       float64   `json:"debit"`
	Credit      float64   `json:"credit"`
	Balance     float64   `json:"balance"`
}

// AccountLedger es el Libro Mayor de una cuenta en un rango de fechas.
type AccountLedger struct {
	Account Account      `json:"account"`
	Opening float64      `json:"opening_balance"`
	Closing float64      `json:"closing_balance"`
	Lines   []LedgerLine `json:"lines"`
}

// TrialBalanceRow es una fila de Sumas y Saldos.
type TrialBalanceRow struct {
	AccountID uuid.UUID `json:"account_id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Debit     float64   `json:"debit"`
	Credit    float64   `json:"credit"`
	Balance   float64   `json:"balance"`
}

// TrialBalance es el reporte de Sumas y Saldos completo.
type TrialBalance struct {
	Rows        []TrialBalanceRow `json:"rows"`
	TotalDebit  float64           `json:"total_debit"`
	TotalCredit float64           `json:"total_credit"`
	AsOf        time.Time         `json:"as_of"`
}

// OutboxHealth resume el estado del outbox contable de una org.
type OutboxHealth struct {
	OrgID   uuid.UUID `json:"org_id"`
	Pending int       `json:"pending"`
	Failed  int       `json:"failed"`
	Posted  int       `json:"posted"`
	Skipped int       `json:"skipped"`
	Dead    int       `json:"dead"`
}
