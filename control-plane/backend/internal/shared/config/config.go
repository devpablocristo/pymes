// Package config loads backend configuration from external environment variables.
package config

import (
	"os"
	"strconv"
)

// Config centraliza la configuracion externa para mantener el mismo codigo entre ambientes.
type Config struct {
	Port                        string
	DatabaseURL                 string
	JWKSURL                     string
	JWTIssuer                   string
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
}

// LoadFromEnv carga valores con defaults seguros para desarrollo local.
func LoadFromEnv() Config {
	return Config{
		Port:                        getEnv("PORT", "8080"),
		DatabaseURL:                 getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/pymes?sslmode=disable"),
		JWKSURL:                     os.Getenv("JWKS_URL"),
		JWTIssuer:                   os.Getenv("JWT_ISSUER"),
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
		InternalServiceToken:        getEnv("INTERNAL_SERVICE_TOKEN", "local-internal-token"),
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
	}
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
