package models

import (
	"time"

	"github.com/google/uuid"
)

type PaymentGatewayConnectionModel struct {
	OrgID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	Provider              string    `gorm:"not null;default:mercadopago"`
	ExternalUserID        string    `gorm:"not null"`
	AccessTokenEncrypted  string    `gorm:"not null"`
	RefreshTokenEncrypted string    `gorm:"not null"`
	TokenExpiresAt        time.Time `gorm:"not null"`
	IsActive              bool      `gorm:"not null;default:true"`
	ConnectedAt           time.Time `gorm:"not null"`
	UpdatedAt             time.Time `gorm:"not null"`
}

func (PaymentGatewayConnectionModel) TableName() string { return "payment_gateway_connections" }

type PaymentPreferenceModel struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrgID           uuid.UUID `gorm:"type:uuid;index;not null"`
	Provider        string    `gorm:"not null;default:mercadopago"`
	ExternalID      string    `gorm:"not null;default:''"`
	ReferenceType   string    `gorm:"not null"`
	ReferenceID     uuid.UUID `gorm:"type:uuid;not null"`
	Amount          float64   `gorm:"type:numeric(15,2);not null"`
	Description     string    `gorm:"not null;default:''"`
	PaymentURL      string    `gorm:"not null;default:''"`
	QRData          string    `gorm:"not null;default:''"`
	Status          string    `gorm:"not null;default:pending"`
	ExternalPayerID string    `gorm:"not null;default:''"`
	PaidAt          *time.Time
	ExpiresAt       time.Time `gorm:"not null"`
	CreatedAt       time.Time `gorm:"not null"`
}

func (PaymentPreferenceModel) TableName() string { return "payment_preferences" }
