package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	platformiam "github.com/devpablocristo/platform/iam/go"
	platformidempotency "github.com/devpablocristo/platform/idempotency/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/jackc/pgx/v5"
)

type idempotencyScopeContextKey struct{}

// IAMIdempotency verifies the current identity and local admission before the
// generic platform middleware may claim or replay a product command.
type IAMIdempotency struct {
	middleware       *platformidempotency.Middleware
	verifier         ClerkSessionVerifier
	identityVerifier ClerkIdentityVerifier
	transactor       SessionTransactor
}

func NewIAMIdempotency(
	store platformidempotency.Store,
	verifier ClerkSessionVerifier,
	identityVerifier ClerkIdentityVerifier,
	transactor SessionTransactor,
) (*IAMIdempotency, error) {
	if store == nil || verifier == nil || identityVerifier == nil || transactor == nil {
		return nil, errors.New("IAM idempotency dependencies are required")
	}
	config := platformidempotency.DefaultMiddlewareConfig()
	config.Scope = func(request *http.Request) (string, error) {
		scope, ok := request.Context().Value(idempotencyScopeContextKey{}).(string)
		if !ok || strings.TrimSpace(scope) == "" {
			return "", platformidempotency.ErrScopeRequired
		}
		return scope, nil
	}
	config.WriteError = writeIAMIdempotencyError
	middleware, err := platformidempotency.NewMiddleware(store, config)
	if err != nil {
		return nil, fmt.Errorf("configure IAM idempotency: %w", err)
	}
	return &IAMIdempotency{
		middleware:       middleware,
		verifier:         verifier,
		identityVerifier: identityVerifier,
		transactor:       transactor,
	}, nil
}

func (middleware *IAMIdempotency) Wrap(next http.Handler) http.Handler {
	if next == nil {
		panic("IAM idempotency next handler is nil")
	}
	if middleware == nil || middleware.middleware == nil {
		return next
	}
	idempotent := middleware.middleware.Wrap(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isIAMCommand(r) {
			next.ServeHTTP(w, r)
			return
		}
		keys := r.Header.Values(platformidempotency.DefaultHeaderName)
		if len(keys) != 1 {
			writeAPIError(
				w,
				http.StatusBadRequest,
				"REQUEST_INVALID",
				"exactly one Idempotency-Key header is required",
			)
			return
		}
		if _, valid := validateIdempotencyKey(w, api.IdempotencyKey(keys[0])); !valid {
			return
		}
		token, err := bearerToken(r)
		if err != nil {
			writeAPIError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", err.Error())
			return
		}

		scope, ok := middleware.authorizeScope(w, r, token)
		if !ok {
			return
		}
		idempotent.ServeHTTP(
			w,
			r.WithContext(context.WithValue(r.Context(), idempotencyScopeContextKey{}, scope)),
		)
	})
}

func (middleware *IAMIdempotency) authorizeScope(
	w http.ResponseWriter,
	r *http.Request,
	token string,
) (string, bool) {
	if isIdentityScopedCommand(r) {
		claims, err := middleware.identityVerifier.VerifyIdentity(r.Context(), token)
		if err != nil {
			writeIAMVerificationError(w, err)
			return "", false
		}
		return "pymes-v2:iam:user:" + claims.Subject, true
	}

	claims, err := middleware.verifier.VerifySession(r.Context(), token)
	if err != nil {
		writeIAMVerificationError(w, err)
		return "", false
	}
	verified := platformiam.VerifiedSession{
		Provider:               "clerk",
		Subject:                claims.Subject,
		SessionID:              claims.SessionID,
		ExternalOrganizationID: claims.OrganizationID,
		ProviderRole:           claims.OrganizationRole,
		ProviderPermissions:    claims.OrganizationPermissions,
		IssuedAt:               claims.IssuedAt,
		ExpiresAt:              claims.ExpiresAt,
	}
	var active platformiam.ActiveMembership
	err = middleware.transactor.WithinSessionTx(
		r.Context(),
		verified,
		func(
			_ context.Context,
			_ pgx.Tx,
			membership platformiam.ActiveMembership,
		) error {
			active = membership
			return nil
		},
	)
	if errors.Is(err, platformiam.ErrActiveMembershipRequired) {
		writeAPIError(
			w,
			http.StatusForbidden,
			"IAM_MEMBERSHIP_REQUIRED",
			"An active local membership is required",
		)
		return "", false
	}
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "IAM_UNAVAILABLE", "IAM is unavailable")
		return "", false
	}
	return "pymes-v2:iam:org:" + active.OrganizationID + ":user:" + active.UserID, true
}

func isIAMCommand(r *http.Request) bool {
	if r == nil || !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		return false
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isIdentityScopedCommand(r *http.Request) bool {
	if r == nil {
		return false
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
		return r.Method == http.MethodPost ||
			r.Method == http.MethodPatch ||
			r.Method == http.MethodDelete
	}
	return r.Method == http.MethodDelete &&
		strings.HasPrefix(r.URL.Path, "/api/v1/sessions/")
}

func writeIAMVerificationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, clerkadapter.ErrOrganizationRequired):
		writeAPIError(
			w,
			http.StatusForbidden,
			"AUTH_ORGANIZATION_REQUIRED",
			"An active organization is required",
		)
	case errors.Is(err, clerkadapter.ErrPendingSession),
		errors.Is(err, clerkadapter.ErrInvalidSessionToken):
		writeAPIError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Invalid session token")
	default:
		writeAPIError(
			w,
			http.StatusServiceUnavailable,
			"AUTH_PROVIDER_UNAVAILABLE",
			"Authentication provider is temporarily unavailable",
		)
	}
}

func writeIAMIdempotencyError(
	w http.ResponseWriter,
	_ *http.Request,
	err error,
) {
	switch {
	case errors.Is(err, platformidempotency.ErrFingerprintMismatch):
		writeAPIError(
			w,
			http.StatusConflict,
			"IDEMPOTENCY_KEY_CONFLICT",
			"Idempotency-Key was already used for a different command",
		)
	case errors.Is(err, platformidempotency.ErrInProgress):
		w.Header().Set("Retry-After", "1")
		writeAPIError(
			w,
			http.StatusConflict,
			"IDEMPOTENCY_IN_PROGRESS",
			"An equivalent command is still in progress",
		)
	case errors.Is(err, platformidempotency.ErrKeyRequired),
		errors.Is(err, platformidempotency.ErrInvalidKey),
		errors.Is(err, platformidempotency.ErrRequestTooLarge),
		errors.Is(err, platformidempotency.ErrResponseTooLarge):
		writeAPIError(w, http.StatusBadRequest, "REQUEST_INVALID", err.Error())
	default:
		writeAPIError(w, http.StatusServiceUnavailable, "IAM_UNAVAILABLE", "IAM is unavailable")
	}
}
