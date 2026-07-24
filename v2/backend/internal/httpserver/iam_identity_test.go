package httpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
	productiam "github.com/devpablocristo/pymes/v2/backend/internal/iam"
)

type iamIdentityVerifierFunc func(context.Context, string) (clerkadapter.SessionClaims, error)

func (fn iamIdentityVerifierFunc) VerifyIdentity(
	ctx context.Context,
	token string,
) (clerkadapter.SessionClaims, error) {
	return fn(ctx, token)
}

type iamOrganizationDirectoryFunc func(
	context.Context,
	string,
	string,
) ([]productiam.AccessibleOrganization, error)

func (fn iamOrganizationDirectoryFunc) ListActiveOrganizations(
	ctx context.Context,
	provider string,
	subject string,
) ([]productiam.AccessibleOrganization, error) {
	return fn(ctx, provider, subject)
}

type iamSessionManagerFake struct {
	list   func(context.Context, clerkadapter.SessionListInput) ([]clerkadapter.Session, error)
	get    func(context.Context, string) (clerkadapter.Session, error)
	revoke func(context.Context, string) error
}

func (fake *iamSessionManagerFake) ListSessions(
	ctx context.Context,
	input clerkadapter.SessionListInput,
) ([]clerkadapter.Session, error) {
	if fake.list == nil {
		panic("unexpected ListSessions call")
	}
	return fake.list(ctx, input)
}

func (fake *iamSessionManagerFake) GetSession(
	ctx context.Context,
	sessionID string,
) (clerkadapter.Session, error) {
	if fake.get == nil {
		panic("unexpected GetSession call")
	}
	return fake.get(ctx, sessionID)
}

func (fake *iamSessionManagerFake) RevokeSession(
	ctx context.Context,
	sessionID string,
) error {
	if fake.revoke == nil {
		panic("unexpected RevokeSession call")
	}
	return fake.revoke(ctx, sessionID)
}

func TestIAMIdentityEndpointsAcceptIdentityWithoutOrganizationAndFilterBySubject(t *testing.T) {
	const (
		subject   = "user_verified"
		sessionID = "sess_current"
	)
	identityCalls := 0
	verifier := iamIdentityVerifierFunc(func(_ context.Context, token string) (clerkadapter.SessionClaims, error) {
		identityCalls++
		if token != "identity-token" {
			t.Fatalf("token = %q", token)
		}
		return clerkadapter.SessionClaims{
			Subject:   subject,
			SessionID: sessionID,
			// Identity-scoped endpoints intentionally work before Clerk has an
			// active organization.
			OrganizationID: "",
		}, nil
	})
	directory := iamOrganizationDirectoryFunc(func(
		_ context.Context,
		provider string,
		resolvedSubject string,
	) ([]productiam.AccessibleOrganization, error) {
		if provider != "clerk" {
			t.Fatalf("provider = %q", provider)
		}
		if resolvedSubject != subject {
			t.Fatalf("subject = %q", resolvedSubject)
		}
		return []productiam.AccessibleOrganization{{
			OrganizationID:         "3c2f0372-0152-45de-ab05-613c39a55ffc",
			ExternalOrganizationID: "org_clerk",
			Name:                   "Acme",
			Slug:                   "acme",
			MembershipID:           "membership_local",
			Role:                   productiam.RoleOwner,
		}}, nil
	})
	manager := &iamSessionManagerFake{
		list: func(
			_ context.Context,
			input clerkadapter.SessionListInput,
		) ([]clerkadapter.Session, error) {
			if input.ProviderUserID != subject {
				t.Fatalf("provider user ID = %q", input.ProviderUserID)
			}
			if input.Limit != 100 || input.Offset != 0 {
				t.Fatalf("provider pagination = %+v", input.ListInput)
			}
			return []clerkadapter.Session{{
				ID:     sessionID,
				UserID: subject,
				Status: "active",
			}}, nil
		},
	}
	handler := newIAMIdentityTestHandler(verifier, directory, manager)

	organizationsResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		organizationsResponse,
		newIAMIdentityRequest(http.MethodGet, "/api/v1/organizations?subject=user_attacker"),
	)
	if organizationsResponse.Code != http.StatusOK {
		t.Fatalf("organizations status = %d, body = %s", organizationsResponse.Code, organizationsResponse.Body)
	}
	organizations := decodeIAMIdentityResponse[api.OrganizationList](t, organizationsResponse)
	if len(organizations.Items) != 1 {
		t.Fatalf("organizations = %+v", organizations.Items)
	}
	if organizations.Items[0].Role != api.RoleOwner ||
		organizations.Items[0].Status != api.OrganizationStatusActive ||
		organizations.Items[0].SyncStatus != api.SyncStatusSynced {
		t.Fatalf("organization = %+v", organizations.Items[0])
	}
	if organizations.Items[0].SwitchKey == nil || *organizations.Items[0].SwitchKey != "org_clerk" {
		t.Fatalf("switch key = %v", organizations.Items[0].SwitchKey)
	}

	sessionsResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		sessionsResponse,
		newIAMIdentityRequest(http.MethodGet, "/api/v1/sessions?user_id=user_attacker"),
	)
	if sessionsResponse.Code != http.StatusOK {
		t.Fatalf("sessions status = %d, body = %s", sessionsResponse.Code, sessionsResponse.Body)
	}
	sessions := decodeIAMIdentityResponse[api.SessionList](t, sessionsResponse)
	if len(sessions.Items) != 1 || !sessions.Items[0].Current {
		t.Fatalf("sessions = %+v", sessions.Items)
	}
	if identityCalls != 2 {
		t.Fatalf("identity verifier calls = %d", identityCalls)
	}
}

