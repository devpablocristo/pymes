package wire

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	authn "github.com/devpablocristo/core/authn/go"
	"github.com/devpablocristo/core/errors/go/domainerr"
	sharedauth "github.com/devpablocristo/pymes/core/shared/backend/auth"
	"github.com/google/uuid"
)

type tenantPrincipal struct {
	OrgID   string
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
type tenantMembershipResolver func(ctx context.Context, orgID uuid.UUID, actor string) (string, bool, error)

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

func withTenantSlugBinding(authMW func(http.Handler) http.Handler, resolve tenantRefResolver, membership tenantMembershipResolver) func(http.Handler) http.Handler {
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
			bound, ok := tenantSlugMatchesPrincipal(r.Context(), r.Header.Get(sharedauth.TenantSlugHeader), principal, resolve, membership, w)
			if !ok {
				return
			}
			next.ServeHTTP(w, r.WithContext(contextWithTenantPrincipal(r.Context(), bound)))
		}))
	}
}

func tenantSlugMatchesPrincipal(ctx context.Context, rawSlug string, principal tenantPrincipal, resolve tenantRefResolver, membership tenantMembershipResolver, w http.ResponseWriter) (tenantPrincipal, bool) {
	slug := strings.TrimSpace(rawSlug)
	authMethod := strings.TrimSpace(principal.AuthMethod)
	if slug == "" {
		if authMethod == "api_key" {
			return principal, true
		}
		writeTenantJSONError(w, http.StatusForbidden, "tenant_slug_required", "tenant slug header is required")
		return tenantPrincipal{}, false
	}
	resolvedOrgID, ok, err := resolve(ctx, slug)
	if err != nil {
		writeTenantJSONError(w, http.StatusForbidden, "tenant_mismatch", "tenant slug is not valid for this session")
		return tenantPrincipal{}, false
	}
	if !ok {
		writeTenantJSONError(w, http.StatusForbidden, "tenant_mismatch", "tenant slug is not valid for this session")
		return tenantPrincipal{}, false
	}
	if strings.EqualFold(strings.TrimSpace(principal.OrgID), resolvedOrgID.String()) {
		return principal, true
	}
	_ = membership
	writeTenantJSONError(w, http.StatusForbidden, "tenant_mismatch", "tenant slug is not valid for this session")
	return tenantPrincipal{}, false
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
	// No exponer err.Error() al cliente: puede contener detalles internos del
	// JWT verifier o JWKS (CLAUDE.md sec 8). Loguear el detalle y devolver
	// un mensaje genérico al caller HTTP.
	if domainerr.IsForbidden(err) {
		slog.Default().Warn("tenant auth forbidden", "error", err.Error())
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	slog.Default().Warn("tenant auth unauthorized", "error", err.Error())
	http.Error(w, "authentication failed", http.StatusUnauthorized)
}
