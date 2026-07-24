package httpserver

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	teamReadMembershipOne   = "11111111-1111-4111-8111-111111111111"
	teamReadMembershipTwo   = "22222222-2222-4222-8222-222222222222"
	teamReadMembershipThree = "33333333-3333-4333-8333-333333333333"
	teamReadUserOne         = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	teamReadUserTwo         = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	teamReadUserThree       = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	teamReadInvitationOne   = "44444444-4444-4444-8444-444444444444"
	teamReadInvitationTwo   = "55555555-5555-4555-8555-555555555555"
	teamReadInvitationThree = "66666666-6666-4666-8666-666666666666"
)

type teamReadContextKey struct{}

func TestListTeamMembersUsesSessionTransactionAndMapsRows(t *testing.T) {
	claims := teamReadClaims("org:member")
	active := teamReadActiveMembership("owner")
	queryCalls := 0
	rows := &teamReadRows{values: teamReadMemberValues()}
	tx := &teamReadTx{
		query: func(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
			queryCalls++
			if got := ctx.Value(teamReadContextKey{}); got != claims.OrganizationID {
				t.Fatalf("transaction context organization = %v", got)
			}
			if !strings.Contains(query, "FROM iam.memberships") {
				t.Fatalf("query does not read memberships: %s", query)
			}
			if strings.Contains(strings.ToLower(query), "org_id") {
				t.Fatalf("query bypasses transaction RLS with an org_id predicate: %s", query)
			}
			if len(args) != 0 {
				t.Fatalf("query args = %v; org_id must come from transaction RLS", args)
			}
			return rows, nil
		},
	}
	transactionCalls := 0
	handler := newTeamReadTestHandler(
		t,
		claims,
		sessionTransactorFunc(func(
			ctx context.Context,
			session platformiam.VerifiedSession,
			callback platformiam.SessionTxFunc,
		) error {
			transactionCalls++
			assertVerifiedSession(t, session, claims)
			ctx = context.WithValue(ctx, teamReadContextKey{}, claims.OrganizationID)
			return callback(ctx, tx, active)
		}),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		newTeamReadRequest(http.MethodGet, "/api/v1/team/members"),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	body := decodeIAMIdentityResponse[api.MemberList](t, response)
	if body.Page.Total != 2 || body.Page.NextCursor != nil || len(body.Items) != 2 {
		t.Fatalf("member list = %+v", body)
	}
	first := body.Items[0]
	if first.Id.String() != teamReadMembershipOne ||
		first.User.Id.String() != teamReadUserOne ||
		first.User.Email != "owner@example.com" ||
		first.User.DisplayName != "Owner Example" ||
		first.Role != api.RoleOwner ||
		first.Status != api.MembershipStatusActive ||
		first.SyncStatus != api.SyncStatusSynced {
		t.Fatalf("first member = %+v", first)
	}
	if first.User.AvatarUrl == nil ||
		*first.User.AvatarUrl != "https://cdn.example/owner.png" {
		t.Fatalf("first member avatar = %v", first.User.AvatarUrl)
	}
	second := body.Items[1]
	if second.Id.String() != teamReadMembershipTwo ||
		second.User.Id.String() != teamReadUserTwo ||
		second.User.Email != "member@example.com" ||
		second.User.DisplayName != "Member Example" ||
		second.Role != api.RoleMember ||
		second.Status != api.MembershipStatusActive ||
		second.SyncStatus != api.SyncStatusPending {
		t.Fatalf("second member = %+v", second)
	}
	if second.User.AvatarUrl != nil {
		t.Fatalf("second member avatar = %v, want nil", second.User.AvatarUrl)
	}
	if transactionCalls != 1 || queryCalls != 1 || !rows.closed {
		t.Fatalf(
			"transaction calls = %d, query calls = %d, rows closed = %t",
			transactionCalls,
			queryCalls,
			rows.closed,
		)
	}
}

func TestListTeamInvitationsFiltersInsideSessionTransactionAndMapsRows(t *testing.T) {
	claims := teamReadClaims("org:admin")
	active := teamReadActiveMembership("admin")
	expiresOne := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	expiresTwo := expiresOne.Add(24 * time.Hour)
	rows := &teamReadRows{values: [][]any{
		{
			teamReadInvitationOne,
			"admin@example.com",
			"admin",
			"pending",
			"inv_clerk_one",
			expiresOne,
		},
		{
			teamReadInvitationTwo,
			"member@example.com",
			"member",
			"pending",
			"",
			expiresTwo,
		},
	}}
	queryCalls := 0
	tx := &teamReadTx{
		query: func(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
			queryCalls++
			if got := ctx.Value(teamReadContextKey{}); got != claims.OrganizationID {
				t.Fatalf("transaction context organization = %v", got)
			}
			if !strings.Contains(query, "FROM iam.invitations") {
				t.Fatalf("query does not read invitations: %s", query)
			}
			if strings.Contains(strings.ToLower(query), "org_id") {
				t.Fatalf("query bypasses transaction RLS with an org_id predicate: %s", query)
			}
			if len(args) != 1 || args[0] != "pending" {
				t.Fatalf("query args = %v, want only status filter", args)
			}
			return rows, nil
		},
	}
	handler := newTeamReadTestHandler(
		t,
		claims,
		teamReadTransactor(t, claims, active, tx),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		newTeamReadRequest(
			http.MethodGet,
			"/api/v1/team/invitations?status=pending",
		),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	body := decodeIAMIdentityResponse[api.InvitationList](t, response)
	if body.Page.Total != 2 || body.Page.NextCursor != nil || len(body.Items) != 2 {
		t.Fatalf("invitation list = %+v", body)
	}
	first := body.Items[0]
	if first.Id.String() != teamReadInvitationOne ||
		first.Email != "admin@example.com" ||
		first.Role != api.RoleAdmin ||
		first.Status != api.InvitationStatusPending ||
		first.SyncStatus != api.SyncStatusSynced ||
		!first.ExpiresAt.Equal(expiresOne) {
		t.Fatalf("first invitation = %+v", first)
	}
	second := body.Items[1]
	if second.Id.String() != teamReadInvitationTwo ||
		second.Email != "member@example.com" ||
		second.Role != api.RoleMember ||
		second.Status != api.InvitationStatusPending ||
		second.SyncStatus != api.SyncStatusQueued ||
		!second.ExpiresAt.Equal(expiresTwo) {
		t.Fatalf("second invitation = %+v", second)
	}
	if queryCalls != 1 || !rows.closed {
		t.Fatalf("query calls = %d, rows closed = %t", queryCalls, rows.closed)
	}
}

func TestTeamReadEndpointsPaginate(t *testing.T) {
	nextCursor := base64.RawURLEncoding.EncodeToString([]byte("2"))
	cursorOne := base64.RawURLEncoding.EncodeToString([]byte("1"))

	t.Run("members", func(t *testing.T) {
		claims := teamReadClaims("org:member")
		active := teamReadActiveMembership("member")
		rows := &teamReadRows{values: append(
			teamReadMemberValues(),
			[]any{
				teamReadMembershipThree,
				"admin",
				"active",
				"membership_clerk_three",
				teamReadUserThree,
				"admin@example.com",
				"Admin Example",
				"",
			},
		)}
		tx := &teamReadTx{
			query: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rows, nil
			},
		}
		handler := newTeamReadTestHandler(
			t,
			claims,
			teamReadTransactor(t, claims, active, tx),
		)
		response := httptest.NewRecorder()
		handler.ServeHTTP(
			response,
			newTeamReadRequest(
				http.MethodGet,
				"/api/v1/team/members?limit=1&cursor="+cursorOne,
			),
		)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body)
		}
		body := decodeIAMIdentityResponse[api.MemberList](t, response)
		if len(body.Items) != 1 ||
			body.Items[0].Id.String() != teamReadMembershipTwo ||
			body.Page.Total != 3 ||
			body.Page.NextCursor == nil ||
			*body.Page.NextCursor != nextCursor {
			t.Fatalf("page = %+v", body)
		}
	})

	t.Run("invitations", func(t *testing.T) {
		claims := teamReadClaims("org:admin")
		active := teamReadActiveMembership("admin")
		expiresAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
		rows := &teamReadRows{values: [][]any{
			{teamReadInvitationOne, "one@example.com", "member", "pending", "", expiresAt},
			{teamReadInvitationTwo, "two@example.com", "member", "revoked", "", expiresAt},
			{teamReadInvitationThree, "three@example.com", "admin", "expired", "", expiresAt},
		}}
		tx := &teamReadTx{
			query: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rows, nil
			},
		}
		handler := newTeamReadTestHandler(
			t,
			claims,
			teamReadTransactor(t, claims, active, tx),
		)
		response := httptest.NewRecorder()
		handler.ServeHTTP(
			response,
			newTeamReadRequest(
				http.MethodGet,
				"/api/v1/team/invitations?limit=1&cursor="+cursorOne,
			),
		)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body)
		}
		body := decodeIAMIdentityResponse[api.InvitationList](t, response)
		if len(body.Items) != 1 ||
			body.Items[0].Id.String() != teamReadInvitationTwo ||
			body.Page.Total != 3 ||
			body.Page.NextCursor == nil ||
			*body.Page.NextCursor != nextCursor {
			t.Fatalf("page = %+v", body)
		}
	})
}

