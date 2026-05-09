package wire

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/devpablocristo/core/config/go/envconfig"
	"github.com/devpablocristo/core/errors/go/domainerr"
	saasbilling "github.com/devpablocristo/core/saas/go/billing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SaaSConfig wires the Pymes tenant access runtime from env-backed config.
type SaaSConfig struct {
	StripeSecretKey       string
	StripeWebhookSecret   string
	StripePriceStarter    string
	StripePriceGrowth     string
	StripePriceEnterprise string
	FrontendURL           string
	PublicBaseURL         string

	ClerkSecretKey     string
	ClerkWebhookSecret string
	Environment        string

	JWKSURL        string
	JWTIssuer      string
	JWTAudience    string
	JWTTenantClaim string
	JWTRoleClaim   string
	JWTScopesClaim string
	JWTActorClaim  string

	AuthEnableJWT   bool
	AuthAllowAPIKey bool
}

// SaaSServices holds initialized Pymes tenant HTTP handlers and auth.
type SaaSServices struct {
	Mux            *http.ServeMux
	AuthMiddleware func(http.Handler) http.Handler
	// ResolveTenantRef mapea Clerk org_..., slug o UUID a tenant UUID interno.
	ResolveTenantRef func(ctx context.Context, ref string) (uuid.UUID, bool, error)
}

