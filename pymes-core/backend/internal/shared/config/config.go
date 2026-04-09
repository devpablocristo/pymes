// Package config loads backend configuration from external environment variables.
package config

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/devpablocristo/core/config/go/envconfig"
)

const localInternalServiceToken = "local-internal-token"

// Config centraliza la configuracion externa para mantener el mismo codigo entre ambientes.
type Config struct {
	Port                        string
	Environment                 string
	DatabaseURL                 string
	JWKSURL                     string
	JWTIssuer                   string
	JWTAudience                 string
	JWTOrgClaim                 string
	JWTRoleClaim                string
	JWTScopesClaim              string
	JWTActorClaim               string
	AuthEnableJWT               bool
	AuthAllowAPIKey             bool
	ClerkWebhookSecret          string
	StripeSecretKey             string
	StripeWebhookSecret         string
	NotificationBackend         string
	FrontendURL                 string
	PublicBaseURL               string
	AWSRegion                   string
	AWSSesFromEmail             string
	SMTPHost                    string
	SMTPPort                    int
	SMTPUser                    string
	SMTPPassword                string
	StripePriceStarter          string
	StripePriceGrowth           string
	StripePriceEnterprise       string
	StorageBackend              string
	S3Bucket                    string
	S3Region                    string
	SchedulerSecret             string
	ExchangeRateProvider        string
	InternalServiceToken        string
	AIServiceURL                string
	ReviewCallbackToken         string
	ReviewSyncInterval          time.Duration
	WhatsAppWebhookVerifyToken  string
	WhatsAppAppSecret           string
	WhatsAppGraphAPIBaseURL     string
	MPAppID                     string
	MPClientSecret              string
	MPWebhookSecret             string
	MPRedirectURI               string
	PaymentGatewayMode          string
	PaymentGatewayEncryptionKey string
	GoogleOAuthClientID         string
	GoogleOAuthClientSecret     string
	GoogleOAuthRedirectURL      string
}

// LoadFromEnv carga valores con defaults seguros para desarrollo local.
func LoadFromEnv() Config {
	cfg := Config{
		Port:                        envconfig.Get("PORT", "8080"),
		Environment:                 envconfig.NormalizeEnv(envconfig.Get("ENVIRONMENT", "development")),
		DatabaseURL:                 envconfig.Get("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/pymes?sslmode=disable"),
		JWKSURL:                     envconfig.Get("JWKS_URL", ""),
		JWTIssuer:                   envconfig.Get("JWT_ISSUER", ""),
		JWTAudience:                 envconfig.Get("JWT_AUDIENCE", ""),
		JWTOrgClaim:                 envconfig.Get("JWT_ORG_CLAIM", ""),
		JWTRoleClaim:                envconfig.Get("JWT_ROLE_CLAIM", ""),
		JWTScopesClaim:              envconfig.Get("JWT_SCOPES_CLAIM", ""),
		JWTActorClaim:               envconfig.Get("JWT_ACTOR_CLAIM", ""),
		AuthEnableJWT:               envconfig.Bool("AUTH_ENABLE_JWT", true),
		AuthAllowAPIKey:             envconfig.Bool("AUTH_ALLOW_API_KEY", true),
		ClerkWebhookSecret:          envconfig.Get("CLERK_WEBHOOK_SECRET", ""),
		StripeSecretKey:             envconfig.Get("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret:         envconfig.Get("STRIPE_WEBHOOK_SECRET", ""),
		NotificationBackend:         envconfig.Get("NOTIFICATION_BACKEND", "noop"),
		FrontendURL:                 envconfig.Get("FRONTEND_URL", "http://localhost:5173"),
		PublicBaseURL:               envconfig.Get("PUBLIC_BASE_URL", "http://localhost:8080"),
		AWSRegion:                   envconfig.Get("AWS_REGION", "us-east-1"),
		AWSSesFromEmail:             envconfig.Get("AWS_SES_FROM_EMAIL", ""),
		SMTPHost:                    envconfig.Get("SMTP_HOST", "localhost"),
		SMTPPort:                    envconfig.Int("SMTP_PORT", 1025),
		SMTPUser:                    envconfig.Get("SMTP_USER", ""),
		SMTPPassword:                envconfig.Get("SMTP_PASSWORD", ""),
		StripePriceStarter:          envconfig.Get("STRIPE_PRICE_STARTER", ""),
		StripePriceGrowth:           envconfig.Get("STRIPE_PRICE_GROWTH", ""),
		StripePriceEnterprise:       envconfig.Get("STRIPE_PRICE_ENTERPRISE", ""),
		StorageBackend:              envconfig.Get("STORAGE_BACKEND", "local"),
		S3Bucket:                    envconfig.Get("S3_BUCKET", ""),
		S3Region:                    envconfig.Get("S3_REGION", "us-east-1"),
		SchedulerSecret:             envconfig.Get("SCHEDULER_SECRET", ""),
		ExchangeRateProvider:        envconfig.Get("EXCHANGE_RATE_PROVIDER", "manual"),
		InternalServiceToken:        strings.TrimSpace(envconfig.Get("INTERNAL_SERVICE_TOKEN", localInternalServiceToken)),
		AIServiceURL:                envconfig.Get("AI_SERVICE_URL", "http://ai:8000"),
		ReviewCallbackToken:         strings.TrimSpace(envconfig.Get("REVIEW_CALLBACK_TOKEN", "")),
		ReviewSyncInterval:          envconfig.Duration("REVIEW_SYNC_INTERVAL_SECONDS", 30*time.Second),
		WhatsAppWebhookVerifyToken:  envconfig.Get("WHATSAPP_WEBHOOK_VERIFY_TOKEN", ""),
		WhatsAppAppSecret:           envconfig.Get("WHATSAPP_APP_SECRET", ""),
		WhatsAppGraphAPIBaseURL:     envconfig.Get("WHATSAPP_GRAPH_API_BASE_URL", "https://graph.facebook.com/v23.0"),
		MPAppID:                     envconfig.Get("MP_APP_ID", ""),
		MPClientSecret:              envconfig.Get("MP_CLIENT_SECRET", ""),
		MPWebhookSecret:             envconfig.Get("MP_WEBHOOK_SECRET", ""),
		MPRedirectURI:               envconfig.Get("MP_REDIRECT_URI", ""),
		PaymentGatewayMode:          envconfig.Get("PAYMENT_GATEWAY_MODE", "mercadopago"),
		PaymentGatewayEncryptionKey: envconfig.Get("PAYMENT_GATEWAY_ENCRYPTION_KEY", ""),
		GoogleOAuthClientID:         envconfig.Get("GOOGLE_OAUTH_CLIENT_ID", ""),
		GoogleOAuthClientSecret:     envconfig.Get("GOOGLE_OAUTH_CLIENT_SECRET", ""),
		GoogleOAuthRedirectURL:      envconfig.Get("GOOGLE_OAUTH_REDIRECT_URL", ""),
	}
	if err := validateInternalServiceToken(cfg.Environment, cfg.InternalServiceToken); err != nil {
		log.Fatal(err)
	}
	return cfg
}

func validateInternalServiceToken(environment, token string) error {
	normalizedToken := strings.TrimSpace(token)
	if envconfig.IsLocal(environment) {
		return nil
	}
	if normalizedToken == "" || strings.EqualFold(normalizedToken, localInternalServiceToken) {
		return fmt.Errorf("invalid INTERNAL_SERVICE_TOKEN for %s environment", envconfig.NormalizeEnv(environment))
	}
	return nil
}
