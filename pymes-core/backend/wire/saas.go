package wire

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	saasjwks "github.com/devpablocristo/core/authn/go/jwks"
	saasbilling "github.com/devpablocristo/core/saas/go/billing"
	saasclerk "github.com/devpablocristo/core/saas/go/clerkwebhook"
	saasidentity "github.com/devpablocristo/core/saas/go/identity"
	saasmiddleware "github.com/devpablocristo/core/saas/go/middleware"
	saasmigrations "github.com/devpablocristo/core/saas/go/migrations"

	"gorm.io/gorm"
)

// SaaSConfig wires core/saas/go from pymes-core env-backed config.
type SaaSConfig struct {
	StripeSecretKey       string
	StripeWebhookSecret   string
	StripePriceStarter    string
	StripePriceGrowth     string
	StripePriceEnterprise string
	FrontendURL           string

	ClerkWebhookSecret string

	JWKSURL        string
	JWTIssuer      string
	JWTAudience    string
	JWTOrgClaim    string
	JWTRoleClaim   string
	JWTScopesClaim string
	JWTActorClaim  string

	AuthEnableJWT   bool
	AuthAllowAPIKey bool
}

// SaaSServices holds initialized core/saas/go HTTP handlers and auth.
type SaaSServices struct {
	Mux            *http.ServeMux
	AuthMiddleware func(http.Handler) http.Handler
}

// SetupSaaS initializes core/saas/go on the given GORM DB (same PostgreSQL as Pymes).
func SetupSaaS(db *gorm.DB, cfg SaaSConfig, log *slog.Logger) (*SaaSServices, error) {
	if db == nil {
		return nil, nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := saasmigrations.MigrateUp(context.Background(), sqlDB, "core-saas"); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}

	store := newPymesSaaSStore(db, log, saasDefaultAPIKeyScopes())

	var jwtVerifier saasmiddleware.PrincipalVerifier
	if cfg.AuthEnableJWT && strings.TrimSpace(cfg.JWKSURL) != "" {
		jwksVerifier := saasjwks.NewVerifier(strings.TrimSpace(cfg.JWKSURL))
		identityUC := saasidentity.NewUsecasesWithOrgResolver(jwksVerifier, store, saasidentity.Config{
			Issuer:      valueOrDefault(cfg.JWTIssuer, ""),
			Audience:    valueOrDefault(cfg.JWTAudience, ""),
			OrgClaim:    valueOrDefault(cfg.JWTOrgClaim, "org_id"),
			RoleClaim:   valueOrDefault(cfg.JWTRoleClaim, "org_role"),
			ScopesClaim: valueOrDefault(cfg.JWTScopesClaim, "scopes"),
			ActorClaim:  valueOrDefault(cfg.JWTActorClaim, "sub"),
		})
		jwtVerifier = &jwtPrincipalVerifier{uc: identityUC}
	}

	var apiKeyVerifier saasmiddleware.PrincipalVerifier
	if cfg.AuthAllowAPIKey {
		apiKeyVerifier = &apiKeyPrincipalVerifier{store: store}
	}

	authMW := saasmiddleware.NewAuthMiddleware(jwtVerifier, apiKeyVerifier)
	billingRuntime := saasbilling.NewRuntime(saasbilling.RuntimeConfig{
		StripeSecretKey:       cfg.StripeSecretKey,
		StripeWebhookSecret:   cfg.StripeWebhookSecret,
		StripePriceStarter:    cfg.StripePriceStarter,
		StripePriceGrowth:     cfg.StripePriceGrowth,
		StripePriceEnterprise: cfg.StripePriceEnterprise,
		ConsoleBaseURL:        cfg.FrontendURL,
	}, store, store, nil, nil, log)
	clerkHandler := saasclerk.NewHandler(saasclerk.Config{
		ClerkWebhookSecret: cfg.ClerkWebhookSecret,
		ConsoleBaseURL:     cfg.FrontendURL,
	}, store, nil, log)

	mux := http.NewServeMux()
	registerPymesSaaSRoutes(mux, store, authMW, billingRuntime)
	clerkHandler.Register(mux)
	saasbilling.NewWebhookHandler(billingRuntime).Register(mux)

	return &SaaSServices{
		Mux:            mux,
		AuthMiddleware: authMW,
	}, nil
}

type apiKeyPrincipalVerifier struct {
	store *pymesSaaSStore
}

func (v *apiKeyPrincipalVerifier) Verify(ctx context.Context, credential string) (saasmiddleware.Principal, error) {
	principal, _, err := v.store.FindPrincipalByAPIKeyHash(ctx, sha256Hex(strings.TrimSpace(credential)))
	if err != nil {
		return saasmiddleware.Principal{}, err
	}
	return saasmiddleware.Principal{
		TenantID:   principal.TenantID,
		Actor:      "api_key:" + principal.TenantID,
		Role:       "service",
		Scopes:     append([]string(nil), principal.Scopes...),
		AuthMethod: "api_key",
	}, nil
}

type jwtPrincipalVerifier struct {
	uc *saasidentity.UseCases
}

func (v *jwtPrincipalVerifier) Verify(ctx context.Context, credential string) (saasmiddleware.Principal, error) {
	principal, err := v.uc.ResolvePrincipal(ctx, strings.TrimSpace(credential))
	if err != nil {
		return saasmiddleware.Principal{}, err
	}
	return saasmiddleware.Principal{
		TenantID:   principal.TenantID,
		Actor:      principal.Actor,
		Role:       principal.Role,
		Scopes:     append([]string(nil), principal.Scopes...),
		AuthMethod: "jwt",
	}, nil
}

func sha256Hex(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func valueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func saasDefaultAPIKeyScopes() []string {
	return []string{"admin:console:read", "admin:console:write"}
}
