// Package verticalconfig: env común de lambdas verticales (solo difiere el puerto por defecto).
package verticalconfig

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const localInternalServiceToken = "local-internal-token"

type Config struct {
	Port                  string
	Environment           string
	DatabaseURL           string
	SeedDemoData          bool
	SeedDemoOrgExternalID string
	JWKSURL               string
	JWTIssuer             string
	JWTAudience           string
	JWTOrgClaim           string
	JWTRoleClaim          string
	AuthEnableJWT         bool
	AuthAllowAPIKey       bool
	InternalServiceToken  string
	PymesCoreURL          string
	FrontendURL           string
}

type Options struct {
	DefaultPort string
}

func Load(opts Options) Config {
	cfg := Config{
		Port:                  getEnv("PORT", opts.DefaultPort),
		Environment:           normalizeEnvironment(getEnv("ENVIRONMENT", "development")),
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/pymes?sslmode=disable"),
		JWKSURL:               os.Getenv("JWKS_URL"),
		JWTIssuer:             os.Getenv("JWT_ISSUER"),
		JWTAudience:           os.Getenv("JWT_AUDIENCE"),
		JWTOrgClaim:           os.Getenv("JWT_ORG_CLAIM"),
		JWTRoleClaim:          os.Getenv("JWT_ROLE_CLAIM"),
		AuthEnableJWT:         getEnvBool("AUTH_ENABLE_JWT", true),
		AuthAllowAPIKey:       getEnvBool("AUTH_ALLOW_API_KEY", true),
		InternalServiceToken:  strings.TrimSpace(getEnv("INTERNAL_SERVICE_TOKEN", localInternalServiceToken)),
		PymesCoreURL:          getEnv("PYMES_CORE_URL", "http://localhost:8080"),
		FrontendURL:           getEnv("FRONTEND_URL", "http://localhost:5173"),
		SeedDemoData:          getEnvBool("PYMES_SEED_DEMO", false),
		SeedDemoOrgExternalID: strings.TrimSpace(os.Getenv("PYMES_SEED_DEMO_ORG_EXTERNAL_ID")),
	}
	validateInternalServiceToken(cfg.Environment, cfg.InternalServiceToken)
	return cfg
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
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

// IsLocalEnvironment indica ambiente de desarrollo local (no usar overrides de org en prod).
func IsLocalEnvironment(environment string) bool {
	return isLocalEnvironment(environment)
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
