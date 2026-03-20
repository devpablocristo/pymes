package admin

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/devpablocristo/pymes/pymes-core/backend/internal/admin/repository/models"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/admin/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetTenantSettings(orgID uuid.UUID) domain.TenantSettings {
	var m models.TenantSettingsModel
	if err := r.db.Where("org_id = ?", orgID).First(&m).Error; err != nil {
		return tenantSettingsToDomain(defaultTenantSettingsModel(orgID))
	}
	return tenantSettingsToDomain(normalizeTenantSettingsModel(m))
}

func (r *Repository) UpdateTenantSettings(orgID uuid.UUID, patch domain.TenantSettingsPatch, actor *string) domain.TenantSettings {
	var m models.TenantSettingsModel
	result := r.db.Where("org_id = ?", orgID).First(&m)
	if result.Error != nil {
		m = defaultTenantSettingsModel(orgID)
	} else {
		m = normalizeTenantSettingsModel(m)
	}

	planChanged := false
	if patch.PlanCode != nil {
		m.PlanCode = defaultString(*patch.PlanCode, "starter")
		planChanged = true
	}
	if patch.HardLimits != nil {
		m.HardLimits = mustJSON(patch.HardLimits)
	} else if planChanged {
		m.HardLimits = mustJSON(DefaultHardLimits(m.PlanCode))
	}
	applyString(&m.Currency, patch.Currency)
	applyFloat(&m.TaxRate, patch.TaxRate)
	applyString(&m.QuotePrefix, patch.QuotePrefix)
	applyString(&m.SalePrefix, patch.SalePrefix)
	applyBool(&m.AllowNegativeStock, patch.AllowNegativeStock)
	applyString(&m.PurchasePrefix, patch.PurchasePrefix)
	applyString(&m.ReturnPrefix, patch.ReturnPrefix)
	applyString(&m.CreditNotePrefix, patch.CreditNotePrefix)
	applyString(&m.BusinessName, patch.BusinessName)
	applyString(&m.BusinessTaxID, patch.BusinessTaxID)
	applyString(&m.BusinessAddress, patch.BusinessAddress)
	applyString(&m.BusinessPhone, patch.BusinessPhone)
	applyString(&m.BusinessEmail, patch.BusinessEmail)
	applyString(&m.WAQuoteTemplate, patch.WAQuoteTemplate)
	applyString(&m.WAReceiptTemplate, patch.WAReceiptTemplate)
	applyString(&m.WADefaultCountryCode, patch.WADefaultCountryCode)
	applyBool(&m.AppointmentsEnabled, patch.AppointmentsEnabled)
	applyString(&m.AppointmentLabel, patch.AppointmentLabel)
	applyInt(&m.AppointmentReminderHours, patch.AppointmentReminderHours)
	applyString(&m.SecondaryCurrency, patch.SecondaryCurrency)
	applyString(&m.DefaultRateType, patch.DefaultRateType)
	applyBool(&m.AutoFetchRates, patch.AutoFetchRates)
	applyBool(&m.ShowDualPrices, patch.ShowDualPrices)
	applyString(&m.BankHolder, patch.BankHolder)
	applyString(&m.BankCBU, patch.BankCBU)
	applyString(&m.BankAlias, patch.BankAlias)
	applyString(&m.BankName, patch.BankName)
	applyBool(&m.ShowQRInPDF, patch.ShowQRInPDF)
	applyString(&m.WAPaymentTemplate, patch.WAPaymentTemplate)
	applyString(&m.WAPaymentLinkTemplate, patch.WAPaymentLinkTemplate)

	m = normalizeTenantSettingsModel(m)
	m.UpdatedBy = actor
	m.UpdatedAt = time.Now().UTC()

	if result.Error != nil {
		r.db.Create(&m)
	} else {
		r.db.Save(&m)
	}

	return tenantSettingsToDomain(m)
}