func TestListMyOrganizationsPaginatesOpaqueCursor(t *testing.T) {
	organizations := []productiam.AccessibleOrganization{
		{
			OrganizationID:         "1f3077cc-a680-41ce-823d-3f76c943ac0f",
			ExternalOrganizationID: "org_one",
			Name:                   "One",
			Slug:                   "one",
			Role:                   productiam.RoleOwner,
		},
		{
			OrganizationID:         "e5ef6789-fbd8-470d-9555-e00ad6726f1f",
			ExternalOrganizationID: "org_two",
			Name:                   "Two",
			Slug:                   "two",
			Role:                   productiam.RoleAdmin,
		},
		{
			OrganizationID:         "51b96d37-7271-43ea-8b51-bf85716f8556",
			ExternalOrganizationID: "org_three",
			Name:                   "Three",
			Slug:                   "three",
			Role:                   productiam.RoleMember,
		},
	}
	handler := newIAMIdentityTestHandler(
		fixedIAMIdentityVerifier("user_verified", "sess_current"),
		iamOrganizationDirectoryFunc(func(
			context.Context,
			string,
			string,
		) ([]productiam.AccessibleOrganization, error) {
			return organizations, nil
		}),
		nil,
	)

	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		firstResponse,
		newIAMIdentityRequest(http.MethodGet, "/api/v1/organizations?limit=2"),
	)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", firstResponse.Code, firstResponse.Body)
	}
	first := decodeIAMIdentityResponse[api.OrganizationList](t, firstResponse)
	if len(first.Items) != 2 || first.Page.Total != 3 {
		t.Fatalf("first page = %+v", first)
	}
	expectedCursor := base64.RawURLEncoding.EncodeToString([]byte("2"))
	if first.Page.NextCursor == nil || *first.Page.NextCursor != expectedCursor {
		t.Fatalf("next cursor = %v, want %q", first.Page.NextCursor, expectedCursor)
	}

	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		secondResponse,
		newIAMIdentityRequest(
			http.MethodGet,
			"/api/v1/organizations?limit=2&cursor="+expectedCursor,
		),
	)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("second status = %d, body = %s", secondResponse.Code, secondResponse.Body)
	}
	second := decodeIAMIdentityResponse[api.OrganizationList](t, secondResponse)
	if len(second.Items) != 1 || second.Items[0].Slug != "three" {
		t.Fatalf("second page = %+v", second)
	}
	if second.Page.Total != 3 || second.Page.NextCursor != nil {
		t.Fatalf("second page info = %+v", second.Page)
	}
}

