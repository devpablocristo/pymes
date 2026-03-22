package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	types "github.com/devpablocristo/core/backend/go/contextkeys"
)

// JWTPrincipal is the minimal claim set resolved from a bearer token.
type JWTPrincipal struct {
	OrgID  string
	Actor  string
	Role   string
	Scopes []string
}

// JWTResolver resolves a bearer JWT to principal claims (optional; may be nil).
type JWTResolver interface {
	ResolvePrincipal(ctx context.Context, token string) (JWTPrincipal, error)
}

type APIKeyResolver interface {
	ResolveAPIKey(raw string) (ResolvedKey, bool)
}

type ResolvedKey struct {
	ID     uuid.UUID
	OrgID  uuid.UUID
	Scopes []string
}

type AuthMiddleware struct {
	identity      JWTResolver
	keyResolver   APIKeyResolver
	authEnableJWT bool
	authAllowKey  bool
}

func NewAuthMiddleware(identity JWTResolver, keyResolver APIKeyResolver, authEnableJWT, authAllowKey bool) *AuthMiddleware {
	return &AuthMiddleware{
		identity:      identity,
		keyResolver:   keyResolver,
		authEnableJWT: authEnableJWT,
		authAllowKey:  authAllowKey,
	}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.authEnableJWT && m.identity != nil {
			header := strings.TrimSpace(c.GetHeader("Authorization"))
			if strings.HasPrefix(strings.ToLower(header), "bearer ") {
				token := strings.TrimSpace(header[len("Bearer "):])
				principal, err := m.identity.ResolvePrincipal(c.Request.Context(), token)
				if err == nil {
					c.Set(types.CtxKeyOrgID, principal.OrgID)
					c.Set(types.CtxKeyActor, principal.Actor)
					c.Set(types.CtxKeyRole, principal.Role)
					c.Set(types.CtxKeyScopes, principal.Scopes)
					c.Set(types.CtxKeyAuthMethod, "jwt")
					c.Next()
					return
				}
			}
		}

		if m.authAllowKey {
			rawKey := strings.TrimSpace(c.GetHeader("X-API-KEY"))
			if rawKey != "" && m.keyResolver != nil {
				key, ok := m.keyResolver.ResolveAPIKey(rawKey)
				if ok {
					actor := "api_key:" + key.ID.String()
					role := "service"
					reqScopes := splitCSV(c.GetHeader("X-Scopes"))
					scopes := key.Scopes
					if len(reqScopes) > 0 {
						scopes = intersectScopes(key.Scopes, reqScopes)
					}
					c.Set(types.CtxKeyOrgID, key.OrgID.String())
					c.Set(types.CtxKeyActor, actor)
					c.Set(types.CtxKeyRole, role)
					c.Set(types.CtxKeyScopes, scopes)
					c.Set(types.CtxKeyAuthMethod, "api_key")
					c.Next()
					return
				}
			}
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		c.Abort()
	}
}

type AuthContext struct {
	OrgID      string   `json:"org_id"`
	Actor      string   `json:"actor"`
	Role       string   `json:"role"`
	Scopes     []string `json:"scopes"`
	AuthMethod string   `json:"auth_method"`
}

func GetAuthContext(c *gin.Context) AuthContext {
	orgID, _ := c.Get(types.CtxKeyOrgID)
	actor, _ := c.Get(types.CtxKeyActor)
	role, _ := c.Get(types.CtxKeyRole)
	scopes, _ := c.Get(types.CtxKeyScopes)
	authMethod, _ := c.Get(types.CtxKeyAuthMethod)

	ctxScopes, _ := scopes.([]string)
	return AuthContext{
		OrgID:      asString(orgID),
		Actor:      asString(actor),
		Role:       asString(role),
		Scopes:     ctxScopes,
		AuthMethod: asString(authMethod),
	}
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	res := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		res = append(res, p)
	}
	return res
}

func intersectScopes(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(a))
	for _, s := range a {
		m[s] = struct{}{}
	}
	res := make([]string, 0)
	for _, s := range b {
		if _, ok := m[s]; ok {
			res = append(res, s)
		}
	}
	return res
}
