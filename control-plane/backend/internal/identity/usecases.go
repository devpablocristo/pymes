package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Principal struct {
	OrgID  string   `json:"org_id"`
	Actor  string   `json:"actor"`
	Role   string   `json:"role"`
	Scopes []string `json:"scopes"`
}

type JWKSVerifier interface {
	VerifyToken(ctx context.Context, tokenString string) (*jwt.Token, error)
}

type Usecases struct {
	verifier JWKSVerifier
	issuer   string
}

func NewUsecases(verifier JWKSVerifier, issuer string) *Usecases {
	return &Usecases{verifier: verifier, issuer: issuer}
}

func (u *Usecases) ResolvePrincipal(ctx context.Context, token string) (Principal, error) {
	if strings.TrimSpace(token) == "" {
		return Principal{}, errors.New("token is required")
	}
	if u.verifier == nil {
		return Principal{}, errors.New("jwt verifier is not configured")
	}

	parsed, err := u.verifier.VerifyToken(ctx, token)
	if err != nil {
		return Principal{}, fmt.Errorf("verify token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Principal{}, errors.New("invalid jwt claims")
	}

	if u.issuer != "" {
		iss, _ := claims["iss"].(string)
		if iss != "" && iss != u.issuer {
			return Principal{}, fmt.Errorf("invalid issuer: %s", iss)
		}
	}

	principal := Principal{
		OrgID: getStringClaim(claims, "org_id"),
		Actor: getStringClaim(claims, "sub"),
		Role:  getStringClaim(claims, "org_role"),
	}
	principal.Scopes = resolveScopes(claims)

	if principal.Actor == "" {
		return Principal{}, errors.New("missing sub claim")
	}
	if principal.Role == "" {
		principal.Role = "member"
	}
	return principal, nil
}

func getStringClaim(claims jwt.MapClaims, key string) string {
	v, ok := claims[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
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