func TestIAMIdentityListEndpointsRejectInvalidPagination(t *testing.T) {
	handler := newIAMIdentityTestHandler(
		fixedIAMIdentityVerifier("user_verified", "sess_current"),
		iamOrganizationDirectoryFunc(func(
			context.Context,
			string,
			string,
		) ([]productiam.AccessibleOrganization, error) {
			return []productiam.AccessibleOrganization{{
				OrganizationID:         "8673181d-df94-4c60-a0a5-17fde926a56b",
				ExternalOrganizationID: "org_one",
				Name:                   "One",
				Slug:                   "one",
				Role:                   productiam.RoleOwner,
			}}, nil
		}),
		&iamSessionManagerFake{
			list: func(
				context.Context,
				clerkadapter.SessionListInput,
			) ([]clerkadapter.Session, error) {
				return []clerkadapter.Session{{ID: "sess_current", Status: "active"}}, nil
			},
		},
	)
	tests := []struct {
		name   string
		target string
	}{
		{
			name:   "organization cursor is not an integer",
			target: "/api/v1/organizations?cursor=bm90LWFuLWludGVnZXI",
		},
		{
			name:   "organization cursor is outside result set",
			target: "/api/v1/organizations?cursor=OTk",
		},
		{
			name:   "session cursor is not base64url",
			target: "/api/v1/sessions?cursor=%2A%2A%2A",
		},
		{
			name:   "session limit is zero",
			target: "/api/v1/sessions?limit=0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, newIAMIdentityRequest(http.MethodGet, test.target))
			assertIAMIdentityAPIError(t, response, http.StatusBadRequest, "REQUEST_INVALID")
		})
	}
}

func TestListMySessionsNormalizesStatusMarksCurrentAndPaginates(t *testing.T) {
	now := time.Date(2026, time.July, 23, 10, 30, 0, 0, time.UTC)
	manager := &iamSessionManagerFake{
		list: func(
			_ context.Context,
			input clerkadapter.SessionListInput,
		) ([]clerkadapter.Session, error) {
			if input.ProviderUserID != "user_verified" {
				t.Fatalf("provider user ID = %q", input.ProviderUserID)
			}
			return []clerkadapter.Session{
				{
					ID:           "sess_current",
					UserID:       "user_verified",
					Status:       " ACTIVE ",
					CreatedAt:    now.Add(-3 * time.Hour),
					LastActiveAt: now,
					ExpiresAt:    now.Add(time.Hour),
				},
				{
					ID:           "sess_foreign",
					UserID:       "different_user",
					Status:       "active",
					CreatedAt:    now.Add(-4 * time.Hour),
					LastActiveAt: now.Add(-time.Hour),
					ExpiresAt:    now.Add(time.Hour),
				},
				{
					ID:           "sess_ended",
					UserID:       "user_verified",
					Status:       "EnDeD",
					CreatedAt:    now.Add(-6 * time.Hour),
					LastActiveAt: now.Add(-2 * time.Hour),
					ExpiresAt:    now.Add(-time.Hour),
				},
				{
					ID:           "sess_revoked",
					UserID:       "user_verified",
					Status:       "REVOKED",
					CreatedAt:    now.Add(-9 * time.Hour),
					LastActiveAt: now.Add(-4 * time.Hour),
					ExpiresAt:    now.Add(-3 * time.Hour),
				},
			}, nil
		},
	}
	handler := newIAMIdentityTestHandler(
		fixedIAMIdentityVerifier("user_verified", "sess_current"),
		nil,
		manager,
	)

	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		firstResponse,
		newIAMIdentityRequest(http.MethodGet, "/api/v1/sessions?limit=2"),
	)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", firstResponse.Code, firstResponse.Body)
	}
	first := decodeIAMIdentityResponse[api.SessionList](t, firstResponse)
	if len(first.Items) != 2 || first.Page.Total != 3 {
		t.Fatalf("first page = %+v", first)
	}
	if first.Items[0].Status != api.SessionStatusActive || !first.Items[0].Current {
		t.Fatalf("current session = %+v", first.Items[0])
	}
	if first.Items[1].Status != api.SessionStatusEnded || first.Items[1].Current {
		t.Fatalf("ended session = %+v", first.Items[1])
	}
	if first.Page.NextCursor == nil {
		t.Fatal("first page has no next cursor")
	}

	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		secondResponse,
		newIAMIdentityRequest(
			http.MethodGet,
			"/api/v1/sessions?limit=2&cursor="+*first.Page.NextCursor,
		),
	)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("second status = %d, body = %s", secondResponse.Code, secondResponse.Body)
	}
	second := decodeIAMIdentityResponse[api.SessionList](t, secondResponse)
	if len(second.Items) != 1 ||
		second.Items[0].Status != api.SessionStatusRevoked ||
		second.Items[0].Current {
		t.Fatalf("second page = %+v", second)
	}
	if second.Page.Total != 3 || second.Page.NextCursor != nil {
		t.Fatalf("second page info = %+v", second.Page)
	}
}