func (r *Repository) ListActivity(orgID uuid.UUID, limit int) []domain.ActivityEvent {
	if limit <= 0 {
		limit = 200
	}
	var rows []models.AdminActivityEventModel
	r.db.Where("org_id = ?", orgID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows)

	result := make([]domain.ActivityEvent, 0, len(rows))
	for _, row := range rows {
		result = append(result, activityToDomain(row))
	}
	return result
}

func tenantSettingsToDomain(m models.TenantSettingsModel) domain.TenantSettings {
	m = normalizeTenantSettingsModel(m)
	var limits map[string]any
	if len(m.HardLimits) > 0 {
		_ = json.Unmarshal(m.HardLimits, &limits)
	}
	if limits == nil {
		limits = DefaultHardLimits(m.PlanCode)
	}

	stripeCustomerID := ""
	if m.StripeCustomerID != nil {
		stripeCustomerID = strings.TrimSpace(*m.StripeCustomerID)
	}
	stripeSubscriptionID := ""
	if m.StripeSubscriptionID != nil {
		stripeSubscriptionID = strings.TrimSpace(*m.StripeSubscriptionID)
	}

	return domain.TenantSettings{
		OrgID:                    m.OrgID,
		PlanCode:                 m.PlanCode,
		HardLimits:               limits,
		BillingStatus:            m.BillingStatus,
		StripeCustomerID:         stripeCustomerID,
		StripeSubscriptionID:     stripeSubscriptionID,
		Currency:                 m.Currency,
		TaxRate:                  m.TaxRate,
		QuotePrefix:              m.QuotePrefix,
		SalePrefix:               m.SalePrefix,
		NextQuoteNumber:          m.NextQuoteNumber,
		NextSaleNumber:           m.NextSaleNumber,
		AllowNegativeStock:       m.AllowNegativeStock,
		PurchasePrefix:           m.PurchasePrefix,
		NextPurchaseNumber:       m.NextPurchaseNumber,
		ReturnPrefix:             m.ReturnPrefix,
		CreditNotePrefix:         m.CreditNotePrefix,
		NextReturnNumber:         m.NextReturnNumber,
		NextCreditNoteNumber:     m.NextCreditNoteNumber,
		BusinessName:             m.BusinessName,
		BusinessTaxID:            m.BusinessTaxID,
		BusinessAddress:          m.BusinessAddress,
		BusinessPhone:            m.BusinessPhone,
		BusinessEmail:            m.BusinessEmail,
		WAQuoteTemplate:          m.WAQuoteTemplate,
		WAReceiptTemplate:        m.WAReceiptTemplate,
		WADefaultCountryCode:     m.WADefaultCountryCode,
		AppointmentsEnabled:      m.AppointmentsEnabled,
		AppointmentLabel:         m.AppointmentLabel,
		AppointmentReminderHours: m.AppointmentReminderHours,
		SecondaryCurrency:        m.SecondaryCurrency,
		DefaultRateType:          m.DefaultRateType,
		AutoFetchRates:           m.AutoFetchRates,
		ShowDualPrices:           m.ShowDualPrices,
		BankHolder:               m.BankHolder,
		BankCBU:                  m.BankCBU,
		BankAlias:                m.BankAlias,
		BankName:                 m.BankName,
		ShowQRInPDF:              m.ShowQRInPDF,
		WAPaymentTemplate:        m.WAPaymentTemplate,
		WAPaymentLinkTemplate:    m.WAPaymentLinkTemplate,
		UpdatedBy:                m.UpdatedBy,
		UpdatedAt:                m.UpdatedAt,
	}
}

func activityToDomain(m models.AdminActivityEventModel) domain.ActivityEvent {
	var payload map[string]any
	if len(m.Payload) > 0 {
		_ = json.Unmarshal(m.Payload, &payload)
	}
	return domain.ActivityEvent{
		ID:           m.ID,
		OrgID:        m.OrgID,
		Actor:        m.Actor,
		Action:       m.Action,
		ResourceType: m.ResourceType,
		ResourceID:   m.ResourceID,
		Payload:      payload,
		CreatedAt:    m.CreatedAt,
	}
}