func TestTeamReadEndpointsRejectInvalidPagination(t *testing.T) {
	tests := []struct {
		name         string
		target       string
		activeRole   string
		providerRole string
		rows         [][]any
	}{
		{
			name:         "member cursor",
			target:       "/api/v1/team/members?cursor=not-base64!",
			activeRole:   "member",
			providerRole: "org:member",
			rows:         teamReadMemberValues(),
		},
		{
			name:         "member limit",
			target:       "/api/v1/team/members?limit=0",
			activeRole:   "member",
			providerRole: "org:member",
			rows:         teamReadMemberValues(),
		},
		{
			name:         "invitation cursor",
			target:       "/api/v1/team/invitations?cursor=not-base64!",
			activeRole:   "admin",
			providerRole: "org:admin",
			rows: [][]any{{
				teamReadInvitationOne,
				"invite@example.com",
				"member",
				"pending",
				"",
				time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC),
			}},
		},
		{
			name:         "invitation limit",
			target:       "/api/v1/team/invitations?limit=0",
			activeRole:   "admin",
			providerRole: "org:admin",
			rows: [][]any{{
				teamReadInvitationOne,
				"invite@example.com",
				"member",
				"pending",
				"",
				time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC),
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := teamReadClaims(test.providerRole)
			active := teamReadActiveMembership(test.activeRole)
			tx := &teamReadTx{
				query: func(context.Context, string, ...any) (pgx.Rows, error) {
					return &teamReadRows{values: test.rows}, nil
				},
			}
			handler := newTeamReadTestHandler(
				t,
				claims,
				teamReadTransactor(t, claims, active, tx),
			)
			response := httptest.NewRecorder()

			handler.ServeHTTP(
				response,
				newTeamReadRequest(http.MethodGet, test.target),
			)

			assertAPIError(t, response, http.StatusBadRequest, "REQUEST_INVALID")
		})
	}
}

