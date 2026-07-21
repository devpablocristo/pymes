package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authn "github.com/devpablocristo/platform/authn/go"
	ginmw "github.com/devpablocristo/platform/http/gin/go"
	ctxkeys "github.com/devpablocristo/platform/security/go/contextkeys"
)

// AuthMiddleware re-exporta el tipo de core.
type AuthMiddleware = ginmw.AuthMiddleware

// AuthContext es el contexto de autenticacion de Pymes.
type AuthContext struct {
	OrgID   string
	Actor      string
	Role       string
	Scopes     []string
	AuthMethod string
}

const TenantSlugHeader = "X-Pymes-Tenant-Slug"

// APIKeyResolver resuelve API keys por hash.
// Los verticales implementan esto vía verticalwire.NewAPIKeyResolver.
type APIKeyResolver interface {
	ResolveAPIKey(raw string) (ResolvedKey, bool)
}

// ResolvedKey identidad resuelta desde una API key.
type ResolvedKey struct {
	ID       uuid.UUID
	OrgID uuid.UUID
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
		OrgID:      p.OrgID,
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

// RequireTenantSlugBinding fuerza que el slug de consola matchee el tenant autenticado.
// Las API keys service-to-service pueden omitirlo; si lo envían, también debe coincidir.
func RequireTenantSlugBinding(resolver OrgRefResolver, membershipResolvers ...TenantMembershipResolver) gin.HandlerFunc {
	var membershipResolver TenantMembershipResolver
	if len(membershipResolvers) > 0 {
		membershipResolver = membershipResolvers[0]
	}
	return func(c *gin.Context) {
		authCtx := GetAuthContext(c)
		slug := strings.TrimSpace(c.GetHeader(TenantSlugHeader))
		authMethod := strings.TrimSpace(authCtx.AuthMethod)
		if slug == "" {
			if authMethod == "api_key" {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "tenant_slug_required",
				"message": "tenant slug header is required",
			})
			return
		}
		if resolver == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "tenant_mismatch",
				"message": "tenant slug is not valid for this session",
			})
			return
		}
		resolvedOrgID, err := resolver.ResolveOrgID(c.Request.Context(), slug)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "tenant_mismatch",
				"message": "tenant slug is not valid for this session",
			})
			return
		}
		if !strings.EqualFold(strings.TrimSpace(authCtx.OrgID), strings.TrimSpace(resolvedOrgID)) {
			if authMethod == "jwt" && membershipResolver != nil {
				role, ok, membershipErr := membershipResolver.FindActiveMembershipRole(c.Request.Context(), resolvedOrgID, authCtx.Actor)
				if membershipErr != nil {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"code":    "tenant_mismatch",
						"message": "tenant slug is not valid for this session",
					})
					return
				}
				if ok {
					c.Set(ctxkeys.CtxKeyOrgID, strings.TrimSpace(resolvedOrgID))
					c.Set(ctxkeys.CtxKeyRole, strings.TrimSpace(role))
					c.Next()
					return
				}
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "tenant_mismatch",
				"message": "tenant slug is not valid for this session",
			})
			return
		}
		c.Next()
	}
}

// GetAuthContext extrae el contexto de autenticación. Delega a core.
func GetAuthContext(c *gin.Context) AuthContext {
	raw := ginmw.GetAuthContext(c)
	orgID := raw.OrgID
	if orgID == "" {
		if v, ok := c.Get(ctxkeys.CtxKeyTenantID); ok {
			if s, ok := v.(string); ok {
				orgID = s
			}
		}
	}
	return AuthContext{
		OrgID:   orgID,
		Actor:      raw.Actor,
		Role:       raw.Role,
		Scopes:     raw.Scopes,
		AuthMethod: raw.AuthMethod,
	}
}

// ParseAuthTenantID extrae y parsea el tenant_id del auth context.
func ParseAuthTenantID(c *gin.Context) (uuid.UUID, bool) {
	auth := GetAuthContext(c)
	orgID, err := uuid.Parse(auth.OrgID)
	if err != nil {
		return uuid.Nil, false
	}
	return orgID, true
}
