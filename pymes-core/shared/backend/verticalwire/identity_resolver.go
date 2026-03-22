package verticalwire

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/devpablocristo/pymes/pymes-core/shared/backend/auth"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/pymescorehttp"
	"github.com/devpablocristo/pymes/pymes-core/shared/backend/verticalconfig"
)

// BuildIdentityResolver construye el resolver JWT de verticales, alineado con claims de Clerk/core/saas.
func BuildIdentityResolver(cfg verticalconfig.Config, log zerolog.Logger, cpHTTP *pymescorehttp.Client) *auth.IdentityResolver {
	var orgRes auth.OrgRefResolver
	if cpHTTP != nil {
		orgRes = NewCoreOrgRefResolver(cpHTTP)
	}
	ic := auth.IdentityConfig{
		Issuer:         cfg.JWTIssuer,
		Audience:       strings.TrimSpace(cfg.JWTAudience),
		OrgClaim:       strings.TrimSpace(cfg.JWTOrgClaim),
		RoleClaim:      strings.TrimSpace(cfg.JWTRoleClaim),
		OrgRefResolver: orgRes,
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

// CoreOrgRefResolver resuelve org_... / slug / UUID vía pymes-core internal API.
type CoreOrgRefResolver struct {
	client *pymescorehttp.Client
}

// NewCoreOrgRefResolver crea un resolver que llama a GET /v1/internal/v1/orgs/resolve-ref.
func NewCoreOrgRefResolver(client *pymescorehttp.Client) *CoreOrgRefResolver {
	return &CoreOrgRefResolver{client: client}
}

func (r *CoreOrgRefResolver) ResolveOrgID(ctx context.Context, ref string) (string, error) {
	if r == nil || r.client == nil {
		return "", fmt.Errorf("pymes-core client not configured")
	}
	m, err := r.client.ResolveOrgRef(ctx, ref)
	if err != nil {
		return "", err
	}
	s, _ := m["org_id"].(string)
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("missing org_id in core response")
	}
	return s, nil
}
