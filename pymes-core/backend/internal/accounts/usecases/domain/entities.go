package domain

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID          uuid.UUID  `json:"id"`
	OrgID    uuid.UUID  `json:"org_id"`
	Type        string     `json:"type"`
	EntityType  string     `json:"entity_type"`
	EntityID    uuid.UUID  `json:"entity_id"`
	EntityName  string     `json:"entity_name"`
	Balance     float64    `json:"balance"`
	Currency    string     `json:"currency"`
	CreditLimit float64    `json:"credit_limit"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Movements   []Movement `json:"movements,omitempty"`
}

type Movement struct {
	ID            uuid.UUID  `json:"id"`
	AccountID     uuid.UUID  `json:"account_id"`
	OrgID      uuid.UUID  `json:"org_id"`
	Type          string     `json:"type"`
	Amount        float64    `json:"amount"`
	Balance       float64    `json:"balance"`
	Description   string     `json:"description"`
	ReferenceType string     `json:"reference_type"`
	ReferenceID   *uuid.UUID `json:"reference_id,omitempty"`
	CreatedBy     string     `json:"created_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Summary agrega saldos por tipo para un tenant. Útil para dashboards y para
// que Companion (IA) consulte estado financiero agregado sin paginar la lista
// completa de accounts.
type Summary struct {
	OrgID              uuid.UUID `json:"org_id"`
	ReceivableTotal    float64   `json:"receivable_total"`
	ReceivableNonZero  int       `json:"receivable_non_zero_count"`
	PayableTotal       float64   `json:"payable_total"`
	PayableNonZero     int       `json:"payable_non_zero_count"`
	NetPosition        float64   `json:"net_position"`
	AccountsTotalCount int       `json:"accounts_total_count"`
	Currency           string    `json:"currency"`
	GeneratedAt        time.Time `json:"generated_at"`
}
