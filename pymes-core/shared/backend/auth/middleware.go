package auth

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authn "github.com/devpablocristo/core/authn/go"
	ginmw "github.com/devpablocristo/core/backend/gin/go"
)

// AuthMiddleware re-exporta el tipo de core.
type AuthMiddleware = ginmw.AuthMiddleware

// AuthContext re-exporta el tipo de core.
type AuthContext = ginmw.AuthContext

// APIKeyResolver interfaz legacy para resolver API keys por hash.
// Los verticales implementan esto vía verticalwire.NewAPIKeyResolver.
type APIKeyResolver interface {
	ResolveAPIKey(raw string) (ResolvedKey, bool)
}

// ResolvedKey identidad resuelta desde una API key.
type ResolvedKey struct {
	ID     uuid.UUID
	OrgID  uuid.UUID
	Scopes []string
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
		OrgID:      p.OrgID,
		Actor:      p.Actor,
		Role:       p.Role,
		Scopes:     p.Scopes,
		AuthMethod: "jwt",
	}, nil
}

// apiKeyAdapter adapta APIKeyResolver legacy a authn.Authenticator.
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
		OrgID:      key.OrgID.String(),
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
	return ginmw.GetAuthContext(c)
}

// ParseAuthOrgID extrae y parsea el org_id del auth context.
func ParseAuthOrgID(c *gin.Context) (uuid.UUID, bool) {
	auth := GetAuthContext(c)
	orgID, err := uuid.Parse(auth.OrgID)
	if err != nil {
		return uuid.Nil, false
	}
	return orgID, true
}
