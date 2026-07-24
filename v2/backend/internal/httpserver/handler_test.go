package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type readinessFunc func(context.Context) error

func (fn readinessFunc) Ping(ctx context.Context) error { return fn(ctx) }

func TestHealthz(t *testing.T) {
	handler := NewHandler(discardLogger(), nil, time.Second)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Request-Id") == "" {
		t.Fatal("missing X-Request-Id response header")
	}
}

func TestReadyzWhenPostgresIsReady(t *testing.T) {
	handler := NewHandler(discardLogger(), readinessFunc(func(context.Context) error { return nil }), time.Second)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	request.Header.Set("X-Request-Id", "caller-request-id")
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ready\"}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Request-Id") != "caller-request-id" {
		t.Fatalf("request id = %q", response.Header().Get("X-Request-Id"))
	}
}

func TestReadyzWhenPostgresIsUnavailable(t *testing.T) {
	handler := NewHandler(discardLogger(), readinessFunc(func(context.Context) error {
		return errors.New("database unavailable")
	}), time.Second)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "{\"status\":\"unavailable\"}\n" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestHealthEndpointsRejectOtherMethods(t *testing.T) {
	handler := NewHandler(discardLogger(), nil, time.Second)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/healthz", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestRuntimeConfigFailsClosedWithoutClerk(t *testing.T) {
	response := httptest.NewRecorder()
	NewHandler(discardLogger(), nil, time.Second).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/runtime-config", nil),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body api.RuntimeConfig
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Auth.Configured || body.Auth.PublishableKey != nil {
		t.Fatalf("unexpected config: %+v", body)
	}
}

func TestRuntimeConfigExposesOnlyPublishableClerkKey(t *testing.T) {
	clerk := config.ClerkConfig{
		PublishableKey:    "pk_test_public",
		SecretKey:         "sk_test_secret",
		Issuer:            "https://issuer.example",
		AuthorizedParties: []string{"http://localhost"},
	}
	handler := NewHandlerWithIAM(discardLogger(), nil, time.Second, NewIAMAPI(clerk))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/runtime-config", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if strings.Contains(response.Body.String(), clerk.SecretKey) {
		t.Fatal("response leaked Clerk secret key")
	}
	if !strings.Contains(response.Body.String(), clerk.PublishableKey) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestIAMEndpointsFailClosedWhenClerkIsMissing(t *testing.T) {
	response := httptest.NewRecorder()
	NewHandler(discardLogger(), nil, time.Second).ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/session", nil),
	)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "AUTH_NOT_CONFIGURED") {
		t.Fatalf("body = %s", response.Body.String())
	}
}

type clerkSessionVerifierFunc func(context.Context, string) (clerkadapter.SessionClaims, error)

func (fn clerkSessionVerifierFunc) VerifySession(
	ctx context.Context,
	token string,
) (clerkadapter.SessionClaims, error) {
	return fn(ctx, token)
}

type sessionTransactorFunc func(
	context.Context,
	platformiam.VerifiedSession,
	platformiam.SessionTxFunc,
) error

func (fn sessionTransactorFunc) WithinSessionTx(
	ctx context.Context,
	session platformiam.VerifiedSession,
	callback platformiam.SessionTxFunc,
) error {
	return fn(ctx, session, callback)
}

