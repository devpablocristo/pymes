// Package verticalconfig: env común de lambdas verticales (solo difiere el puerto por defecto).
package verticalconfig

import (
	"os"
	"strconv"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	JWKSURL              string
	JWTIssuer            string
	JWTAudience          string
	JWTOrgClaim          string
	JWTRoleClaim         string
	AuthEnableJWT        bool
	AuthAllowAPIKey      bool
	InternalServiceToken string
	PymesCoreURL         string
	FrontendURL          string
}

type Options struct {
	DefaultPort string
}

func Load(opts Options) Config {
	return Config{
		Port:                 getEnv("PORT", opts.DefaultPort),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/pymes?sslmode=disable"),
		JWKSURL:              os.Getenv("JWKS_URL"),
		JWTIssuer:            os.Getenv("JWT_ISSUER"),
		JWTAudience:          os.Getenv("JWT_AUDIENCE"),
		JWTOrgClaim:          os.Getenv("JWT_ORG_CLAIM"),
		JWTRoleClaim:         os.Getenv("JWT_ROLE_CLAIM"),
		AuthEnableJWT:        getEnvBool("AUTH_ENABLE_JWT", true),
		AuthAllowAPIKey:      getEnvBool("AUTH_ALLOW_API_KEY", true),
		InternalServiceToken: getEnv("INTERNAL_SERVICE_TOKEN", "local-internal-token"),
		PymesCoreURL:         getEnv("PYMES_CORE_URL", "http://localhost:8080"),
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