// SetupSaaS initializes Pymes tenant access on the given GORM DB.
func SetupSaaS(db *gorm.DB, cfg SaaSConfig, log *slog.Logger) (*SaaSServices, error) {
	if db == nil {
		return nil, nil
	}
	if log == nil {
		log = slog.Default()
	}

	store := newPymesSaaSStore(db, log, saasDefaultAPIKeyScopes())
	store.clerk = newClerkBackendClient(strings.TrimSpace(cfg.ClerkSecretKey), strings.TrimSpace(cfg.JWKSURL))
	store.frontendURL = strings.TrimSpace(cfg.FrontendURL)
	store.publicBaseURL = strings.TrimSpace(cfg.PublicBaseURL)
	store.environment = envconfig.NormalizeEnv(cfg.Environment)
	store.clerkWebhookSecret = strings.TrimSpace(cfg.ClerkWebhookSecret)

	var jwtVerifier tenantPrincipalVerifier
	if cfg.AuthEnableJWT && strings.TrimSpace(cfg.JWKSURL) != "" {
		jwtVerifier = &jwtPrincipalVerifier{store: store, cfg: cfg}
	}

	var apiKeyVerifier tenantPrincipalVerifier
	if cfg.AuthAllowAPIKey {
		apiKeyVerifier = &apiKeyPrincipalVerifier{store: store}
	}

	resolveTenantRef := func(ctx context.Context, ref string) (uuid.UUID, bool, error) {
		idStr, ok, err := store.ResolveTenantIDByExternalRef(ctx, ref)
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

	resolveMembership := func(ctx context.Context, tenantID uuid.UUID, actor string) (string, bool, error) {
		return store.FindActiveMembershipRoleByExternalUser(ctx, tenantID.String(), actor)
	}

	authMW := newTenantAuthMiddleware(jwtVerifier, apiKeyVerifier)
	authMW = withTenantSlugBinding(authMW, resolveTenantRef, resolveMembership)
	billingRuntime := saasbilling.NewRuntime(saasbilling.RuntimeConfig{
		StripeSecretKey:       cfg.StripeSecretKey,
		StripeWebhookSecret:   cfg.StripeWebhookSecret,
		StripePriceStarter:    cfg.StripePriceStarter,
		StripePriceGrowth:     cfg.StripePriceGrowth,
		StripePriceEnterprise: cfg.StripePriceEnterprise,
		ConsoleBaseURL:        cfg.FrontendURL,
	}, store, store, nil, nil, log)

	mux := http.NewServeMux()
	registerPymesSaaSRoutes(mux, store, authMW, billingRuntime, pymesSaaSHTTPAuth{
		JWKSURL:   strings.TrimSpace(cfg.JWKSURL),
		JWTIssuer: strings.TrimSpace(cfg.JWTIssuer),
	})
	saasbilling.NewWebhookHandler(billingRuntime).Register(mux)

	return &SaaSServices{
		Mux:              mux,
		AuthMiddleware:   authMW,
		ResolveTenantRef: resolveTenantRef,
	}, nil
}

type apiKeyPrincipalVerifier struct {
	store *pymesSaaSStore
}

type jwtPrincipalVerifier struct {
	store *pymesSaaSStore
	cfg   SaaSConfig
}

func (v *jwtPrincipalVerifier) Verify(ctx context.Context, credential string) (tenantPrincipal, error) {
	claims, err := verifyJWTClaimsMap(ctx, strings.TrimSpace(credential), v.cfg.JWKSURL, v.cfg.JWTIssuer)
	if err != nil {
		return tenantPrincipal{}, err
	}
	actor := stringClaim(claims, valueOrDefault(v.cfg.JWTActorClaim, "sub"))
	if actor == "" {
		actor = stringClaim(claims, "sub")
	}
	if actor == "" {
		return tenantPrincipal{}, domainerr.Unauthorized("missing user claim")
	}
	rawTenant := firstTenantClaim(claims, valueOrDefault(v.cfg.JWTTenantClaim, "tenant_id"), "tenant_id", "o.id")
	if rawTenant == "" {
		return tenantPrincipal{}, domainerr.Forbidden("tenant claim is required")
	}
	tenantID := rawTenant
	if _, parseErr := uuid.Parse(tenantID); parseErr != nil {
		resolved, ok, resolveErr := v.store.ResolveTenantIDByExternalRef(ctx, rawTenant)
		if resolveErr != nil {
			return tenantPrincipal{}, resolveErr
		}
		if !ok {
			return tenantPrincipal{}, domainerr.Forbidden("tenant is not registered in Pymes")
		}
		tenantID = resolved
	}
	role, ok, err := v.store.FindActiveMembershipRoleByExternalUser(ctx, tenantID, actor)
	if err != nil {
		return tenantPrincipal{}, err
	}
	if !ok {
		return tenantPrincipal{}, domainerr.Forbidden("active tenant membership required")
	}
	return tenantPrincipal{
		TenantID:   tenantID,
		Actor:      actor,
		Role:       role,
		Scopes:     scopesFromClaims(claims, valueOrDefault(v.cfg.JWTScopesClaim, "scopes")),
		AuthMethod: "jwt",
	}, nil
}

func (v *apiKeyPrincipalVerifier) Verify(ctx context.Context, credential string) (tenantPrincipal, error) {
	principal, keyID, err := v.store.FindPrincipalByAPIKeyHash(ctx, sha256Hex(strings.TrimSpace(credential)))
	if err != nil {
		return tenantPrincipal{}, err
	}
	actor := "api_key:" + principal.TenantID
	if strings.TrimSpace(keyID) != "" {
		actor = "api_key:" + strings.TrimSpace(keyID)
	}
	return tenantPrincipal{
		TenantID:   principal.TenantID,
		Actor:      actor,
		Role:       "service",
		Scopes:     append([]string(nil), principal.Scopes...),
		AuthMethod: "api_key",
	}, nil
}

func firstTenantClaim(claims map[string]any, names ...string) string {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if value := nestedStringClaim(claims, name); value != "" {
			return value
		}
	}
	return ""
}

func nestedStringClaim(claims map[string]any, path string) string {
	parts := strings.Split(strings.TrimSpace(path), ".")
	var current any = claims
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[part]
	}
	value, _ := current.(string)
	return strings.TrimSpace(value)
}

func scopesFromClaims(claims map[string]any, claimName string) []string {
	raw := claims[strings.TrimSpace(claimName)]
	switch typed := raw.(type) {
	case string:
		return splitScopesCSV(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		return append([]string(nil), typed...)
	default:
		return nil
	}
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
