package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	JWKSURL              string
	JWTIssuer            string
	AuthEnableJWT        bool
	AuthAllowAPIKey      bool
	InternalServiceToken string
	PymesCoreURL      string
	FrontendURL          string
}

func LoadFromEnv() Config {
	return Config{
		Port:                 getEnv("PORT", "8082"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/pymes?sslmode=disable"),
		JWKSURL:              os.Getenv("JWKS_URL"),
		JWTIssuer:            os.Getenv("JWT_ISSUER"),
		AuthEnableJWT:        getEnvBool("AUTH_ENABLE_JWT", true),
		AuthAllowAPIKey:      getEnvBool("AUTH_ALLOW_API_KEY", true),
		InternalServiceToken: getEnv("INTERNAL_SERVICE_TOKEN", "local-internal-token"),
		PymesCoreURL:      getEnv("PYMES_CORE_URL", "http://localhost:8080"),
		FrontendURL:          getEnv("FRONTEND_URL", "http://localhost:5173"),
	}
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
