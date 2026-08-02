package domain

import (
	"encoding/base64"
	"strings"
)

const maxOAuthOrganizationIDLength = 255

// RoutedOAuthState keeps only a base64url routing hint next to cryptographic
// entropy. The complete value is still hashed and validated against the
// durable tenant, actor, session, expiry and consumed-at record.
func RoutedOAuthState(organizationID string, entropy []byte) (string, error) {
	if organizationID == "" ||
		len(organizationID) > maxOAuthOrganizationIDLength ||
		len(entropy) < 32 {
		return "", ErrOAuthStateInvalid
	}
	return base64.RawURLEncoding.EncodeToString([]byte(organizationID)) +
		"." + base64.RawURLEncoding.EncodeToString(entropy), nil
}

// OrganizationFromOAuthState returns an untrusted routing hint. Callers must
// validate the full state against its durable one-time record before use.
func OrganizationFromOAuthState(state string) (string, error) {
	parts := strings.Split(state, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ErrOAuthStateInvalid
	}
	organization, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(organization) == 0 ||
		len(organization) > maxOAuthOrganizationIDLength {
		return "", ErrOAuthStateInvalid
	}
	entropy, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(entropy) < 32 {
		return "", ErrOAuthStateInvalid
	}
	return string(organization), nil
}