func TestListMySessionsRejectsUnsupportedProviderStatus(t *testing.T) {
	handler := newIAMIdentityTestHandler(
		fixedIAMIdentityVerifier("user_verified", "sess_current"),
		nil,
		&iamSessionManagerFake{
			list: func(
				context.Context,
				clerkadapter.SessionListInput,
			) ([]clerkadapter.Session, error) {
				return []clerkadapter.Session{{
					ID:     "sess_current",
					UserID: "user_verified",
					Status: "provider-added-status",
				}}, nil
			},
		},
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		newIAMIdentityRequest(http.MethodGet, "/api/v1/sessions"),
	)

	assertIAMIdentityAPIError(
		t,
		response,
		http.StatusServiceUnavailable,
		"AUTH_PROVIDER_UNAVAILABLE",
	)
}

func TestRevokeMySessionRequiresIdempotencyKeyBeforeIdentityLookup(t *testing.T) {
	identityCalls := 0
	handler := newIAMIdentityTestHandler(
		iamIdentityVerifierFunc(func(context.Context, string) (clerkadapter.SessionClaims, error) {
			identityCalls++
			return clerkadapter.SessionClaims{Subject: "user_verified"}, nil
		}),
		nil,
		&iamSessionManagerFake{},
	)
	tests := []struct {
		name        string
		headerValue *string
	}{
		{name: "missing"},
		{name: "blank", headerValue: stringPointer("   ")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := newIAMIdentityRequest(
				http.MethodDelete,
				"/api/v1/sessions/sess_target",
			)
			if test.headerValue != nil {
				request.Header.Set("Idempotency-Key", *test.headerValue)
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			assertIAMIdentityAPIError(t, response, http.StatusBadRequest, "REQUEST_INVALID")
		})
	}
	if identityCalls != 0 {
		t.Fatalf("identity verifier calls = %d", identityCalls)
	}
}

func TestRevokeMySessionChecksOwnershipBeforeRevoking(t *testing.T) {
	revokeCalled := false
	manager := &iamSessionManagerFake{
		get: func(_ context.Context, sessionID string) (clerkadapter.Session, error) {
			if sessionID != "sess_target" {
				t.Fatalf("session ID = %q", sessionID)
			}
			return clerkadapter.Session{
				ID:     sessionID,
				UserID: "user_other",
				Status: "active",
			}, nil
		},
		revoke: func(context.Context, string) error {
			revokeCalled = true
			return nil
		},
	}
	handler := newIAMIdentityTestHandler(
		fixedIAMIdentityVerifier("user_verified", "sess_current"),
		nil,
		manager,
	)
	request := newIAMIdentityRequest(http.MethodDelete, "/api/v1/sessions/sess_target")
	request.Header.Set("Idempotency-Key", "revoke-sess-target")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	assertIAMIdentityAPIError(t, response, http.StatusForbidden, "IAM_FORBIDDEN")
	if revokeCalled {
		t.Fatal("RevokeSession called for a session owned by another user")
	}
}

func TestRevokeMySessionTreatsMissingProviderSessionAsIdempotentSuccess(t *testing.T) {
	revokeCalled := false
	manager := &iamSessionManagerFake{
		get: func(context.Context, string) (clerkadapter.Session, error) {
			return clerkadapter.Session{}, &clerkadapter.APIError{
				StatusCode: http.StatusNotFound,
			}
		},
		revoke: func(context.Context, string) error {
			revokeCalled = true
			return nil
		},
	}
	handler := newIAMIdentityTestHandler(
		fixedIAMIdentityVerifier("user_verified", "sess_current"),
		nil,
		manager,
	)
	request := newIAMIdentityRequest(http.MethodDelete, "/api/v1/sessions/sess_missing")
	request.Header.Set("Idempotency-Key", "revoke-sess-missing")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	if revokeCalled {
		t.Fatal("RevokeSession called after an idempotent 404")
	}
}

