package wire

import (
	"context"
	"fmt"
	"strings"

	saasjwks "github.com/devpablocristo/core/authn/go/jwks"
)

// normalizeIssuerURL alinea issuers OIDC/Clerk (barra final opcional).
func normalizeIssuerURL(raw string) string {
	raw = strings.TrimSpace(raw)
	return strings.TrimSuffix(raw, "/")
}

func stringClaim(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// clerkEmailFromClaims extrae email típico de tokens de sesión Clerk.
func clerkEmailFromClaims(m map[string]any) string {
	if s := stringClaim(m, "email"); s != "" {
		return s
	}
	raw, ok := m["email_addresses"]
	if !ok || raw == nil {
		return ""
	}
	arr, ok := raw.([]any)
	if !ok {
		return ""
	}
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if s := stringClaim(obj, "email_address"); s != "" {
			return s
		}
	}
	return ""
}

// clerkDisplayNameFromClaims arma un nombre legible desde claims habituales de Clerk.
func clerkDisplayNameFromClaims(m map[string]any) string {
	if s := stringClaim(m, "name"); s != "" {
		return s
	}
	first := stringClaim(m, "first_name")
	if first == "" {
		first = stringClaim(m, "given_name")
	}
	last := stringClaim(m, "last_name")
	if last == "" {
		last = stringClaim(m, "family_name")
	}
	combined := strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
	if combined != "" {
		return combined
	}
	if s := stringClaim(m, "username"); s != "" {
		return s
	}
	return ""
}

// verifyJWTClaimsMap valida firma (JWKS) y opcionalmente el issuer.
func verifyJWTClaimsMap(ctx context.Context, rawToken, jwksURL, expectedIssuer string) (map[string]any, error) {
	jwksURL = strings.TrimSpace(jwksURL)
	if jwksURL == "" {
		return nil, fmt.Errorf("jwks url is required")
	}
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, fmt.Errorf("token is required")
	}
	verifier := saasjwks.NewVerifier(jwksURL)
	claims, err := verifier.VerifyToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	if expectedIssuer != "" {
		iss := normalizeIssuerURL(stringClaim(claims, "iss"))
		want := normalizeIssuerURL(expectedIssuer)
		if iss != "" && want != "" && iss != want {
			return nil, fmt.Errorf("invalid token issuer")
		}
	}
	sub := stringClaim(claims, "sub")
	if sub == "" {
		return nil, fmt.Errorf("missing sub claim")
	}
	return claims, nil
}

func placeholderClerkEmail(externalID string) string {
	ext := strings.TrimSpace(externalID)
	if ext == "" {
		ext = "unknown"
	}
	return fmt.Sprintf("%s@users.clerk.placeholder", ext)
}
