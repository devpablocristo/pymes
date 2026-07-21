package domain

import (
	"time"

	"github.com/google/uuid"
)

// FiscalSettings es la config fiscal del emisor (org). El certificado y su clave
// privada se guardan aparte (la clave, cifrada); en el dominio expuesto sólo se
// informa si hay certificado cargado, nunca el material sensible.
type FiscalSettings struct {
	OrgID              uuid.UUID `json:"org_id"`
	CUIT               string    `json:"cuit"`
	Environment        string    `json:"environment"` // homologation | production
	TaxCondition       string    `json:"tax_condition"`
	DefaultPointOfSale int       `json:"default_point_of_sale"`
	HasCertificate     bool      `json:"has_certificate"`
	Enabled            bool      `json:"enabled"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// IvaLine es una alícuota del comprobante (Id ARCA, base imponible, importe).
type IvaLine struct {
	ID      int     `json:"id"`
	BaseImp float64 `json:"base_imp"`
	Importe float64 `json:"importe"`
}

// Note es una observación o error de ARCA (código + mensaje).
type Note struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// FiscalVoucher es un comprobante fiscal (documento legal de una venta) con su CAE.
type FiscalVoucher struct {
	ID                   uuid.UUID  `json:"id"`
	OrgID                uuid.UUID  `json:"org_id"`
	SaleID               *uuid.UUID `json:"sale_id,omitempty"`
	ReturnID             *uuid.UUID `json:"return_id,omitempty"`
	AssociatedVoucherID  *uuid.UUID `json:"associated_voucher_id,omitempty"`
	VoucherType          int        `json:"voucher_type"`
	PointOfSale          int        `json:"point_of_sale"`
	CbteNro              int64      `json:"cbte_nro"`
	Concepto             int        `json:"concepto"`
	DocTipo              int        `json:"doc_tipo"`
	DocNro               string     `json:"doc_nro"`
	CondicionIVAReceptor int        `json:"condicion_iva_receptor"`
	Currency             string     `json:"currency"`
	ExchangeRate         float64    `json:"exchange_rate"`
	ImpNeto              float64    `json:"imp_neto"`
	ImpIVA               float64    `json:"imp_iva"`
	ImpTrib              float64    `json:"imp_trib"`
	ImpOpEx              float64    `json:"imp_op_ex"`
	ImpTotConc           float64    `json:"imp_tot_conc"`
	ImpTotal             float64    `json:"imp_total"`
	IvaBreakdown         []IvaLine  `json:"iva_breakdown"`
	CAE                  string     `json:"cae"`
	CAEVto               string     `json:"cae_vto"`
	QRURL                string     `json:"qr_url"`
	Status               string     `json:"status"` // pending|authorized|rejected|error
	AfipResult           string     `json:"afip_result"`
	Observations         []Note     `json:"observations,omitempty"`
	Errors               []Note     `json:"errors,omitempty"`
	EmittedAt            *time.Time `json:"emitted_at,omitempty"`
	CreatedBy            string     `json:"created_by,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

// AuthTicket es el TA (Ticket de Acceso) del WSAA cacheado por org+servicio.
type AuthTicket struct {
	Token     string    `json:"-"`
	Sign      string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Valid indica si el TA sigue vigente (con margen de seguridad).
func (t AuthTicket) Valid(now time.Time) bool {
	return t.Token != "" && t.ExpiresAt.After(now.Add(5*time.Minute))
}
