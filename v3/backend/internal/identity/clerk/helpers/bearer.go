// Package helpers contains Clerk HTTP-envelope parsing.
package helpers

import (
	"errors"
	"net/http"
	"strings"
)

// BearerToken returns the exact non-empty session token from Authorization.
func BearerToken(header http.Header) (string, error) {
	value := strings.TrimSpace(header.Get("Authorization"))
	if !strings.HasPrefix(value, "Bearer ") {
		return "", errors.New("bearer token required")
	}
	token := strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
	if token == "" {
		return "", errors.New("bearer token required")
	}
	return token, nil
}
