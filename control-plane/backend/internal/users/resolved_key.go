// Package users holds minimal types shared with internal APIs after SaaS migration.
// User/org management and API key verification are implemented in saas-core.
package users

import "github.com/google/uuid"

// ResolvedAPIKey is the result of resolving a raw API key (internal service use).
type ResolvedAPIKey struct {
	ID     uuid.UUID
	OrgID  uuid.UUID
	Scopes []string
}
