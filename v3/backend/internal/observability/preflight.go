// architecture:adapter external
package observability

import (
	"net/http"

	preflighthelpers "github.com/devpablocristo/pymes/v3/backend/internal/observability/preflight/helpers"
	preflightmodels "github.com/devpablocristo/pymes/v3/backend/internal/observability/preflight/models"
)

// PreflightGate prevents a zero-traffic tagged API revision from becoming a
// public canary. Stable service traffic remains unchanged, while the release
// verifier must present the per-release capability on the tagged hostname.
func PreflightGate(next http.Handler, config preflightmodels.Config) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if preflighthelpers.TaggedHost(request.Host, config.Tag) &&
			!preflighthelpers.TokenMatches(
				request.Header.Get(preflighthelpers.Header),
				config.Token,
			) {
			response.Header().Set("Cache-Control", "no-store")
			http.NotFound(response, request)
			return
		}
		next.ServeHTTP(response, request)
	})
}
