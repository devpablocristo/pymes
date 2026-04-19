// Package verticalconfig: env común de lambdas verticales (solo difiere el puerto por defecto).
package verticalconfig

import (
	"fmt"
	"log"
	"strings"

	"github.com/devpablocristo/core/config/go/envconfig"
)

const localInternalServiceToken = "local-internal-token"

type Config struct {
	Port                 string
	Environment          string
	DatabaseURL          string
	JWKSURL              string
	JWTIssuer            string
	JWTAudience          string
	JWTOrgClaim          string
	JWTRoleClaim         string
	JWTActorClaim        string
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
	cfg := Config{
		Port:                 envconfig.Get("PORT", opts.DefaultPort),
		Environment:          envconfig.NormalizeEnv(envconfig.Get("ENVIRONMENT", "development")),
		DatabaseURL:          envconfig.Get("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/pymes?sslmode=disable"),
		JWKSURL:              envconfig.Get("JWKS_URL", ""),
		JWTIssuer:            envconfig.Get("JWT_ISSUER", ""),
		JWTAudience:          envconfig.Get("JWT_AUDIENCE", ""),
		JWTOrgClaim:          envconfig.Get("JWT_ORG_CLAIM", ""),
		JWTRoleClaim:         envconfig.Get("JWT_ROLE_CLAIM", ""),
		JWTActorClaim:        envconfig.Get("JWT_ACTOR_CLAIM", ""),
		AuthEnableJWT:        envconfig.Bool("AUTH_ENABLE_JWT", true),
		AuthAllowAPIKey:      envconfig.Bool("AUTH_ALLOW_API_KEY", true),
		InternalServiceToken: strings.TrimSpace(envconfig.Get("INTERNAL_SERVICE_TOKEN", localInternalServiceToken)),
		PymesCoreURL:         envconfig.Get("PYMES_CORE_URL", "http://localhost:8080"),
		FrontendURL:          envconfig.Get("FRONTEND_URL", "http://localhost:5173"),
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