func TestTeamReadPermissionIntersectsLocalAndClerkRoles(t *testing.T) {
	tests := []struct {
		name         string
		activeRole   string
		providerRole string
		permission   productiam.Permission
		wantAllowed  bool
	}{
		{
			name:         "provider admin preserves local owner authority",
			activeRole:   "owner",
			providerRole: "org:admin",
			permission:   productiam.PermissionOrganizationUpdate,
			wantAllowed:  true,
		},
		{
			name:         "provider member caps local owner authority",
			activeRole:   "owner",
			providerRole: "org:member",
			permission:   productiam.PermissionOrganizationUpdate,
			wantAllowed:  false,
		},
		{
			name:         "provider admin preserves local admin invitation authority",
			activeRole:   "admin",
			providerRole: "org:admin",
			permission:   productiam.PermissionInvitationManage,
			wantAllowed:  true,
		},
		{
			name:         "provider admin preserves local owner invitation authority",
			activeRole:   "owner",
			providerRole: "org:admin",
			permission:   productiam.PermissionInvitationManage,
			wantAllowed:  true,
		},
		{
			name:         "member cannot manage invitations",
			activeRole:   "member",
			providerRole: "org:member",
			permission:   productiam.PermissionInvitationManage,
			wantAllowed:  false,
		},
		{
			name:         "unknown provider role fails closed",
			activeRole:   "owner",
			providerRole: "org:auditor",
			permission:   productiam.PermissionTeamView,
			wantAllowed:  false,
		},
		{
			name:         "unknown local role fails closed",
			activeRole:   "superadmin",
			providerRole: "org:admin",
			permission:   productiam.PermissionTeamView,
			wantAllowed:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := requirePermission(
				teamReadActiveMembership(test.activeRole),
				teamReadClaims(test.providerRole),
				test.permission,
			)
			if test.wantAllowed && err != nil {
				t.Fatalf("permission denied: %v", err)
			}
			if !test.wantAllowed && !errors.Is(err, errIAMForbidden) {
				t.Fatalf("error = %v, want %v", err, errIAMForbidden)
			}
		})
	}
}