func DefaultHardLimits(plan string) map[string]any {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case "growth":
		return map[string]any{"users_max": 25, "storage_mb": 5000, "api_calls_rpm": 500}
	case "enterprise":
		return map[string]any{"users_max": "unlimited", "storage_mb": 50000, "api_calls_rpm": 2000}
	default:
		return map[string]any{"users_max": 5, "storage_mb": 500, "api_calls_rpm": 100}
	}
}

func defaultTenantSettingsModel(orgID uuid.UUID) models.TenantSettingsModel {
	return normalizeTenantSettingsModel(models.TenantSettingsModel{
		OrgID: orgID,
		HardLimits: mustJSON(DefaultHardLimits("starter")),
	})
}

func normalizeTenantSettingsModel(in models.TenantSettingsModel) models.TenantSettingsModel {
	out := in
	out.PlanCode = defaultString(out.PlanCode, "starter")
	if len(out.HardLimits) == 0 {
		out.HardLimits = mustJSON(DefaultHardLimits(out.PlanCode))
	}
	out.BillingStatus = defaultString(out.BillingStatus, "trialing")
	out.Currency = defaultString(out.Currency, "ARS")
	if out.TaxRate <= 0 {
		out.TaxRate = 21
	}
	out.QuotePrefix = defaultString(out.QuotePrefix, "PRE")
	out.SalePrefix = defaultString(out.SalePrefix, "VTA")
	if out.NextQuoteNumber <= 0 {
		out.NextQuoteNumber = 1
	}
	if out.NextSaleNumber <= 0 {
		out.NextSaleNumber = 1
	}
	out.PurchasePrefix = defaultString(out.PurchasePrefix, "CPA")
	if out.NextPurchaseNumber <= 0 {
		out.NextPurchaseNumber = 1
	}
	out.ReturnPrefix = defaultString(out.ReturnPrefix, "DEV")
	out.CreditNotePrefix = defaultString(out.CreditNotePrefix, "NC")
	if out.NextReturnNumber <= 0 {
		out.NextReturnNumber = 1
	}
	if out.NextCreditNoteNumber <= 0 {
		out.NextCreditNoteNumber = 1
	}
	out.WAQuoteTemplate = defaultString(out.WAQuoteTemplate, "Hola {customer_name}, te enviamos el presupuesto {number} por {total}.")
	out.WAReceiptTemplate = defaultString(out.WAReceiptTemplate, "Hola {customer_name}, tu comprobante de compra {number} por {total}. Gracias por tu compra!")
	out.WADefaultCountryCode = defaultString(out.WADefaultCountryCode, "54")
	out.AppointmentLabel = defaultString(out.AppointmentLabel, "Turno")
	if out.AppointmentReminderHours < 0 {
		out.AppointmentReminderHours = 24
	}
	out.DefaultRateType = defaultString(out.DefaultRateType, "blue")
	out.WAPaymentTemplate = defaultString(out.WAPaymentTemplate, "Podes transferir a:\nAlias: {bank_alias}\nCBU: {bank_cbu}\nTitular: {bank_holder}\nBanco: {bank_name}\nMonto: {total}")
	out.WAPaymentLinkTemplate = defaultString(out.WAPaymentLinkTemplate, "Hola {customer_name}, podes pagar {total} de tu compra {number} con este link: {payment_url}")
	return out
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func defaultString(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}

func applyString(dst *string, src *string) {
	if src != nil {
		*dst = strings.TrimSpace(*src)
	}
}

func applyFloat(dst *float64, src *float64) {
	if src != nil {
		*dst = *src
	}
}

func applyInt(dst *int, src *int) {
	if src != nil {
		*dst = *src
	}
}

func applyBool(dst *bool, src *bool) {
	if src != nil {
		*dst = *src
	}
}