func TestRevokeMySessionRevokesOwnedSession(t *testing.T) {
	revokeCalls := 0
	manager := &iamSessionManagerFake{
		get: func(_ context.Context, sessionID string) (clerkadapter.Session, error) {
			return clerkadapter.Session{
				ID:     sessionID,
				UserID: "user_verified",
				Status: "active",
			}, nil
		},
		revoke: func(_ context.Context, sessionID string) error {
			revokeCalls++
			if sessionID != "sess_target" {
				t.Fatalf("session ID = %q", sessionID)
			}
			return nil
		},
	}
	handler := newIAMIdentityTestHandler(
		fixedIAMIdentityVerifier("user_verified", "sess_current"),
		nil,
		manager,
	)
	request := newIAMIdentityRequest(http.MethodDelete, "/api/v1/sessions/sess_target")
	request.Header.Set("Idempotency-Key", "revoke-sess-target")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	if revokeCalls != 1 {
		t.Fatalf("revoke calls = %d", revokeCalls)
	}
}

func TestIAMSessionEndpointsPropagateClerkRateLimitRetryAfter(t *testing.T) {
	newRateLimitError := func() error {
		return &clerkadapter.APIError{
			StatusCode: http.StatusTooManyRequests,
			Headers: http.Header{
				"Retry-After": []string{"17"},
			},
		}
	}
	tests := []struct {
		name    string
		target  string
		manager *iamSessionManagerFake
	}{
		{
			name:   "list",
			target: "/api/v1/sessions",
			manager: &iamSessionManagerFake{
				list: func(
					context.Context,
					clerkadapter.SessionListInput,
				) ([]clerkadapter.Session, error) {
					return nil, newRateLimitError()
				},
			},
		},
		{
			name:   "lookup before revoke",
			target: "/api/v1/sessions/sess_target",
			manager: &iamSessionManagerFake{
				get: func(context.Context, string) (clerkadapter.Session, error) {
					return clerkadapter.Session{}, newRateLimitError()
				},
			},
		},
		{
			name:   "revoke",
			target: "/api/v1/sessions/sess_target",
			manager: &iamSessionManagerFake{
				get: func(_ context.Context, sessionID string) (clerkadapter.Session, error) {
					return clerkadapter.Session{
						ID:     sessionID,
						UserID: "user_verified",
					}, nil
				},
				revoke: func(context.Context, string) error {
					return newRateLimitError()
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := newIAMIdentityTestHandler(
				fixedIAMIdentityVerifier("user_verified", "sess_current"),
				nil,
				test.manager,
			)
			method := http.MethodGet
			if test.name != "list" {
				method = http.MethodDelete
			}
			request := newIAMIdentityRequest(method, test.target)
			if method == http.MethodDelete {
				request.Header.Set("Idempotency-Key", "revoke-sess-target")
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			assertIAMIdentityAPIError(
				t,
				response,
				http.StatusTooManyRequests,
				"AUTH_PROVIDER_RATE_LIMITED",
			)
			if response.Header().Get("Retry-After") != "17" {
				t.Fatalf("Retry-After = %q", response.Header().Get("Retry-After"))
			}
		})
	}
}

func newIAMIdentityTestHandler(
	identityVerifier ClerkIdentityVerifier,
	organizationDirectory productiam.OrganizationDirectory,
	sessionManager ClerkSessionManager,
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
			IdentityVerifier:      identityVerifier,
			OrganizationDirectory: organizationDirectory,
			SessionManager:        sessionManager,
		}),
	)
}

func fixedIAMIdentityVerifier(subject, sessionID string) ClerkIdentityVerifier {
	return iamIdentityVerifierFunc(func(
		context.Context,
		string,
	) (clerkadapter.SessionClaims, error) {
		return clerkadapter.SessionClaims{
			Subject:   subject,
			SessionID: sessionID,
		}, nil
	})
}

func newIAMIdentityRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer identity-token")
	return request
}

func decodeIAMIdentityResponse[T any](
	t *testing.T,
	response *httptest.ResponseRecorder,
) T {
	t.Helper()
	var body T
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, response.Body)
	}
	return body
}

func assertIAMIdentityAPIError(
	t *testing.T,
	response *httptest.ResponseRecorder,
	status int,
	code string,
) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, status, response.Body)
	}
	body := decodeIAMIdentityResponse[api.ErrorResponse](t, response)
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q; body = %+v", body.Error.Code, code, body)
	}
}

func stringPointer(value string) *string {
	return &value
}
