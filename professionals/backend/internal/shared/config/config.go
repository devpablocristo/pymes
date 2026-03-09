package config

import (
	"os"
	"strconv"
)

// Config centraliza la configuracion externa para mantener el mismo codigo entre ambientes.
type Config struct {
	Port                 string
	DatabaseURL          string
	JWKSURL              string
	JWTIssuer            string
	AuthEnableJWT        bool
	AuthAllowAPIKey      bool
	InternalServiceToken string
	ControlPlaneURL      string
	FrontendURL          string
}

// LoadFromEnv carga valores con defaults seguros para desarrollo local.
func LoadFromEnv() Config {
	return Config{
		Port:                 getEnv("PORT", "8081"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/pymes?sslmode=disable"),
		JWKSURL:              os.Getenv("JWKS_URL"),
		JWTIssuer:            os.Getenv("JWT_ISSUER"),
		AuthEnableJWT:        getEnvBool("AUTH_ENABLE_JWT", true),
		AuthAllowAPIKey:      getEnvBool("AUTH_ALLOW_API_KEY", true),
		InternalServiceToken: getEnv("INTERNAL_SERVICE_TOKEN", "local-internal-token"),
		ControlPlaneURL:      getEnv("CONTROL_PLANE_URL", "http://localhost:8080"),
		FrontendURL:          getEnv("FRONTEND_URL", "http://localhost:5173"),
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
