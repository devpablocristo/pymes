package wire

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	saasadmin "github.com/devpablocristo/saas-core/admin"
	saasadmindomain "github.com/devpablocristo/saas-core/admin/usecases/domain"
	saasbilling "github.com/devpablocristo/saas-core/billing"
	billingdomain "github.com/devpablocristo/saas-core/billing/usecases/domain"
	saasclerk "github.com/devpablocristo/saas-core/clerkwebhook"
	saasidentity "github.com/devpablocristo/saas-core/identity"
	saasjwks "github.com/devpablocristo/saas-core/identity/executor/jwks"
	saasmigrations "github.com/devpablocristo/saas-core/migrations"
	saasorg "github.com/devpablocristo/saas-core/org"
	saasmiddleware "github.com/devpablocristo/saas-core/shared/middleware"
	saasusers "github.com/devpablocristo/saas-core/users"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SaaSConfig wires saas-core from pymes-core env-backed config.
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

// SaaSServices holds initialized saas-core HTTP handlers and auth.
type SaaSServices struct {
	Mux            *http.ServeMux
	AuthMiddleware func(http.Handler) http.Handler
}

// SetupSaaS initializes saas-core on the given GORM DB (same PostgreSQL as Pymes).
// Does not register saas "admin" HTTP routes — Pymes keeps ERP admin under /admin/*.
func SetupSaaS(db *gorm.DB, cfg SaaSConfig, log *slog.Logger) (*SaaSServices, error) {
	if db == nil {
		return nil, nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := saasmigrations.MigrateUp(context.Background(), sqlDB, "saas-core"); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}

	orgRepo := saasorg.NewRepository(db)
	defaultAPIKeyScopes := saasDefaultAPIKeyScopes()
	orgHandler := saasorg.NewHandlerWithScopes(orgRepo, defaultAPIKeyScopes)
	orgUC := saasorg.NewUsecases(orgRepo)

	usersRepo := saasusers.NewRepositoryWithScopes(db, defaultAPIKeyScopes)
	usersUC := saasusers.NewUsecases(usersRepo)
	usersHandler := saasusers.NewHandler(usersUC)

	clerkHandler := saasclerk.NewHandler(saasclerk.Config{
		ClerkWebhookSecret: cfg.ClerkWebhookSecret,
		TowerBaseURL:       cfg.FrontendURL,
	}, &userSyncerAdapter{uc: usersUC}, nil, log)

	adminRepo := saasadmin.NewRepository(db)

	var jwtVerifier saasmiddleware.PrincipalVerifier
	if cfg.AuthEnableJWT && strings.TrimSpace(cfg.JWKSURL) != "" {
		jwksVerifier := saasjwks.NewVerifier(strings.TrimSpace(cfg.JWKSURL))
		identityUC := saasidentity.NewUsecasesWithOrgResolver(jwksVerifier, orgRepo, saasidentity.Config{
			Issuer:      strings.TrimSpace(cfg.JWTIssuer),
			Audience:    strings.TrimSpace(cfg.JWTAudience),
			OrgClaim:    valueOrDefault(cfg.JWTOrgClaim, "org_id"),
			RoleClaim:   valueOrDefault(cfg.JWTRoleClaim, "org_role"),
			ScopesClaim: valueOrDefault(cfg.JWTScopesClaim, "scopes"),
			ActorClaim:  valueOrDefault(cfg.JWTActorClaim, "sub"),
		})
		jwtVerifier = &jwtPrincipalVerifier{uc: identityUC}
	}

	var apiKeyVerifier saasmiddleware.PrincipalVerifier
	if cfg.AuthAllowAPIKey {
		apiKeyVerifier = &apiKeyPrincipalVerifier{uc: orgUC}
	}

	authMW := saasmiddleware.NewAuthMiddleware(jwtVerifier, apiKeyVerifier)

	stripeClient := saasbilling.NewStripeClient(cfg.StripeSecretKey)
	billingRepo := saasbilling.NewRepository(db)
	billingUC := saasbilling.NewUsecases(
		saasbilling.Config{
			StripeSecretKey:       cfg.StripeSecretKey,
			StripeWebhookSecret:   cfg.StripeWebhookSecret,
			StripePriceStarter:    cfg.StripePriceStarter,
			StripePriceGrowth:     cfg.StripePriceGrowth,
			StripePriceEnterprise: cfg.StripePriceEnterprise,
			TowerBaseURL:          cfg.FrontendURL,
		},
		billingRepo,
		&tenantSettingsAdapter{repo: adminRepo},
		stripeClient,
		nil,
		nil,
		log,
	)
	billingHandler := saasbilling.NewHandler(billingUC)

	mux := http.NewServeMux()
	orgHandler.Register(mux)
	usersHandler.Register(mux)
	clerkHandler.Register(mux)
	billingHandler.Register(mux)
	billingHandler.RegisterWebhook(mux)

	return &SaaSServices{
		Mux:            mux,
		AuthMiddleware: authMW,
	}, nil
}

