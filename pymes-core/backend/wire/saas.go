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
	kerneldomain "github.com/devpablocristo/core/saas/go/kernel/usecases/domain"
	saasmiddleware "github.com/devpablocristo/core/saas/go/middleware"
	saasmigrations "github.com/devpablocristo/core/saas/go/migrations"

	"github.com/google/uuid"
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
	// ResolveOrgRef mapea external_id (Clerk org_...), slug o UUID a org UUID interno (misma lógica que JWT en core).
	ResolveOrgRef func(ctx context.Context, ref string) (uuid.UUID, bool, error)
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

	var jwtVerifier saasidentity.PrincipalVerifier
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

	var apiKeyVerifier saasidentity.PrincipalVerifier
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
	registerPymesSaaSRoutes(mux, store, authMW, billingRuntime, pymesSaaSHTTPAuth{
		JWKSURL:   strings.TrimSpace(cfg.JWKSURL),
		JWTIssuer: strings.TrimSpace(cfg.JWTIssuer),
	})
	clerkHandler.Register(mux)
	saasbilling.NewWebhookHandler(billingRuntime).Register(mux)

	resolveOrgRef := func(ctx context.Context, ref string) (uuid.UUID, bool, error) {
		idStr, ok, err := store.FindOrgIDByExternalID(ctx, ref)
		if err != nil {
			return uuid.Nil, false, err
		}
		if !ok {
			return uuid.Nil, false, nil
		}
		id, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			return uuid.Nil, false, err
		}
		return id, true, nil
	}

	return &SaaSServices{
		Mux:            mux,
		AuthMiddleware: authMW,
		ResolveOrgRef:  resolveOrgRef,
	}, nil
}

type apiKeyPrincipalVerifier struct {
	store *pymesSaaSStore
}

func (v *apiKeyPrincipalVerifier) Verify(ctx context.Context, credential string) (kerneldomain.Principal, error) {
	principal, keyID, err := v.store.FindPrincipalByAPIKeyHash(ctx, sha256Hex(strings.TrimSpace(credential)))
	if err != nil {
		return kerneldomain.Principal{}, err
	}
	actor := "api_key:" + principal.TenantID
	if strings.TrimSpace(keyID) != "" {
		actor = "api_key:" + strings.TrimSpace(keyID)
	}
	return kerneldomain.Principal{
		TenantID:   principal.TenantID,
		Actor:      actor,
		Role:       "service",
		Scopes:     append([]string(nil), principal.Scopes...),
		AuthMethod: "api_key",
	}, nil
}

type jwtPrincipalVerifier struct {
	uc *saasidentity.UseCases
}

func (v *jwtPrincipalVerifier) Verify(ctx context.Context, credential string) (kerneldomain.Principal, error) {
	principal, err := v.uc.ResolvePrincipal(ctx, strings.TrimSpace(credential))
	if err != nil {
		return kerneldomain.Principal{}, err
	}
	return kerneldomain.Principal{
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
