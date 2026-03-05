package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/control-plane/backend/internal/identity"
	"github.com/devpablocristo/pymes/control-plane/backend/pkg/types"
)

type APIKeyResolver interface {
	ResolveAPIKey(raw string) (ResolvedKey, bool)
}

type ResolvedKey struct {
	ID     uuid.UUID
	OrgID  uuid.UUID
	Scopes []string
}

type AuthMiddleware struct {
	identity     *identity.Usecases
	keyResolver  APIKeyResolver
	authEnableJWT bool
	authAllowKey  bool
}

func NewAuthMiddleware(identityUC *identity.Usecases, keyResolver APIKeyResolver, authEnableJWT, authAllowKey bool) *AuthMiddleware {
	return &AuthMiddleware{
		identity:      identityUC,
		keyResolver:   keyResolver,
		authEnableJWT: authEnableJWT,
		authAllowKey:  authAllowKey,
	}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.authEnableJWT {
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
			if rawKey != "" {
				key, ok := m.keyResolver.ResolveAPIKey(rawKey)
				if ok {
				actor := sanitizeHeader(c.GetHeader("X-Actor"), 128)
				if actor == "" {
					actor = "api_key:" + key.ID.String()
				}
				role := sanitizeHeader(c.GetHeader("X-Role"), 32)
				if role == "" {
					role = "service"
				}
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

func sanitizeHeader(v string, maxLen int) string {
	v = strings.TrimSpace(v)
	if len(v) > maxLen {
		v = v[:maxLen]
	}
	for _, ch := range v {
		if ch < 32 || ch == 127 {
			return ""
		}
	}
	return v
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
