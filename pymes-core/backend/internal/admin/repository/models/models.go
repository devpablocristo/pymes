package models

import (
	"time"

	"github.com/google/uuid"
)

type TenantSettingsModel struct {
	OrgID                    uuid.UUID `gorm:"type:uuid;primaryKey"`
	PlanCode                 string    `gorm:"not null;default:starter"`
	HardLimits               []byte    `gorm:"type:jsonb"`
	StripeCustomerID         *string
	StripeSubscriptionID     *string
	BillingStatus            string  `gorm:"not null;default:trialing"`
	Currency                 string  `gorm:"not null;default:ARS"`
	SupportedCurrencies      []byte  `gorm:"type:jsonb;not null"`
	TaxRate                  float64 `gorm:"not null;default:21"`
	QuotePrefix              string  `gorm:"not null;default:PRE"`
	SalePrefix               string  `gorm:"not null;default:VTA"`
	NextQuoteNumber          int     `gorm:"not null;default:1"`
	NextSaleNumber           int     `gorm:"not null;default:1"`
	AllowNegativeStock       bool    `gorm:"not null;default:true"`
	PurchasePrefix           string  `gorm:"not null;default:CPA"`
	NextPurchaseNumber       int     `gorm:"not null;default:1"`
	ReturnPrefix             string  `gorm:"not null;default:DEV"`
	CreditNotePrefix         string  `gorm:"not null;default:NC"`
	NextReturnNumber         int     `gorm:"not null;default:1"`
	NextCreditNoteNumber     int     `gorm:"not null;default:1"`
	BusinessName             string  `gorm:"not null;default:''"`
	BusinessTaxID            string  `gorm:"not null;default:''"`
	BusinessAddress          string  `gorm:"not null;default:''"`
	BusinessPhone            string  `gorm:"not null;default:''"`
	BusinessEmail            string  `gorm:"not null;default:''"`
	TeamSize                 string  `gorm:"not null;default:''"`
	Sells                    string  `gorm:"not null;default:''"`
	ClientLabel              string  `gorm:"not null;default:''"`
	UsesBilling              bool    `gorm:"not null;default:false"`
	PaymentMethod            string  `gorm:"not null;default:''"`
	Vertical                 string  `gorm:"not null;default:''"`
	OnboardingCompletedAt    *time.Time
	WAQuoteTemplate          string `gorm:"not null;default:''"`
	WAReceiptTemplate        string `gorm:"not null;default:''"`
	WADefaultCountryCode     string `gorm:"not null;default:'54'"`
	SchedulingEnabled      bool   `gorm:"column:scheduling_enabled;not null;default:false"`
	SchedulingLabel        string `gorm:"column:scheduling_label;not null;default:'Turno'"`
	SchedulingReminderHours int   `gorm:"column:scheduling_reminder_hours;not null;default:24"`
	SecondaryCurrency        string `gorm:"not null;default:''"`
	DefaultRateType          string `gorm:"not null;default:'blue'"`
	AutoFetchRates           bool   `gorm:"not null;default:false"`
	ShowDualPrices           bool   `gorm:"not null;default:false"`
	BankHolder               string `gorm:"not null;default:''"`
	BankCBU                  string `gorm:"not null;default:''"`
	BankAlias                string `gorm:"not null;default:''"`
	BankName                 string `gorm:"not null;default:''"`
	ShowQRInPDF              bool   `gorm:"not null;default:false"`
	WAPaymentTemplate        string `gorm:"not null;default:''"`
	WAPaymentLinkTemplate    string `gorm:"not null;default:''"`
	UpdatedBy                *string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

func (TenantSettingsModel) TableName() string { return "tenant_settings" }

type AdminActivityEventModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID        uuid.UUID `gorm:"type:uuid;index;not null"`
	Actor        string
	Action       string
	ResourceType string
	ResourceID   string
	Payload      []byte `gorm:"type:jsonb"`
	CreatedAt    time.Time
}

func (AdminActivityEventModel) TableName() string { return "admin_activity_events" }
