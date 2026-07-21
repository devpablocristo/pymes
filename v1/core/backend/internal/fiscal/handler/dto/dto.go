package dto

type SaveSettingsRequest struct {
	CUIT               string `json:"cuit,omitempty"`
	Environment        string `json:"environment,omitempty"` // homologation | production
	TaxCondition       string `json:"tax_condition,omitempty"`
	DefaultPointOfSale int    `json:"default_point_of_sale,omitempty"`
	Enabled            bool   `json:"enabled,omitempty"`
	// CertPEM + KeyPEM opcionales; si se envían, reemplazan el certificado.
	// La clave privada se guarda cifrada; nunca se devuelve.
	CertPEM string `json:"cert_pem,omitempty"`
	KeyPEM  string `json:"key_pem,omitempty"`
}

// EmitVoucherRequest emite el comprobante fiscal de una venta. voucher_type,
// point_of_sale y concepto en 0 se resuelven automáticamente. Para servicios
// (concepto 2/3) se pueden informar las fechas de servicio (YYYY-MM-DD);
// exchange_rate solo aplica a moneda extranjera.
type EmitVoucherRequest struct {
	SaleID       string  `json:"sale_id" binding:"required"`
	VoucherType  int     `json:"voucher_type,omitempty"`
	PointOfSale  int     `json:"point_of_sale,omitempty"`
	Concepto     int     `json:"concepto,omitempty"`
	ServiceFrom  string  `json:"service_from,omitempty"`
	ServiceTo    string  `json:"service_to,omitempty"`
	PaymentDue   string  `json:"payment_due,omitempty"`
	ExchangeRate float64 `json:"exchange_rate,omitempty"`
}

// EmitCreditNoteRequest emite la nota de crédito de una devolución.
type EmitCreditNoteRequest struct {
	ReturnID    string `json:"return_id" binding:"required"`
	PointOfSale int    `json:"point_of_sale,omitempty"`
}
