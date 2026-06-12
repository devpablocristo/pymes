package verticalwire

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/devpablocristo/pymes/core/shared/backend/auth"
	"github.com/devpablocristo/pymes/core/shared/backend/pymescorehttp"
	"github.com/devpablocristo/pymes/core/shared/backend/verticalconfig"
)

// BuildIdentityResolver construye el resolver JWT de verticales, alineado con claims de Clerk/core/saas.
func BuildIdentityResolver(cfg verticalconfig.Config, log zerolog.Logger, cpHTTP *pymescorehttp.Client) *auth.IdentityResolver {
	var tenantRes auth.OrgRefResolver
	if cpHTTP != nil {
		tenantRes = NewCoreTenantRefResolver(cpHTTP)
	}
	ic := auth.IdentityConfig{
		Issuer:            cfg.JWTIssuer,
		Audience:          strings.TrimSpace(cfg.JWTAudience),
		OrgClaim:          strings.TrimSpace(cfg.JWTTenantClaim),
		RoleClaim:         strings.TrimSpace(cfg.JWTRoleClaim),
		OrgRefResolver: tenantRes,
	}
	if cfg.JWKSURL == "" {
		log.Warn().Msg("JWKS_URL not set; JWT auth will fail unless AUTH_ENABLE_JWT=false")
		return auth.NewIdentityResolverWithConfig(nil, ic)
	}
	verifier, err := auth.NewJWKSVerifier(cfg.JWKSURL)
	if err != nil {
		log.Error().Err(err).Msg("invalid JWKS verifier; JWT auth will fail")
		return auth.NewIdentityResolverWithConfig(nil, ic)
	}
	return auth.NewIdentityResolverWithConfig(verifier, ic)
}

// CoreOrgRefResolver resuelve Clerk organization id, slug o UUID a tenant_id vía core internal API.
type CoreOrgRefResolver struct {
	client *pymescorehttp.Client
}

// NewCoreTenantRefResolver crea un resolver que llama a GET /v1/internal/v1/tenants/resolve-ref.
func NewCoreTenantRefResolver(client *pymescorehttp.Client) *CoreOrgRefResolver {
	return &CoreOrgRefResolver{client: client}
}

func (r *CoreOrgRefResolver) ResolveOrgID(ctx context.Context, ref string) (string, error) {
	if r == nil || r.client == nil {
		return "", fmt.Errorf("core client not configured")
	}
	m, err := r.client.ResolveOrgRef(ctx, ref)
	if err != nil {
		return "", err
	}
	s, _ := m["org_id"].(string)
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("missing tenant_id in core response")
	}
	return s, nil
}
