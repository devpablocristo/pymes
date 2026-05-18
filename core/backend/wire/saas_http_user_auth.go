package wire

import (
	"context"
	"strings"

	authn "github.com/devpablocristo/core/authn/go"
	"github.com/devpablocristo/core/errors/go/domainerr"
)

type clerkAuthenticatedUser struct {
	ExternalID string
	Email      string
	Name       string
	AvatarURL  *string
}

func authenticatedClerkUser(ctx context.Context, authorization string, httpAuth pymesSaaSHTTPAuth) (clerkAuthenticatedUser, error) {
	raw, ok := authn.BearerToken(authorization)
	if !ok || strings.TrimSpace(raw) == "" {
		return clerkAuthenticatedUser{}, domainerr.Unauthorized("bearer token is required")
	}
	claims, err := verifyJWTClaimsMap(ctx, raw, httpAuth.JWKSURL, httpAuth.JWTIssuer)
	if err != nil {
		return clerkAuthenticatedUser{}, domainerr.Unauthorized("invalid bearer token")
	}
	sub := stringClaim(claims, "sub")
	if sub == "" {
		return clerkAuthenticatedUser{}, domainerr.Unauthorized("missing user claim")
	}
	email := clerkEmailFromClaims(claims)
	if email == "" {
		email = placeholderClerkEmail(sub)
	}
	name := clerkDisplayNameFromClaims(claims)
	if name == "" {
		name = email
	}
	var avatar *string
	if image := strings.TrimSpace(stringClaim(claims, "image_url")); image != "" {
		avatar = &image
	}
	return clerkAuthenticatedUser{
		ExternalID: sub,
		Email:      email,
		Name:       name,
		AvatarURL:  avatar,
	}, nil
}
