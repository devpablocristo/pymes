package wire

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	authn "github.com/devpablocristo/core/authn/go"
	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/google/uuid"
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

type tenantRefResolver func(ctx context.Context, ref string) (uuid.UUID, bool, error)

const tenantSlugHeader = "X-Pymes-Tenant-Slug"

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

func withTenantSlugBinding(authMW func(http.Handler) http.Handler, resolve tenantRefResolver) func(http.Handler) http.Handler {
	if authMW == nil || resolve == nil {
		return authMW
	}
	return func(next http.Handler) http.Handler {
		return authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := tenantPrincipalFromContext(r.Context())
			if !ok {
				writeTenantJSONError(w, http.StatusUnauthorized, "authentication_required", "authentication required")
				return
			}
			if !tenantSlugMatchesPrincipal(r.Context(), r.Header.Get(tenantSlugHeader), principal, resolve, w) {
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

func tenantSlugMatchesPrincipal(ctx context.Context, rawSlug string, principal tenantPrincipal, resolve tenantRefResolver, w http.ResponseWriter) bool {
	slug := strings.TrimSpace(rawSlug)
	authMethod := strings.TrimSpace(principal.AuthMethod)
	if slug == "" {
		if authMethod == "api_key" {
			return true
		}
		writeTenantJSONError(w, http.StatusForbidden, "tenant_slug_required", "tenant slug header is required")
		return false
	}
	resolvedTenantID, ok, err := resolve(ctx, slug)
	if err != nil {
		writeTenantJSONError(w, http.StatusForbidden, "tenant_mismatch", "tenant slug is not valid for this session")
		return false
	}
	if !ok || !strings.EqualFold(strings.TrimSpace(principal.TenantID), resolvedTenantID.String()) {
		writeTenantJSONError(w, http.StatusForbidden, "tenant_mismatch", "tenant slug is not valid for this session")
		return false
	}
	return true
}

func writeTenantJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": message,
	})
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