func TestListTeamInvitationsRequiresInvitationManagementPermission(t *testing.T) {
	tests := []struct {
		name         string
		activeRole   string
		providerRole string
		wantStatus   int
		wantQuery    bool
	}{
		{
			name:         "member is forbidden",
			activeRole:   "member",
			providerRole: "org:member",
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "admin is allowed",
			activeRole:   "admin",
			providerRole: "org:admin",
			wantStatus:   http.StatusOK,
			wantQuery:    true,
		},
		{
			name:         "owner is allowed",
			activeRole:   "owner",
			providerRole: "org:admin",
			wantStatus:   http.StatusOK,
			wantQuery:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := teamReadClaims(test.providerRole)
			active := teamReadActiveMembership(test.activeRole)
			queryCalled := false
			tx := &teamReadTx{
				query: func(context.Context, string, ...any) (pgx.Rows, error) {
					queryCalled = true
					return &teamReadRows{}, nil
				},
			}
			handler := newTeamReadTestHandler(
				t,
				claims,
				teamReadTransactor(t, claims, active, tx),
			)
			response := httptest.NewRecorder()

			handler.ServeHTTP(
				response,
				newTeamReadRequest(http.MethodGet, "/api/v1/team/invitations"),
			)

			if test.wantStatus == http.StatusForbidden {
				assertAPIError(t, response, http.StatusForbidden, "IAM_FORBIDDEN")
			} else if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body)
			}
			if queryCalled != test.wantQuery {
				t.Fatalf("query called = %t, want %t", queryCalled, test.wantQuery)
			}
		})
	}
}

func TestTeamReadHandlerRejectsUnknownClerkRoleBeforeQuery(t *testing.T) {
	claims := teamReadClaims("org:auditor")
	active := teamReadActiveMembership("owner")
	queryCalled := false
	tx := &teamReadTx{
		query: func(context.Context, string, ...any) (pgx.Rows, error) {
			queryCalled = true
			return &teamReadRows{}, nil
		},
	}
	handler := newTeamReadTestHandler(
		t,
		claims,
		teamReadTransactor(t, claims, active, tx),
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		newTeamReadRequest(http.MethodGet, "/api/v1/team/members"),
	)

	assertAPIError(t, response, http.StatusForbidden, "IAM_FORBIDDEN")
	if queryCalled {
		t.Fatal("team query ran after fail-closed role intersection")
	}
}

func TestTeamReadEndpointsDependOnSessionTransactorForRLS(t *testing.T) {
	for _, target := range []string{
		"/api/v1/team/members",
		"/api/v1/team/invitations",
	} {
		t.Run(target, func(t *testing.T) {
			claims := teamReadClaims("org:member")
			transactionCalls := 0
			handler := newTeamReadTestHandler(
				t,
				claims,
				sessionTransactorFunc(func(
					_ context.Context,
					session platformiam.VerifiedSession,
					_ platformiam.SessionTxFunc,
				) error {
					transactionCalls++
					assertVerifiedSession(t, session, claims)
					// Not invoking the callback proves the handler has no DB path
					// outside the transactor that installs tenant RLS context.
					return platformiam.ErrActiveMembershipRequired
				}),
			)
			response := httptest.NewRecorder()

			handler.ServeHTTP(
				response,
				newTeamReadRequest(http.MethodGet, target),
			)

			assertAPIError(
				t,
				response,
				http.StatusForbidden,
				"IAM_MEMBERSHIP_REQUIRED",
			)
			if transactionCalls != 1 {
				t.Fatalf("transaction calls = %d, want 1", transactionCalls)
			}
		})
	}
}

