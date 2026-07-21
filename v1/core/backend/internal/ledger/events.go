package ledger

import (
	"time"

	"github.com/google/uuid"
)

// SaleEvent es el snapshot de una venta que el módulo sales encola en el outbox
// contable. Es un contrato JSON-serializable: el worker lo reconstruye desde el
// payload y aplica las posting rules. Lleva los totales YA PERSISTIDOS de la
// venta (Subtotal/TaxTotal/Total) para anclar el asiento al documento y evitar
// divergencias de redondeo (ver posting.go).
type SaleEvent struct {
	OrgID         uuid.UUID       `json:"org_id"`
	SaleID        uuid.UUID       `json:"sale_id"`
	Number        string          `json:"number"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Currency      string          `json:"currency"`
	PaymentStatus string          `json:"payment_status"` // "paid" (contado) | "pending" (crédito)
	PaymentMethod string          `json:"payment_method"` // cash|card|transfer|other
	PartyID       *uuid.UUID      `json:"party_id,omitempty"`
	Subtotal      float64         `json:"subtotal"` // neto (sin IVA)
	TaxTotal      float64         `json:"tax_total"`
	Total         float64         `json:"total"`
	Lines         []SaleEventLine `json:"lines"`
	Actor         string          `json:"actor,omitempty"`
}

// SaleEventLine es lo mínimo que el asiento necesita de cada línea de venta:
// su neto y su alícuota, para desglosar el IVA por tasa.
type SaleEventLine struct {
	TaxRate  float64 `json:"tax_rate"`
	Subtotal float64 `json:"subtotal"`
}

// PaymentEvent es el snapshot de un cobro de venta. El worker decide si genera
// asiento: si la venta fue a crédito (su asiento debitó Deudores) postea
// DR caja/banco / CR Deudores; si fue de contado, NO postea (la venta ya
// reconoció la caja) — evita el doble-conteo.
type PaymentEvent struct {
	OrgID      uuid.UUID `json:"org_id"`
	PaymentID  uuid.UUID `json:"payment_id"`
	SaleID     uuid.UUID `json:"sale_id"`
	Method     string    `json:"method"`
	Amount     float64   `json:"amount"`
	OccurredAt time.Time `json:"occurred_at"`
	Currency   string    `json:"currency"`
	Actor      string    `json:"actor,omitempty"`
}

// PurchasePaymentEvent es el snapshot de un pago a proveedor. Siempre postea
// DR Proveedores / CR caja-banco (la compra recibida acreditó Proveedores, nunca
// Caja, así que no hay doble-conteo).
type PurchasePaymentEvent struct {
	OrgID      uuid.UUID `json:"org_id"`
	PaymentID  uuid.UUID `json:"payment_id"`
	PurchaseID uuid.UUID `json:"purchase_id"`
	Method     string    `json:"method"`
	Amount     float64   `json:"amount"`
	OccurredAt time.Time `json:"occurred_at"`
	Currency   string    `json:"currency"`
	Actor      string    `json:"actor,omitempty"`
}

// ReturnEvent es el snapshot de una devolución (storno parcial de la venta).
// Reversa Ventas + IVA débito proporcional a lo devuelto; el contrapartida es
// caja (refund cash/original) o el pasivo por nota de crédito.
type ReturnEvent struct {
	OrgID        uuid.UUID       `json:"org_id"`
	ReturnID     uuid.UUID       `json:"return_id"`
	SaleID       uuid.UUID       `json:"sale_id"`
	Number       string          `json:"number"`
	OccurredAt   time.Time       `json:"occurred_at"`
	Currency     string          `json:"currency"`
	RefundMethod string          `json:"refund_method"` // cash | credit_note | original_method
	PartyID      *uuid.UUID      `json:"party_id,omitempty"`
	Subtotal     float64         `json:"subtotal"`
	TaxTotal     float64         `json:"tax_total"`
	Total        float64         `json:"total"`
	Lines        []SaleEventLine `json:"lines"`
	Actor        string          `json:"actor,omitempty"`
}

// ReversalEvent pide stornear el asiento posteado de un documento (void de venta,
// SoftDelete de cobro, void de devolución). El worker ubica el asiento por
// (RefType, RefID, TargetEvent) y lo reversa; si no existe, no hace nada.
type ReversalEvent struct {
	OrgID       uuid.UUID `json:"org_id"`
	RefType     string    `json:"ref_type"`
	RefID       uuid.UUID `json:"ref_id"`
	TargetEvent string    `json:"target_event"`
	Actor       string    `json:"actor,omitempty"`
}

// PurchaseEvent dispara la reconciliación contable de una compra. Lleva sólo el
// id: el worker lee el estado actual de la compra y lleva el mayor a ese estado
// (alta si está 'received', storno si dejó de estarlo), por eso es re-armable y
// soporta el toggle received<->draft.
type PurchaseEvent struct {
	OrgID      uuid.UUID `json:"org_id"`
	PurchaseID uuid.UUID `json:"purchase_id"`
	Actor      string    `json:"actor,omitempty"`
}
