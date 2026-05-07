package wire

import (
	"context"
	"net/http"
	"strings"

	authn "github.com/devpablocristo/core/authn/go"
	"github.com/devpablocristo/core/errors/go/domainerr"
)

type tenantPrincipal struct {
	TenantID   string
	Actor      string
	Role       string
	Scopes     []string
	AuthMethod string
}

type tenantPrincipalKey struct{}

func tenantPrincipalFromContext(ctx context.Context) (tenantPrincipal, bool) {
	if ctx == nil {
		return tenantPrincipal{}, false
	}
	p, ok := ctx.Value(tenantPrincipalKey{}).(tenantPrincipal)
	return p, ok
}

func contextWithTenantPrincipal(ctx context.Context, principal tenantPrincipal) context.Context {
	return context.WithValue(ctx, tenantPrincipalKey{}, principal)
}

type tenantPrincipalVerifier interface {
	Verify(ctx context.Context, credential string) (tenantPrincipal, error)
}

func newTenantAuthMiddleware(jwtVerifier, apiKeyVerifier tenantPrincipalVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if raw, ok := authn.BearerToken(r.Header.Get("Authorization")); ok && strings.TrimSpace(raw) != "" {
				if jwtVerifier == nil {
					http.Error(w, "jwt auth is not configured", http.StatusUnauthorized)
					return
				}
				principal, err := jwtVerifier.Verify(r.Context(), raw)
				if err != nil {
					writeTenantAuthError(w, err)
					return
				}
				next.ServeHTTP(w, r.WithContext(contextWithTenantPrincipal(r.Context(), principal)))
				return
			}
			if raw := strings.TrimSpace(r.Header.Get("X-API-KEY")); raw != "" {
				if apiKeyVerifier == nil {
					http.Error(w, "api key auth is not configured", http.StatusUnauthorized)
					return
				}
				principal, err := apiKeyVerifier.Verify(r.Context(), raw)
				if err != nil {
					writeTenantAuthError(w, err)
					return
				}
				next.ServeHTTP(w, r.WithContext(contextWithTenantPrincipal(r.Context(), principal)))
				return
			}
			http.Error(w, "authentication required", http.StatusUnauthorized)
		})
	}
}

func writeTenantAuthError(w http.ResponseWriter, err error) {
	if err == nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	if domainerr.IsForbidden(err) {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Error(w, err.Error(), http.StatusUnauthorized)
}
