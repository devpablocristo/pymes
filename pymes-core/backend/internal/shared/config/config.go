// Package config loads backend configuration from external environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	WhatsAppWebhookVerifyToken  string
	WhatsAppAppSecret           string
	WhatsAppGraphAPIBaseURL     string
	MPAppID                     string
	MPClientSecret              string
	MPWebhookSecret             string
	MPRedirectURI               string
	PaymentGatewayMode          string
	PaymentGatewayEncryptionKey string
	// SeedDemoData aplica SQL embebido en backend/seeds/ tras migrar (solo desarrollo / compose).
	SeedDemoData bool
	// SeedDemoOrgExternalID: si está definido (ej. org_xxx de Clerk), los seeds usan esa fila en orgs.external_id
	// y no crean la org local fija. La fila debe existir antes del arranque (login o webhook).
	SeedDemoOrgExternalID string
}

// LoadFromEnv carga valores con defaults seguros para desarrollo local.
func LoadFromEnv() Config {
	cfg := Config{
		Port:                        getEnv("PORT", "8080"),
		Environment:                 normalizeEnvironment(getEnv("ENVIRONMENT", "development")),
		DatabaseURL:                 getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/pymes?sslmode=disable"),
		JWKSURL:                     os.Getenv("JWKS_URL"),
		JWTIssuer:                   os.Getenv("JWT_ISSUER"),
		JWTAudience:                 os.Getenv("JWT_AUDIENCE"),
		JWTOrgClaim:                 os.Getenv("JWT_ORG_CLAIM"),
		JWTRoleClaim:                os.Getenv("JWT_ROLE_CLAIM"),
		JWTScopesClaim:              os.Getenv("JWT_SCOPES_CLAIM"),
		JWTActorClaim:               os.Getenv("JWT_ACTOR_CLAIM"),
		AuthEnableJWT:               getEnvBool("AUTH_ENABLE_JWT", true),
		AuthAllowAPIKey:             getEnvBool("AUTH_ALLOW_API_KEY", true),
		ClerkWebhookSecret:          os.Getenv("CLERK_WEBHOOK_SECRET"),
		StripeSecretKey:             os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret:         os.Getenv("STRIPE_WEBHOOK_SECRET"),
		NotificationBackend:         getEnv("NOTIFICATION_BACKEND", "noop"),
		FrontendURL:                 getEnv("FRONTEND_URL", "http://localhost:5173"),
		AWSRegion:                   getEnv("AWS_REGION", "us-east-1"),
		AWSSesFromEmail:             os.Getenv("AWS_SES_FROM_EMAIL"),
		SMTPHost:                    getEnv("SMTP_HOST", "localhost"),
		SMTPPort:                    getEnvInt("SMTP_PORT", 1025),
		SMTPUser:                    os.Getenv("SMTP_USER"),
		SMTPPassword:                os.Getenv("SMTP_PASSWORD"),
		StripePriceStarter:          os.Getenv("STRIPE_PRICE_STARTER"),
		StripePriceGrowth:           os.Getenv("STRIPE_PRICE_GROWTH"),
		StripePriceEnterprise:       os.Getenv("STRIPE_PRICE_ENTERPRISE"),
		StorageBackend:              getEnv("STORAGE_BACKEND", "local"),
		S3Bucket:                    os.Getenv("S3_BUCKET"),
		S3Region:                    getEnv("S3_REGION", "us-east-1"),
		SchedulerSecret:             os.Getenv("SCHEDULER_SECRET"),
		ExchangeRateProvider:        getEnv("EXCHANGE_RATE_PROVIDER", "manual"),
		InternalServiceToken:        strings.TrimSpace(getEnv("INTERNAL_SERVICE_TOKEN", localInternalServiceToken)),
		AIServiceURL:                getEnv("AI_SERVICE_URL", "http://ai:8000"),
		WhatsAppWebhookVerifyToken:  os.Getenv("WHATSAPP_WEBHOOK_VERIFY_TOKEN"),
		WhatsAppAppSecret:           os.Getenv("WHATSAPP_APP_SECRET"),
		WhatsAppGraphAPIBaseURL:     getEnv("WHATSAPP_GRAPH_API_BASE_URL", "https://graph.facebook.com/v23.0"),
		MPAppID:                     os.Getenv("MP_APP_ID"),
		MPClientSecret:              os.Getenv("MP_CLIENT_SECRET"),
		MPWebhookSecret:             os.Getenv("MP_WEBHOOK_SECRET"),
		MPRedirectURI:               os.Getenv("MP_REDIRECT_URI"),
		PaymentGatewayMode:          getEnv("PAYMENT_GATEWAY_MODE", "mercadopago"),
		PaymentGatewayEncryptionKey: os.Getenv("PAYMENT_GATEWAY_ENCRYPTION_KEY"),
		SeedDemoData:                getEnvBool("PYMES_SEED_DEMO", false),
		SeedDemoOrgExternalID:       strings.TrimSpace(os.Getenv("PYMES_SEED_DEMO_ORG_EXTERNAL_ID")),
	}
	validateInternalServiceToken(cfg.Environment, cfg.InternalServiceToken)
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizeEnvironment(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return "development"
	}
	return normalized
}

func isLocalEnvironment(environment string) bool {
	switch normalizeEnvironment(environment) {
	case "development", "dev", "local", "test":
		return true
	default:
		return false
	}
}

func validateInternalServiceToken(environment, token string) {
	normalizedToken := strings.TrimSpace(token)
	if isLocalEnvironment(environment) {
		return
	}
	if normalizedToken == "" || strings.EqualFold(normalizedToken, localInternalServiceToken) {
		panic(fmt.Sprintf("invalid INTERNAL_SERVICE_TOKEN for %s environment", normalizeEnvironment(environment)))
	}
}
