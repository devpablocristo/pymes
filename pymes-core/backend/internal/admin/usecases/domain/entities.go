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
	TeamSize                 string         `json:"team_size"`
	Sells                    string         `json:"sells"`
	ClientLabel              string         `json:"client_label"`
	UsesBilling              bool           `json:"uses_billing"`
	PaymentMethod            string         `json:"payment_method"`
	Vertical                 string         `json:"vertical"`
	OnboardingCompletedAt    *time.Time     `json:"onboarding_completed_at,omitempty"`
	WAQuoteTemplate          string         `json:"wa_quote_template"`
	WAReceiptTemplate        string         `json:"wa_receipt_template"`
	WADefaultCountryCode     string         `json:"wa_default_country_code"`
	SchedulingEnabled        bool           `json:"scheduling_enabled"`
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
	TeamSize                 *string        `json:"team_size,omitempty"`
	Sells                    *string        `json:"sells,omitempty"`
	ClientLabel              *string        `json:"client_label,omitempty"`
	UsesBilling              *bool          `json:"uses_billing,omitempty"`
	PaymentMethod            *string        `json:"payment_method,omitempty"`
	Vertical                 *string        `json:"vertical,omitempty"`
	OnboardingCompletedAt    *time.Time     `json:"onboarding_completed_at,omitempty"`
	WAQuoteTemplate          *string        `json:"wa_quote_template,omitempty"`
	WAReceiptTemplate        *string        `json:"wa_receipt_template,omitempty"`
	WADefaultCountryCode     *string        `json:"wa_default_country_code,omitempty"`
	SchedulingEnabled        *bool          `json:"scheduling_enabled,omitempty"`
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

var validVerticals = map[string]struct{}{
	"none":          {},
	"professionals": {},
	"workshops":     {},
	"bike_shop":     {},
	"beauty":        {},
	"restaurants":   {},
}

var validTeamSizes = map[string]struct{}{
	"solo":   {},
	"small":  {},
	"medium": {},
	"large":  {},
}

var validSells = map[string]struct{}{
	"products": {},
	"services": {},
	"both":     {},
	"unsure":   {},
}

var validPaymentMethods = map[string]struct{}{
	"cash":     {},
	"transfer": {},
	"card":     {},
	"mixed":    {},
}

func NormalizeEnum(raw string, allowed map[string]struct{}, field string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", nil
	}
	if _, ok := allowed[value]; !ok {
		return "", fmt.Errorf("%s: invalid value %q", field, raw)
	}
	return value, nil
}

func NormalizeClientLabel(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if utf8.RuneCountInString(value) > 80 {
		return "", fmt.Errorf("client_label: maximum 80 characters")
	}
	return value, nil
}

func NormalizeVertical(raw string) (string, error) {
	return NormalizeEnum(raw, validVerticals, "vertical")
}

func NormalizeTeamSize(raw string) (string, error) {
	return NormalizeEnum(raw, validTeamSizes, "team_size")
}

func NormalizeSells(raw string) (string, error) {
	return NormalizeEnum(raw, validSells, "sells")
}

func NormalizePaymentMethod(raw string) (string, error) {
	return NormalizeEnum(raw, validPaymentMethods, "payment_method")
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
