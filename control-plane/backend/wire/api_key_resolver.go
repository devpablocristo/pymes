package wire

import (
	"github.com/devpablocristo/pymes/control-plane/backend/internal/shared/handlers"
	"github.com/devpablocristo/pymes/control-plane/backend/internal/users"
)

type apiKeyResolverAdapter struct {
	repo *users.Repository
}

func newAPIKeyResolver(repo *users.Repository) handlers.APIKeyResolver {
	return &apiKeyResolverAdapter{repo: repo}
}

func (a *apiKeyResolverAdapter) ResolveAPIKey(raw string) (handlers.ResolvedKey, bool) {
	key, ok := a.repo.ResolveAPIKey(raw)
	if !ok {
		return handlers.ResolvedKey{}, false
	}
	return handlers.ResolvedKey{
		ID:     key.ID,
		OrgID:  key.OrgID,
		Scopes: key.Scopes,
	}, true
}