func TestListTeamInvitationsRejectsInvalidStatusBeforeSessionTransaction(t *testing.T) {
	verifierCalled := false
	transactorCalled := false
	handler := NewHandlerWithIAM(
		discardLogger(),
		nil,
		time.Second,
		NewIAMAPI(teamReadClerkConfig(), IAMDependencies{
			Verifier: clerkSessionVerifierFunc(func(
				context.Context,
				string,
			) (clerkadapter.SessionClaims, error) {
				verifierCalled = true
				return teamReadClaims("org:member"), nil
			}),
			Transactor: sessionTransactorFunc(func(
				context.Context,
				platformiam.VerifiedSession,
				platformiam.SessionTxFunc,
			) error {
				transactorCalled = true
				return nil
			}),
		}),
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		newTeamReadRequest(
			http.MethodGet,
			"/api/v1/team/invitations?status=unknown",
		),
	)

	assertAPIError(t, response, http.StatusBadRequest, "REQUEST_INVALID")
	if verifierCalled || transactorCalled {
		t.Fatalf(
			"verifier called = %t, transactor called = %t",
			verifierCalled,
			transactorCalled,
		)
	}
}

func TestTeamReadEndpointsMapQueryErrors(t *testing.T) {
	queryErr := errors.New("database query failed")
	tests := []struct {
		name   string
		target string
	}{
		{name: "members", target: "/api/v1/team/members"},
		{name: "invitations", target: "/api/v1/team/invitations"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			role := "member"
			providerRole := "org:member"
			if test.name == "invitations" {
				role = "admin"
				providerRole = "org:admin"
			}
			claims := teamReadClaims(providerRole)
			active := teamReadActiveMembership(role)
			tx := &teamReadTx{
				query: func(context.Context, string, ...any) (pgx.Rows, error) {
					return nil, queryErr
				},
			}
			handler := newTeamReadTestHandler(
				t,
				claims,
				teamReadTransactor(t, claims, active, tx),
			)
			response := httptest.NewRecorder()

			handler.ServeHTTP(
				response,
				newTeamReadRequest(http.MethodGet, test.target),
			)

			assertAPIError(
				t,
				response,
				http.StatusServiceUnavailable,
				"IAM_UNAVAILABLE",
			)
		})
	}
}

func TestTeamReadEndpointsRejectInvalidStoredEnums(t *testing.T) {
	expiresAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		target string
		row    []any
	}{
		{
			name:   "member role",
			target: "/api/v1/team/members",
			row: []any{
				teamReadMembershipOne,
				"superadmin",
				"active",
				"",
				teamReadUserOne,
				"member@example.com",
				"Member",
				"",
			},
		},
		{
			name:   "member status",
			target: "/api/v1/team/members",
			row: []any{
				teamReadMembershipOne,
				"member",
				"unknown",
				"",
				teamReadUserOne,
				"member@example.com",
				"Member",
				"",
			},
		},
		{
			name:   "invitation role",
			target: "/api/v1/team/invitations",
			row: []any{
				teamReadInvitationOne,
				"invite@example.com",
				"superadmin",
				"pending",
				"",
				expiresAt,
			},
		},
		{
			name:   "invitation status",
			target: "/api/v1/team/invitations",
			row: []any{
				teamReadInvitationOne,
				"invite@example.com",
				"member",
				"unknown",
				"",
				expiresAt,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			role := "member"
			providerRole := "org:member"
			if strings.HasPrefix(test.name, "invitation") {
				role = "admin"
				providerRole = "org:admin"
			}
			claims := teamReadClaims(providerRole)
			active := teamReadActiveMembership(role)
			tx := &teamReadTx{
				query: func(context.Context, string, ...any) (pgx.Rows, error) {
					return &teamReadRows{values: [][]any{test.row}}, nil
				},
			}
			handler := newTeamReadTestHandler(
				t,
				claims,
				teamReadTransactor(t, claims, active, tx),
			)
			response := httptest.NewRecorder()

			handler.ServeHTTP(
				response,
				newTeamReadRequest(http.MethodGet, test.target),
			)

			assertAPIError(
				t,
				response,
				http.StatusServiceUnavailable,
				"IAM_UNAVAILABLE",
			)
		})
	}
}

func newTeamReadTestHandler(
	t *testing.T,
	claims clerkadapter.SessionClaims,
	transactor SessionTransactor,
) http.Handler {
	t.Helper()
	verifier := clerkSessionVerifierFunc(func(
		_ context.Context,
		token string,
	) (clerkadapter.SessionClaims, error) {
		if token != "team-read-token" {
			t.Fatalf("token = %q", token)
		}
		return claims, nil
	})
	return NewHandlerWithIAM(
		discardLogger(),
		nil,
		time.Second,
		NewIAMAPI(teamReadClerkConfig(), IAMDependencies{
			Verifier:   verifier,
			Transactor: transactor,
		}),
	)
}