type userSyncerAdapter struct {
	uc *saasusers.Usecases
}

func (a *userSyncerAdapter) SyncUser(ctx context.Context, externalID, email, name string, avatarURL *string) (saasclerk.SyncedUser, error) {
	u, err := a.uc.SyncUser(ctx, externalID, email, name, avatarURL)
	if err != nil {
		return saasclerk.SyncedUser{}, err
	}
	return saasclerk.SyncedUser{ID: u.ID, ExternalID: u.ExternalID}, nil
}

func (a *userSyncerAdapter) SyncOrganization(ctx context.Context, orgExternalID, orgName string) (uuid.UUID, error) {
	return a.uc.SyncOrganization(ctx, orgExternalID, orgName)
}

func (a *userSyncerAdapter) SyncMembership(ctx context.Context, orgID uuid.UUID, userExternalID, email, name string, avatarURL *string, role string) (saasclerk.SyncedMember, error) {
	m, err := a.uc.SyncMembership(ctx, orgID, userExternalID, email, name, avatarURL, role)
	if err != nil {
		return saasclerk.SyncedMember{}, err
	}
	return saasclerk.SyncedMember{ID: m.ID, OrgID: m.OrgID}, nil
}

func (a *userSyncerAdapter) SoftDeleteUser(ctx context.Context, externalID string) error {
	return a.uc.SoftDeleteUser(ctx, externalID)
}

func (a *userSyncerAdapter) RemoveMembership(ctx context.Context, userExternalID, orgExternalID, orgName string) error {
	return a.uc.RemoveMembership(ctx, userExternalID, orgExternalID, orgName)
}

type tenantSettingsAdapter struct {
	repo *saasadmin.Repository
}

func (a *tenantSettingsAdapter) UpsertTenantSettings(ctx context.Context, s billingdomain.TenantSettings) (billingdomain.TenantSettings, error) {
	stored, err := a.repo.UpsertTenantSettings(ctx, saasadmindomain.TenantSettings{
		OrgID:      s.OrgID,
		PlanCode:   s.PlanCode,
		Status:     saasadmindomain.TenantStatusActive,
		HardLimits: s.HardLimits,
		UpdatedAt:  s.UpdatedAt,
	})
	if err != nil {
		return billingdomain.TenantSettings{}, err
	}
	return billingdomain.TenantSettings{
		OrgID:      stored.OrgID,
		PlanCode:   stored.PlanCode,
		HardLimits: stored.HardLimits,
		UpdatedAt:  stored.UpdatedAt,
	}, nil
}

type apiKeyPrincipalVerifier struct {
	uc *saasorg.Usecases
}

func (v *apiKeyPrincipalVerifier) Verify(ctx context.Context, credential string) (saasmiddleware.Principal, error) {
	principal, err := v.uc.ResolvePrincipal(ctx, sha256Hex(strings.TrimSpace(credential)))
	if err != nil {
		return saasmiddleware.Principal{}, err
	}
	return saasmiddleware.Principal{
		OrgID:      principal.OrgID.String(),
		Scopes:     append([]string(nil), principal.Scopes...),
		AuthMethod: "api_key",
	}, nil
}

type jwtPrincipalVerifier struct {
	uc *saasidentity.Usecases
}

func (v *jwtPrincipalVerifier) Verify(ctx context.Context, credential string) (saasmiddleware.Principal, error) {
	principal, err := v.uc.ResolvePrincipal(ctx, strings.TrimSpace(credential))
	if err != nil {
		return saasmiddleware.Principal{}, err
	}
	return saasmiddleware.Principal{
		OrgID:      principal.OrgID.String(),
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
	return append([]string(nil), saasusers.DefaultAPIKeyScopes...)
}
