package auth

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authn "github.com/devpablocristo/core/authn/go"
	ginmw "github.com/devpablocristo/core/http/gin/go"
	ctxkeys "github.com/devpablocristo/core/security/go/contextkeys"
)

// AuthMiddleware re-exporta el tipo de core.
type AuthMiddleware = ginmw.AuthMiddleware

// AuthContext es el contexto de autenticacion de Pymes.
type AuthContext struct {
	TenantID   string
	Actor      string
	Role       string
	Scopes     []string
	AuthMethod string
}

// APIKeyResolver resuelve API keys por hash.
// Los verticales implementan esto vía verticalwire.NewAPIKeyResolver.
type APIKeyResolver interface {
	ResolveAPIKey(raw string) (ResolvedKey, bool)
}

// ResolvedKey identidad resuelta desde una API key.
type ResolvedKey struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	Scopes   []string
}

// jwtAdapter adapta IdentityResolver a authn.Authenticator.
type jwtAdapter struct {
	resolver *IdentityResolver
}

func (a *jwtAdapter) Authenticate(ctx context.Context, cred authn.Credential) (*authn.Principal, error) {
	bc, ok := cred.(authn.BearerCredential)
	if !ok {
		return nil, authn.ErrWrongCredentialKind
	}
	p, err := a.resolver.ResolvePrincipal(ctx, bc.Token)
	if err != nil {
		return nil, err
	}
	return &authn.Principal{
		OrgID:      p.TenantID,
		Actor:      p.Actor,
		Role:       p.Role,
		Scopes:     p.Scopes,
		AuthMethod: "jwt",
	}, nil
}

// apiKeyAdapter adapta APIKeyResolver a authn.Authenticator.
type apiKeyAdapter struct {
	resolver APIKeyResolver
}

func (a *apiKeyAdapter) Authenticate(ctx context.Context, cred authn.Credential) (*authn.Principal, error) {
	kc, ok := cred.(authn.APIKeyCredential)
	if !ok {
		return nil, authn.ErrWrongCredentialKind
	}
	key, found := a.resolver.ResolveAPIKey(kc.Key)
	if !found {
		return nil, fmt.Errorf("authn: invalid api key")
	}
	return &authn.Principal{
		OrgID:      key.TenantID.String(),
		Actor:      "api_key:" + key.ID.String(),
		Role:       "service",
		Scopes:     key.Scopes,
		AuthMethod: "api_key",
	}, nil
}

// NewAuthMiddleware crea middleware de autenticación. Delega a core.
func NewAuthMiddleware(identity *IdentityResolver, keyResolver APIKeyResolver, authEnableJWT, authAllowKey bool) *AuthMiddleware {
	var jwtAuth authn.Authenticator
	if authEnableJWT && identity != nil {
		jwtAuth = &jwtAdapter{resolver: identity}
	}
	var apiKeyAuth authn.Authenticator
	if authAllowKey && keyResolver != nil {
		apiKeyAuth = &apiKeyAdapter{resolver: keyResolver}
	}
	return ginmw.NewAuthMiddleware(jwtAuth, apiKeyAuth)
}

// GetAuthContext extrae el contexto de autenticación. Delega a core.
func GetAuthContext(c *gin.Context) AuthContext {
	raw := ginmw.GetAuthContext(c)
	tenantID := raw.OrgID
	if tenantID == "" {
		if v, ok := c.Get(ctxkeys.CtxKeyTenantID); ok {
			if s, ok := v.(string); ok {
				tenantID = s
			}
		}
	}
	return AuthContext{
		TenantID:   tenantID,
		Actor:      raw.Actor,
		Role:       raw.Role,
		Scopes:     raw.Scopes,
		AuthMethod: raw.AuthMethod,
	}
}

// ParseAuthTenantID extrae y parsea el tenant_id del auth context.
func ParseAuthTenantID(c *gin.Context) (uuid.UUID, bool) {
	auth := GetAuthContext(c)
	tenantID, err := uuid.Parse(auth.TenantID)
	if err != nil {
		return uuid.Nil, false
	}
	return tenantID, true
}
