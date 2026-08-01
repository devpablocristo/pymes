// Package helpers contains transport-only matching for the preflight gate.
package helpers

import (
	"crypto/subtle"
	"net"
	"strings"
)

const Header = "X-Pymes-Preflight-Token"

// TaggedHost reports whether Cloud Run routed the request through the exact
// candidate tag. A caller cannot select a tagged revision through the stable
// service hostname.
func TaggedHost(host, tag string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	return tag != "" && strings.HasPrefix(host, strings.ToLower(tag)+"---")
}

// TokenMatches compares the capability without timing-dependent early exits.
func TokenMatches(actual, expected string) bool {
	if len(actual) != len(expected) || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
