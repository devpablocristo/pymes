// Package helpers contains Clerk HTTP-envelope parsing.
package helpers

import (
	"errors"
	"net/http"
	"strings"
)

const sessionCookieName = "__session"

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

// SessionToken accepts Clerk's supported transports: Authorization for
// cross-origin API calls and __session for same-origin browser navigation.
// A present but malformed Authorization header never falls back to a cookie.
func SessionToken(request *http.Request) (string, error) {
	if request == nil {
		return "", errors.New("session token required")
	}
	if strings.TrimSpace(request.Header.Get("Authorization")) != "" {
		return BearerToken(request.Header)
	}
	var token string
	for _, cookie := range request.Cookies() {
		if cookie.Name != sessionCookieName {
			continue
		}
		value := strings.TrimSpace(cookie.Value)
		if value == "" || token != "" {
			return "", errors.New("unambiguous session cookie required")
		}
		token = value
	}
	if token == "" {
		return "", errors.New("session token required")
	}
	return token, nil
}
