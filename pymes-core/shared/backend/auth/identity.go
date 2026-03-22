package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Principal struct {
	OrgID  string   `json:"org_id"`
	Actor  string   `json:"actor"`
	Role   string   `json:"role"`
	Scopes []string `json:"scopes"`
}

type TokenVerifier interface {
	VerifyToken(ctx context.Context, tokenString string) (*jwt.Token, error)
}

// OrgRefResolver traduce un identificador de tenant del JWT (p. ej. org_... de Clerk) al UUID interno de orgs.
type OrgRefResolver interface {
	ResolveOrgID(ctx context.Context, ref string) (string, error)
}

// IdentityConfig alinea verticales con core/saas: claims configurables + resolución de org externa.
type IdentityConfig struct {
	Issuer         string
	Audience       string
	OrgClaim       string
	RoleClaim      string
	OrgRefResolver OrgRefResolver
}

type IdentityResolver struct {
	verifier TokenVerifier
	cfg      IdentityConfig
}

func NewIdentityResolver(verifier TokenVerifier, issuer string) *IdentityResolver {
	return NewIdentityResolverWithConfig(verifier, IdentityConfig{Issuer: issuer})
}

func NewIdentityResolverWithConfig(verifier TokenVerifier, cfg IdentityConfig) *IdentityResolver {
	return &IdentityResolver{verifier: verifier, cfg: cfg}
}

func (r *IdentityResolver) ResolvePrincipal(ctx context.Context, token string) (Principal, error) {
	if strings.TrimSpace(token) == "" {
		return Principal{}, errors.New("token is required")
	}
	if r.verifier == nil {
		return Principal{}, errors.New("jwt verifier is not configured")
	}

	parsed, err := r.verifier.VerifyToken(ctx, token)
	if err != nil {
		return Principal{}, fmt.Errorf("verify token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Principal{}, errors.New("invalid jwt claims")
	}

	if r.cfg.Issuer != "" {
		iss, _ := claims["iss"].(string)
		if iss != "" && iss != r.cfg.Issuer {
			return Principal{}, fmt.Errorf("invalid issuer: %s", iss)
		}
	}
	if strings.TrimSpace(r.cfg.Audience) != "" && !audienceMatches(claims["aud"], r.cfg.Audience) {
		return Principal{}, errors.New("invalid token audience")
	}

	orgNames := make([]string, 0, 5)
	if c := strings.TrimSpace(r.cfg.OrgClaim); c != "" {
		orgNames = append(orgNames, c)
	}
	orgNames = append(orgNames, "tenant_id", "org_id", "o.id")

	rawOrg := firstStringClaim(claims, orgNames...)
	if strings.TrimSpace(rawOrg) == "" {
		rawOrg = clerkCompactOrgIDFromClaims(claims)
	}

	roleNames := make([]string, 0, 5)
	if c := strings.TrimSpace(r.cfg.RoleClaim); c != "" {
		roleNames = append(roleNames, c)
	}
	roleNames = append(roleNames, "role", "org_role", "o.rol")

	principal := Principal{
		Actor: getStringClaim(claims, "sub"),
		Role:  normalizeRoleValue(firstStringClaim(claims, roleNames...)),
	}
	principal.Scopes = resolveScopes(claims)

	if principal.Actor == "" {
		return Principal{}, errors.New("missing sub claim")
	}
	if principal.Role == "" {
		principal.Role = "member"
	}

	orgIDStr := strings.TrimSpace(rawOrg)
	if orgIDStr != "" {
		if _, err := uuid.Parse(orgIDStr); err == nil {
			principal.OrgID = orgIDStr
		} else if r.cfg.OrgRefResolver != nil {
			resolved, resErr := r.cfg.OrgRefResolver.ResolveOrgID(ctx, orgIDStr)
			if resErr != nil {
				return Principal{}, fmt.Errorf("resolve org: %w", resErr)
			}
			resolved = strings.TrimSpace(resolved)
			if _, err := uuid.Parse(resolved); err != nil {
				return Principal{}, errors.New("resolved org is not a valid uuid")
			}
			principal.OrgID = resolved
		} else {
			principal.OrgID = orgIDStr
		}
	}

	return principal, nil
}

func audienceMatches(value any, audience string) bool {
	audience = strings.TrimSpace(audience)
	if audience == "" {
		return true
	}
	switch typed := value.(type) {
	case string:
		return typed == audience
	case []any:
		for _, item := range typed {
			if s, ok := item.(string); ok && s == audience {
				return true
			}
		}
	case []string:
		for _, item := range typed {
			if item == audience {
				return true
			}
		}
	}
	return false
}

// clerkCompactOrgIDFromClaims obtiene org_... del claim "o" (session token Clerk v2).
// Ver: https://clerk.com/docs/guides/sessions/session-tokens#organization-claim
func clerkCompactOrgIDFromClaims(claims jwt.MapClaims) string {
	raw, ok := claims["o"]
	if !ok || raw == nil {
		return ""
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	id := strings.TrimSpace(toStringClaim(m["id"]))
	return id
}

func firstStringClaim(claims jwt.MapClaims, names ...string) string {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if value := strings.TrimSpace(claimStringValue(claims, name)); value != "" {
			return value
		}
	}
	return ""
}

func claimStringValue(claims jwt.MapClaims, path string) string {
	return toStringClaim(claimValue(claims, path))
}

func claimValue(claims jwt.MapClaims, path string) any {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if value, ok := claims[path]; ok {
		return value
	}
	parts := strings.Split(path, ".")
	var current any = claims
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil
		}
		node, ok := current.(map[string]any)
		if !ok {
			// jwt.MapClaims values pueden ser map[string]interface{}
			nodeLegacy, ok2 := current.(map[string]interface{})
			if !ok2 {
				return nil
			}
			current, ok = nodeLegacy[part]
			if !ok {
				return nil
			}
			continue
		}
		current, ok = node[part]
		if !ok {
			return nil
		}
	}
	return current
}

func toStringClaim(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%.0f", typed))
	default:
		return ""
	}
}

func normalizeRoleValue(role string) string {
	role = strings.TrimSpace(role)
	role = strings.TrimPrefix(role, "org:")
	return strings.TrimSpace(role)
}

func getStringClaim(claims jwt.MapClaims, key string) string {
	return claimStringValue(claims, key)
}

func resolveScopes(claims jwt.MapClaims) []string {
	for _, key := range []string{"org_permissions", "scopes", "scope"} {
		if v, ok := claims[key]; ok {
			return parseScopes(v)
		}
	}
	return nil
}

func parseScopes(raw any) []string {
	m := make(map[string]struct{})
	out := make([]string, 0)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := m[v]; ok {
			return
		}
		m[v] = struct{}{}
		out = append(out, v)
	}

	switch v := raw.(type) {
	case string:
		for _, part := range strings.Split(v, ",") {
			add(part)
		}
	case []string:
		for _, item := range v {
			add(item)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	}
	return out
}
