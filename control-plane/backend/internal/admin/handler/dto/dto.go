package dto

type UpdateTenantSettingsRequest struct {
	PlanCode                 *string        `json:"plan_code,omitempty"`
	HardLimits               map[string]any `json:"hard_limits,omitempty"`
	Currency                 *string        `json:"currency,omitempty"`
	TaxRate                  *float64       `json:"tax_rate,omitempty"`
	QuotePrefix              *string        `json:"quote_prefix,omitempty"`
	SalePrefix               *string        `json:"sale_prefix,omitempty"`
	AllowNegativeStock       *bool          `json:"allow_negative_stock,omitempty"`
	PurchasePrefix           *string        `json:"purchase_prefix,omitempty"`
	ReturnPrefix             *string        `json:"return_prefix,omitempty"`
	CreditNotePrefix         *string        `json:"credit_note_prefix,omitempty"`
	BusinessName             *string        `json:"business_name,omitempty"`
	BusinessTaxID            *string        `json:"business_tax_id,omitempty"`
	BusinessAddress          *string        `json:"business_address,omitempty"`
	BusinessPhone            *string        `json:"business_phone,omitempty"`
	BusinessEmail            *string        `json:"business_email,omitempty"`
	WAQuoteTemplate          *string        `json:"wa_quote_template,omitempty"`
	WAReceiptTemplate        *string        `json:"wa_receipt_template,omitempty"`
	WADefaultCountryCode     *string        `json:"wa_default_country_code,omitempty"`
	AppointmentsEnabled      *bool          `json:"appointments_enabled,omitempty"`
	AppointmentLabel         *string        `json:"appointment_label,omitempty"`
	AppointmentReminderHours *int           `json:"appointment_reminder_hours,omitempty"`
	SecondaryCurrency        *string        `json:"secondary_currency,omitempty"`
	DefaultRateType          *string        `json:"default_rate_type,omitempty"`
	AutoFetchRates           *bool          `json:"auto_fetch_rates,omitempty"`
	ShowDualPrices           *bool          `json:"show_dual_prices,omitempty"`
	BankHolder               *string        `json:"bank_holder,omitempty"`
	BankCBU                  *string        `json:"bank_cbu,omitempty"`
	BankAlias                *string        `json:"bank_alias,omitempty"`
	BankName                 *string        `json:"bank_name,omitempty"`
	ShowQRInPDF              *bool          `json:"show_qr_in_pdf,omitempty"`
	WAPaymentTemplate        *string        `json:"wa_payment_template,omitempty"`
	WAPaymentLinkTemplate    *string        `json:"wa_payment_link_template,omitempty"`
}
