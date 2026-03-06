package domain

import (
	"time"

	"github.com/google/uuid"
)

type PaymentGatewayConnection struct {
	OrgID          uuid.UUID
	Provider       string
	ExternalUserID string
	AccessToken    string
	RefreshToken   string
	TokenExpiresAt time.Time
	IsActive       bool
	ConnectedAt    time.Time
	UpdatedAt      time.Time
}

type PaymentPreference struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	Provider        string
	ExternalID      string
	ReferenceType   string
	ReferenceID     uuid.UUID
	Amount          float64
	Description     string
	PaymentURL      string
	QRData          string
	Status          string
	ExternalPayerID string
	PaidAt          *time.Time
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

type ConnectionStatus struct {
	Connected      bool
	Provider       string
	ExternalUserID string
	TokenExpiresAt *time.Time
	ConnectedAt    *time.Time
}

type BankInfo struct {
	Holder string
	CBU    string
	Alias  string
	Name   string
}

type SaleSnapshot struct {
	ID            uuid.UUID
	Number        string
	CustomerName  string
	CustomerPhone string
	Total         float64
	Currency      string
}

type QuoteSnapshot struct {
	ID           uuid.UUID
	Number       string
	CustomerName string
	Total        float64
	Currency     string
}
