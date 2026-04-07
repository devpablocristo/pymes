// Package verticalconfig: env común de lambdas verticales (solo difiere el puerto por defecto).
package verticalconfig

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/devpablocristo/core/config/go/envconfig"
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
	JWTActorClaim         string
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
		Port:                  envconfig.Get("PORT", opts.DefaultPort),
		Environment:           envconfig.NormalizeEnv(envconfig.Get("ENVIRONMENT", "development")),
		DatabaseURL:           envconfig.Get("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/pymes?sslmode=disable"),
		JWKSURL:               os.Getenv("JWKS_URL"),
		JWTIssuer:             os.Getenv("JWT_ISSUER"),
		JWTAudience:           os.Getenv("JWT_AUDIENCE"),
		JWTOrgClaim:           os.Getenv("JWT_ORG_CLAIM"),
		JWTRoleClaim:          os.Getenv("JWT_ROLE_CLAIM"),
		JWTActorClaim:         os.Getenv("JWT_ACTOR_CLAIM"),
		AuthEnableJWT:         envconfig.Bool("AUTH_ENABLE_JWT", true),
		AuthAllowAPIKey:       envconfig.Bool("AUTH_ALLOW_API_KEY", true),
		InternalServiceToken:  strings.TrimSpace(envconfig.Get("INTERNAL_SERVICE_TOKEN", localInternalServiceToken)),
		PymesCoreURL:          envconfig.Get("PYMES_CORE_URL", "http://localhost:8080"),
		FrontendURL:           envconfig.Get("FRONTEND_URL", "http://localhost:5173"),
		SeedDemoData:          envconfig.Bool("PYMES_SEED_DEMO", false),
		SeedDemoOrgExternalID: strings.TrimSpace(os.Getenv("PYMES_SEED_DEMO_ORG_EXTERNAL_ID")),
	}
	if err := validateInternalServiceToken(cfg.Environment, cfg.InternalServiceToken); err != nil {
		log.Fatal(err)
	}
	return cfg
}

// IsLocalEnvironment indica ambiente de desarrollo local. Delega a core.
func IsLocalEnvironment(environment string) bool {
	return envconfig.IsLocal(environment)
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
