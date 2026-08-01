// Package helpers contains database-to-domain mappings for identity.
package helpers

import "strings"

// LocalRole maps Clerk roles to the stable local role vocabulary.
func LocalRole(value string) string {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "org:") {
	case "owner":
		return "owner"
	case "admin":
		return "admin"
	case "viewer":
		return "viewer"
	default:
		return "member"
	}
}
