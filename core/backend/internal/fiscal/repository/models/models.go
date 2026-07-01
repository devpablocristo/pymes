package models

import (
	"time"

	"github.com/google/uuid"
)

type SettingsModel struct {
	OrgID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	CUIT               string    `gorm:"column:cuit;not null;default:''"`
	Environment        string    `gorm:"not null;default:homologation"`
	TaxCondition       string    `gorm:"not null;default:''"`
	CertPEM            string    `gorm:"column:cert_pem;not null;default:''"`
	KeyEncrypted       string    `gorm:"column:key_encrypted;not null;default:''"`
	DefaultPointOfSale int       `gorm:"not null;default:1"`
	Enabled            bool      `gorm:"not null;default:false"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (SettingsModel) TableName() string { return "fiscal_settings" }

type AuthTicketModel struct {
	OrgID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	Service   string    `gorm:"primaryKey"`
	Token     string    `gorm:"not null"`
	Sign      string    `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null"`
	UpdatedAt time.Time
}

func (AuthTicketModel) TableName() string { return "fiscal_auth_tickets" }

type VoucherModel struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey"`
	OrgID                uuid.UUID  `gorm:"type:uuid;index;not null"`
	SaleID               *uuid.UUID `gorm:"type:uuid"`
	ReturnID             *uuid.UUID `gorm:"type:uuid"`
	AssociatedVoucherID  *uuid.UUID `gorm:"type:uuid"`
	VoucherType          int
	PointOfSale          int
	CbteNro              int64
	Concepto             int
	DocTipo              int
	DocNro               string
	CondicionIvaReceptor *int `gorm:"column:condicion_iva_receptor"`
	Currency             string
	ExchangeRate         float64
	ImpNeto              float64
	ImpIVA               float64 `gorm:"column:imp_iva"`
	ImpTrib              float64
	ImpOpEx              float64
	ImpTotConc           float64
	ImpTotal             float64
	IvaBreakdown         []byte     `gorm:"column:iva_breakdown;type:jsonb"`
	CAE                  string     `gorm:"column:cae"`
	CAEVto               *time.Time `gorm:"column:cae_vto;type:date"`
	QRURL                string     `gorm:"column:qr_url"`
	Status               string
	AfipResult           string `gorm:"column:afip_result"`
	Observations         []byte `gorm:"type:jsonb"`
	Errors               []byte `gorm:"type:jsonb"`
	EmittedAt            *time.Time
	CreatedBy            string
	CreatedAt            time.Time
}

func (VoucherModel) TableName() string { return "fiscal_vouchers" }
