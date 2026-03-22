package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type TenantSettings struct {
	OrgID                    uuid.UUID      `json:"org_id"`
	PlanCode                 string         `json:"plan_code"`
	HardLimits               map[string]any `json:"hard_limits"`
	BillingStatus            string         `json:"billing_status"`
	StripeCustomerID         string         `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID     string         `json:"stripe_subscription_id,omitempty"`
	Currency                 string         `json:"currency"`
	SupportedCurrencies      []string       `json:"supported_currencies"`
	TaxRate                  float64        `json:"tax_rate"`
	QuotePrefix              string         `json:"quote_prefix"`
	SalePrefix               string         `json:"sale_prefix"`
	NextQuoteNumber          int            `json:"next_quote_number"`
	NextSaleNumber           int            `json:"next_sale_number"`
	AllowNegativeStock       bool           `json:"allow_negative_stock"`
	PurchasePrefix           string         `json:"purchase_prefix"`
	NextPurchaseNumber       int            `json:"next_purchase_number"`
	ReturnPrefix             string         `json:"return_prefix"`
	CreditNotePrefix         string         `json:"credit_note_prefix"`
	NextReturnNumber         int            `json:"next_return_number"`
	NextCreditNoteNumber     int            `json:"next_credit_note_number"`
	BusinessName             string         `json:"business_name"`
	BusinessTaxID            string         `json:"business_tax_id"`
	BusinessAddress          string         `json:"business_address"`
	BusinessPhone            string         `json:"business_phone"`
	BusinessEmail            string         `json:"business_email"`
	WAQuoteTemplate          string         `json:"wa_quote_template"`
	WAReceiptTemplate        string         `json:"wa_receipt_template"`
	WADefaultCountryCode     string         `json:"wa_default_country_code"`
	AppointmentsEnabled      bool           `json:"appointments_enabled"`
	AppointmentLabel         string         `json:"appointment_label"`
	AppointmentReminderHours int            `json:"appointment_reminder_hours"`
	SecondaryCurrency        string         `json:"secondary_currency"`
	DefaultRateType          string         `json:"default_rate_type"`
	AutoFetchRates           bool           `json:"auto_fetch_rates"`
	ShowDualPrices           bool           `json:"show_dual_prices"`
	BankHolder               string         `json:"bank_holder"`
	BankCBU                  string         `json:"bank_cbu"`
	BankAlias                string         `json:"bank_alias"`
	BankName                 string         `json:"bank_name"`
	ShowQRInPDF              bool           `json:"show_qr_in_pdf"`
	WAPaymentTemplate        string         `json:"wa_payment_template"`
	WAPaymentLinkTemplate    string         `json:"wa_payment_link_template"`
	UpdatedBy                *string        `json:"updated_by,omitempty"`
	UpdatedAt                time.Time      `json:"updated_at"`
}

type TenantSettingsPatch struct {
	PlanCode                 *string        `json:"plan_code,omitempty"`
	HardLimits               map[string]any `json:"hard_limits,omitempty"`
	Currency                 *string        `json:"currency,omitempty"`
	SupportedCurrencies      *[]string      `json:"supported_currencies,omitempty"`
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

// NormalizeSupportedCurrencies valida y normaliza códigos (3–8 caracteres alfanuméricos; máx. 16).
func NormalizeSupportedCurrencies(raw []string) ([]string, error) {
	if len(raw) > 16 {
		return nil, fmt.Errorf("supported_currencies: maximum 16 codes")
	}
	seen := make(map[string]bool)
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		c := strings.ToUpper(strings.TrimSpace(r))
		if c == "" {
			continue
		}
		n := utf8.RuneCountInString(c)
		if n < 3 || n > 8 {
			return nil, fmt.Errorf("supported_currencies: invalid code length for %q", r)
		}
		for _, ch := range c {
			if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) {
				return nil, fmt.Errorf("supported_currencies: invalid code %q", c)
			}
		}
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("supported_currencies: at least one currency required")
	}
	return out, nil
}

type ActivityEvent struct {
	ID           uuid.UUID      `json:"id"`
	OrgID        uuid.UUID      `json:"org_id"`
	Actor        string         `json:"actor,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}
