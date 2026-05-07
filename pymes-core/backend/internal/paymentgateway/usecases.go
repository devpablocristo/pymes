// Package paymentgateway contains payment gateway business logic and provider orchestration.
package paymentgateway

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	auditdomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/audit/usecases/domain"
	"github.com/devpablocristo/pymes/pymes-core/backend/internal/paymentgateway/gateway"
	gatewaydomain "github.com/devpablocristo/pymes/pymes-core/backend/internal/paymentgateway/usecases/domain"
)

const (
	providerMercadoPago     = "mercadopago"
	mpOAuthBaseURL          = "https://auth.mercadopago.com/authorization"
	mpPreferenceTTL         = 72 * time.Hour
	mpOAuthStateTTL         = 45 * time.Minute
	growthMonthlyLinksLimit = 50
	mpWebhookServiceName    = "mercadopago_webhook"
)

var (
	ErrPlanRestricted          = errors.New("mercadopago no esta disponible para tu plan")
	ErrPlanMonthlyLimitReached = errors.New("alcanzaste el limite mensual de links de pago")
	ErrInvalidOAuthState       = errors.New("estado de oauth invalido")
	ErrInvalidWebhookSignature = errors.New("firma de webhook invalida")
	ErrUnsupportedProvider     = errors.New("provider no soportado")
	ErrInvalidReference        = errors.New("referencia invalida")
	ErrBankAliasMissing        = errors.New("configura tu alias en ajustes")
	ErrGatewayConfigMissing    = errors.New("configuracion de mercadopago incompleta")
)

type repositoryPort interface {
	ResolveTenantID(ctx context.Context, ref string) (uuid.UUID, error)
	GetPlanCode(ctx context.Context, tenantID uuid.UUID) string
	GetBankInfo(ctx context.Context, tenantID uuid.UUID) (gatewaydomain.BankInfo, bool, error)
	GetWhatsAppTransferTemplate(ctx context.Context, tenantID uuid.UUID) string
	GetWhatsAppLinkTemplate(ctx context.Context, tenantID uuid.UUID) string

	GetConnection(ctx context.Context, tenantID uuid.UUID) (gatewaydomain.PaymentGatewayConnection, error)
	GetConnectionByExternalUserID(ctx context.Context, externalUserID string) (gatewaydomain.PaymentGatewayConnection, error)
	GetServiceIDByName(ctx context.Context, name string) (uuid.UUID, error)
	ListActiveConnections(ctx context.Context) ([]gatewaydomain.PaymentGatewayConnection, error)
	SaveConnection(ctx context.Context, in gatewaydomain.PaymentGatewayConnection) error
	Disconnect(ctx context.Context, tenantID uuid.UUID) error

	CountMonthlyPreferences(ctx context.Context, tenantID uuid.UUID, since time.Time) (int64, error)
	SavePreference(ctx context.Context, in gatewaydomain.PaymentPreference) (gatewaydomain.PaymentPreference, error)
	GetLatestPreference(ctx context.Context, tenantID uuid.UUID, refType string, refID uuid.UUID) (gatewaydomain.PaymentPreference, error)
	GetPreferenceByExternalID(ctx context.Context, provider, externalID string) (gatewaydomain.PaymentPreference, error)

	GetSaleSnapshot(ctx context.Context, tenantID, saleID uuid.UUID) (gatewaydomain.SaleSnapshot, error)
	GetQuoteSnapshot(ctx context.Context, tenantID, quoteID uuid.UUID) (gatewaydomain.QuoteSnapshot, error)
	ProcessApprovedSalePayment(ctx context.Context, in ProcessSalePaymentInput) error
	MarkPreferenceApproved(ctx context.Context, tenantID uuid.UUID, refType string, refID uuid.UUID, payerID string, paidAt time.Time) error
	StoreWebhookEvent(ctx context.Context, in gatewaydomain.WebhookEvent) error
	LockPendingWebhookEvents(ctx context.Context, limit int) ([]gatewaydomain.WebhookEvent, error)
	MarkWebhookEventProcessed(ctx context.Context, id uuid.UUID) error
	MarkWebhookEventError(ctx context.Context, id uuid.UUID, errorMessage string) error
}

type mercadoPagoPort interface {
	ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (gateway.OAuthTokens, error)
	RefreshToken(ctx context.Context, clientID, clientSecret, refreshToken string) (gateway.OAuthTokens, error)
	CreatePreference(ctx context.Context, accessToken string, in gateway.PreferenceInput) (gateway.PreferenceOutput, error)
	GetPaymentDetail(ctx context.Context, accessToken, paymentID string) (gateway.PaymentDetail, error)
}

type auditPort interface {
	LogWithActor(ctx context.Context, in auditdomain.LogInput)
}

type Usecases struct {
	repo            repositoryPort
	mp              mercadoPagoPort
	audit           auditPort
	crypto          *Crypto
	mode            string
	mpAppID         string
	mpClientSecret  string
	mpWebhookSecret string
	mpRedirectURI   string
	frontendURL     string
	now             func() time.Time
}

func NewUsecases(
	repo repositoryPort,
	mp mercadoPagoPort,
	audit auditPort,
	crypto *Crypto,
	mode string,
	mpAppID string,
	mpClientSecret string,
	mpWebhookSecret string,
	mpRedirectURI string,
	frontendURL string,
) *Usecases {
	return &Usecases{
		repo:            repo,
		mp:              mp,
		audit:           audit,
		crypto:          crypto,
		mode:            normalizeGatewayMode(mode),
		mpAppID:         strings.TrimSpace(mpAppID),
		mpClientSecret:  strings.TrimSpace(mpClientSecret),
		mpWebhookSecret: strings.TrimSpace(mpWebhookSecret),
		mpRedirectURI:   strings.TrimSpace(mpRedirectURI),
		frontendURL:     strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
		now:             func() time.Time { return time.Now().UTC() },
	}
}

type CreatePreferenceRequest struct {
	ReferenceType string
	ReferenceID   uuid.UUID
}

type WhatsAppResult struct {
	URL     string
	Message string
}