func TestCurrentSessionRejectsMissingOrMalformedBearer(t *testing.T) {
	tests := []struct {
		name          string
		authorization []string
	}{
		{name: "missing"},
		{name: "wrong scheme", authorization: []string{"Basic credential"}},
		{name: "missing credential", authorization: []string{"Bearer"}},
		{name: "multiple headers", authorization: []string{"Bearer first", "Bearer second"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifierCalled := false
			handler := currentSessionTestHandler(
				clerkSessionVerifierFunc(func(context.Context, string) (clerkadapter.SessionClaims, error) {
					verifierCalled = true
					return clerkadapter.SessionClaims{}, nil
				}),
				sessionTransactorFunc(func(
					context.Context,
					platformiam.VerifiedSession,
					platformiam.SessionTxFunc,
				) error {
					t.Fatal("transactor must not be called for an invalid bearer")
					return nil
				}),
			)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
			for _, value := range test.authorization {
				request.Header.Add("Authorization", value)
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			assertAPIError(t, response, http.StatusUnauthorized, "AUTH_INVALID_TOKEN")
			if verifierCalled {
				t.Fatal("verifier was called before validating the bearer header")
			}
		})
	}
}

func TestCurrentSessionRejectsInvalidClerkToken(t *testing.T) {
	transactorCalled := false
	handler := currentSessionTestHandler(
		clerkSessionVerifierFunc(func(_ context.Context, token string) (clerkadapter.SessionClaims, error) {
			if token != "invalid-token" {
				t.Fatalf("token = %q", token)
			}
			return clerkadapter.SessionClaims{}, fmt.Errorf(
				"verification failed: %w",
				clerkadapter.ErrInvalidSessionToken,
			)
		}),
		sessionTransactorFunc(func(
			context.Context,
			platformiam.VerifiedSession,
			platformiam.SessionTxFunc,
		) error {
			transactorCalled = true
			return nil
		}),
	)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	request.Header.Set("Authorization", "Bearer invalid-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	assertAPIError(t, response, http.StatusUnauthorized, "AUTH_INVALID_TOKEN")
	if transactorCalled {
		t.Fatal("transactor was called for an invalid Clerk token")
	}
}

func TestCurrentSessionReportsClerkVerificationOutageAsUnavailable(t *testing.T) {
	transactorCalled := false
	handler := currentSessionTestHandler(
		clerkSessionVerifierFunc(func(
			context.Context,
			string,
		) (clerkadapter.SessionClaims, error) {
			return clerkadapter.SessionClaims{}, fmt.Errorf(
				"JWKS retrieval failed: %w",
				clerkadapter.ErrProviderUnavailable,
			)
		}),
		sessionTransactorFunc(func(
			context.Context,
			platformiam.VerifiedSession,
			platformiam.SessionTxFunc,
		) error {
			transactorCalled = true
			return nil
		}),
	)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	request.Header.Set("Authorization", "Bearer temporarily-unverifiable-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	assertAPIError(
		t,
		response,
		http.StatusServiceUnavailable,
		"AUTH_PROVIDER_UNAVAILABLE",
	)
	if transactorCalled {
		t.Fatal("transactor was called while Clerk verification was unavailable")
	}
}

func TestCurrentSessionRequiresActiveOrganization(t *testing.T) {
	transactorCalled := false
	handler := currentSessionTestHandler(
		clerkSessionVerifierFunc(func(context.Context, string) (clerkadapter.SessionClaims, error) {
			return clerkadapter.SessionClaims{}, clerkadapter.ErrOrganizationRequired
		}),
		sessionTransactorFunc(func(
			context.Context,
			platformiam.VerifiedSession,
			platformiam.SessionTxFunc,
		) error {
			transactorCalled = true
			return nil
		}),
	)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	request.Header.Set("Authorization", "Bearer token-without-organization")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	assertAPIError(t, response, http.StatusForbidden, "AUTH_ORGANIZATION_REQUIRED")
	if transactorCalled {
		t.Fatal("transactor was called without an active Clerk organization")
	}
}

func TestCurrentSessionRequiresActiveLocalMembership(t *testing.T) {
	now := time.Now().UTC()
	claims := clerkadapter.SessionClaims{
		Subject:                 "user_external",
		SessionID:               "sess_external",
		OrganizationID:          "org_external",
		OrganizationRole:        "org:admin",
		OrganizationPermissions: []string{"org:team:read"},
		IssuedAt:                now.Add(-time.Minute),
		ExpiresAt:               now.Add(time.Minute),
	}
	handler := currentSessionTestHandler(
		clerkSessionVerifierFunc(func(context.Context, string) (clerkadapter.SessionClaims, error) {
			return claims, nil
		}),
		sessionTransactorFunc(func(
			_ context.Context,
			session platformiam.VerifiedSession,
			_ platformiam.SessionTxFunc,
		) error {
			assertVerifiedSession(t, session, claims)
			return fmt.Errorf("resolve membership: %w", platformiam.ErrActiveMembershipRequired)
		}),
	)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	assertAPIError(t, response, http.StatusForbidden, "IAM_MEMBERSHIP_REQUIRED")
}

func TestCurrentSessionReturnsEffectiveLocalSession(t *testing.T) {
	now := time.Now().UTC()
	claims := clerkadapter.SessionClaims{
		Subject:                 "user_external",
		SessionID:               "sess_external",
		OrganizationID:          "org_external",
		OrganizationRole:        "org:member",
		OrganizationPermissions: []string{"org:team:read"},
		IssuedAt:                now.Add(-time.Minute),
		ExpiresAt:               now.Add(time.Minute),
	}
	active := platformiam.ActiveMembership{
		MembershipID:   "33333333-3333-3333-3333-333333333333",
		OrganizationID: "22222222-2222-2222-2222-222222222222",
		UserID:         "11111111-1111-1111-1111-111111111111",
		Role:           "owner",
	}
	tx := &currentSessionTx{
		row: currentSessionRow{values: []string{
			active.UserID,
			"ana@example.com",
			"Ana Pérez",
			"https://cdn.example/avatar.png",
			active.OrganizationID,
			"Acme",
			"acme",
			claims.OrganizationID,
			active.MembershipID,
			active.Role,
			"active",
		}},
	}
	handler := currentSessionTestHandler(
		clerkSessionVerifierFunc(func(_ context.Context, token string) (clerkadapter.SessionClaims, error) {
			if token != "valid-token" {
				t.Fatalf("token = %q", token)
			}
			return claims, nil
		}),
		sessionTransactorFunc(func(
			ctx context.Context,
			session platformiam.VerifiedSession,
			callback platformiam.SessionTxFunc,
		) error {
			assertVerifiedSession(t, session, claims)
			return callback(ctx, tx, active)
		}),
	)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body api.CurrentSession
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.SessionId != claims.SessionID {
		t.Fatalf("session id = %q", body.SessionId)
	}
	if body.User.Email != "ana@example.com" || body.User.DisplayName != "Ana Pérez" {
		t.Fatalf("user = %+v", body.User)
	}
	if body.Organization.Name != "Acme" || body.Organization.Slug != "acme" {
		t.Fatalf("organization = %+v", body.Organization)
	}
	if body.Organization.SwitchKey == nil || *body.Organization.SwitchKey != claims.OrganizationID {
		t.Fatalf("organization switch key = %v", body.Organization.SwitchKey)
	}
	if body.Membership.Role != api.RoleOwner || body.Organization.Role != api.RoleOwner {
		t.Fatalf(
			"local roles = membership %q, organization %q",
			body.Membership.Role,
			body.Organization.Role,
		)
	}
	if body.Role != api.RoleMember {
		t.Fatalf("effective role = %q", body.Role)
	}
	permissionNames := make([]string, len(body.Permissions))
	for index, permission := range body.Permissions {
		permissionNames[index] = string(permission)
	}
	if got := strings.Join(permissionNames, ","); got !=
		"organization:view,team:view,sessions:manage:self" {
		t.Fatalf("permissions = %q", got)
	}
}

func currentSessionTestHandler(
	verifier ClerkSessionVerifier,
	transactor SessionTransactor,
) http.Handler {
	clerk := config.ClerkConfig{
		PublishableKey: "pk_test_public",
		SecretKey:      "sk_test_secret",
		Issuer:         "https://issuer.example",
	}
	return NewHandlerWithIAM(
		discardLogger(),
		nil,
		time.Second,
		NewIAMAPI(clerk, IAMDependencies{
			Verifier:   verifier,
			Transactor: transactor,
		}),
	)
}

func assertAPIError(
	t *testing.T,
	response *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, wantStatus, response.Body.String())
	}
	var body api.ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q", body.Error.Code, wantCode)
	}
	if body.Error.RequestId == nil || strings.TrimSpace(*body.Error.RequestId) == "" {
		t.Fatal("error response is missing request_id")
	}
}