func teamReadTransactor(
	t *testing.T,
	claims clerkadapter.SessionClaims,
	active platformiam.ActiveMembership,
	tx pgx.Tx,
) SessionTransactor {
	t.Helper()
	return sessionTransactorFunc(func(
		ctx context.Context,
		session platformiam.VerifiedSession,
		callback platformiam.SessionTxFunc,
	) error {
		assertVerifiedSession(t, session, claims)
		ctx = context.WithValue(ctx, teamReadContextKey{}, claims.OrganizationID)
		return callback(ctx, tx, active)
	})
}

func teamReadClerkConfig() config.ClerkConfig {
	return config.ClerkConfig{
		PublishableKey: "pk_test_team_read",
		SecretKey:      "sk_test_team_read",
		Issuer:         "https://clerk.team-read.test",
	}
}

func teamReadClaims(providerRole string) clerkadapter.SessionClaims {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	return clerkadapter.SessionClaims{
		Subject:                 "user_clerk_team_read",
		SessionID:               "session_clerk_team_read",
		OrganizationID:          "organization_clerk_team_read",
		OrganizationRole:        providerRole,
		OrganizationPermissions: []string{"org:team:read"},
		IssuedAt:                now.Add(-time.Minute),
		ExpiresAt:               now.Add(time.Minute),
	}
}

func teamReadActiveMembership(role string) platformiam.ActiveMembership {
	return platformiam.ActiveMembership{
		MembershipID:   "77777777-7777-4777-8777-777777777777",
		OrganizationID: "88888888-8888-4888-8888-888888888888",
		UserID:         "99999999-9999-4999-8999-999999999999",
		Role:           role,
	}
}

func teamReadMemberValues() [][]any {
	return [][]any{
		{
			teamReadMembershipOne,
			"owner",
			"active",
			"membership_clerk_one",
			teamReadUserOne,
			"owner@example.com",
			"Owner Example",
			"https://cdn.example/owner.png",
		},
		{
			teamReadMembershipTwo,
			"member",
			"active",
			"",
			teamReadUserTwo,
			"member@example.com",
			"Member Example",
			"",
		},
	}
}

func newTeamReadRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer team-read-token")
	return request
}

type teamReadTx struct {
	pgx.Tx
	query func(context.Context, string, ...any) (pgx.Rows, error)
}

func (tx *teamReadTx) Query(
	ctx context.Context,
	query string,
	args ...any,
) (pgx.Rows, error) {
	if tx.query == nil {
		panic("unexpected Query call")
	}
	return tx.query(ctx, query, args...)
}

type teamReadRows struct {
	values  [][]any
	index   int
	current int
	err     error
	closed  bool
}

func (rows *teamReadRows) Close() {
	rows.closed = true
}

func (rows *teamReadRows) Err() error {
	return rows.err
}

func (rows *teamReadRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (rows *teamReadRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (rows *teamReadRows) Next() bool {
	if rows.index >= len(rows.values) {
		rows.Close()
		return false
	}
	rows.current = rows.index
	rows.index++
	return true
}

func (rows *teamReadRows) Scan(destinations ...any) error {
	if rows.current < 0 || rows.current >= len(rows.values) {
		return errors.New("scan called without a current row")
	}
	values := rows.values[rows.current]
	if len(destinations) != len(values) {
		return errors.New("unexpected scan destination count")
	}
	for index, value := range values {
		switch destination := destinations[index].(type) {
		case *string:
			typed, ok := value.(string)
			if !ok {
				return errors.New("unexpected string scan value")
			}
			*destination = typed
		case *time.Time:
			typed, ok := value.(time.Time)
			if !ok {
				return errors.New("unexpected time scan value")
			}
			*destination = typed
		default:
			return errors.New("unexpected scan destination type")
		}
	}
	return nil
}

func (rows *teamReadRows) Values() ([]any, error) {
	if rows.current < 0 || rows.current >= len(rows.values) {
		return nil, errors.New("values called without a current row")
	}
	return append([]any(nil), rows.values[rows.current]...), nil
}

func (rows *teamReadRows) RawValues() [][]byte {
	return nil
}

func (rows *teamReadRows) Conn() *pgx.Conn {
	return nil
}