func assertVerifiedSession(
	t *testing.T,
	got platformiam.VerifiedSession,
	want clerkadapter.SessionClaims,
) {
	t.Helper()
	if got.Provider != "clerk" ||
		got.Subject != want.Subject ||
		got.SessionID != want.SessionID ||
		got.ExternalOrganizationID != want.OrganizationID ||
		got.ProviderRole != want.OrganizationRole ||
		!got.IssuedAt.Equal(want.IssuedAt) ||
		!got.ExpiresAt.Equal(want.ExpiresAt) ||
		strings.Join(got.ProviderPermissions, ",") != strings.Join(want.OrganizationPermissions, ",") {
		t.Fatalf("verified session = %+v; Clerk claims = %+v", got, want)
	}
}

type currentSessionTx struct {
	row pgx.Row
}

func (tx *currentSessionTx) Begin(context.Context) (pgx.Tx, error) {
	panic("unexpected Begin")
}

func (tx *currentSessionTx) Commit(context.Context) error {
	panic("unexpected Commit")
}

func (tx *currentSessionTx) Rollback(context.Context) error {
	panic("unexpected Rollback")
}

func (tx *currentSessionTx) CopyFrom(
	context.Context,
	pgx.Identifier,
	[]string,
	pgx.CopyFromSource,
) (int64, error) {
	panic("unexpected CopyFrom")
}

func (tx *currentSessionTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	panic("unexpected SendBatch")
}

func (tx *currentSessionTx) LargeObjects() pgx.LargeObjects {
	panic("unexpected LargeObjects")
}

func (tx *currentSessionTx) Prepare(
	context.Context,
	string,
	string,
) (*pgconn.StatementDescription, error) {
	panic("unexpected Prepare")
}

func (tx *currentSessionTx) Exec(
	context.Context,
	string,
	...any,
) (pgconn.CommandTag, error) {
	panic("unexpected Exec")
}

func (tx *currentSessionTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (tx *currentSessionTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return tx.row
}

func (tx *currentSessionTx) Conn() *pgx.Conn {
	panic("unexpected Conn")
}

type currentSessionRow struct {
	values []string
}

func (row currentSessionRow) Scan(destinations ...any) error {
	if len(destinations) != len(row.values) {
		return fmt.Errorf(
			"scan destination count = %d, want %d",
			len(destinations),
			len(row.values),
		)
	}
	for index, destination := range destinations {
		value, ok := destination.(*string)
		if !ok {
			return fmt.Errorf("scan destination %d has type %T, want *string", index, destination)
		}
		*value = row.values[index]
	}
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
