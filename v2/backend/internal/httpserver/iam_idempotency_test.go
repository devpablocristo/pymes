package httpserver

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	platformiam "github.com/devpablocristo/platform/iam/go"
	platformidempotency "github.com/devpablocristo/platform/idempotency/go"
	clerkadapter "github.com/devpablocristo/platform/sdks/clerk/go"
)

type iamIdempotencyStore struct {
	mu       sync.Mutex
	claim    platformidempotency.ClaimRequest
	lease    platformidempotency.Lease
	response *platformidempotency.Response
	err      error
	claims   int
}

func (store *iamIdempotencyStore) Claim(
	_ context.Context,
	claim platformidempotency.ClaimRequest,
) (platformidempotency.ClaimResult, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.claim = claim
	store.claims++
	if store.err != nil {
		return platformidempotency.ClaimResult{}, store.err
	}
	if store.response != nil {
		return platformidempotency.ClaimResult{
			Outcome:  platformidempotency.OutcomeReplay,
			Response: *store.response,
		}, nil
	}
	store.lease = platformidempotency.Lease{
		Scope:       claim.Scope,
		Key:         claim.Key,
		Fingerprint: claim.Fingerprint,
		Token:       "lease",
	}
	return platformidempotency.ClaimResult{
		Outcome: platformidempotency.OutcomeAcquired,
		Lease:   store.lease,
	}, nil
}

func (store *iamIdempotencyStore) Complete(
	_ context.Context,
	_ platformidempotency.Lease,
	response platformidempotency.Response,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	copy := response
	copy.Header = response.Header.Clone()
	copy.Body = bytes.Clone(response.Body)
	store.response = &copy
	return nil
}

func (*iamIdempotencyStore) Abandon(
	context.Context,
	platformidempotency.Lease,
) error {
	return nil
}

type iamIdempotencyVerifier struct {
	claims clerkadapter.SessionClaims
	err    error
	calls  int
}

func (verifier *iamIdempotencyVerifier) VerifySession(
	context.Context,
	string,
) (clerkadapter.SessionClaims, error) {
	verifier.calls++
	return verifier.claims, verifier.err
}

func (verifier *iamIdempotencyVerifier) VerifyIdentity(
	context.Context,
	string,
) (clerkadapter.SessionClaims, error) {
	verifier.calls++
	return verifier.claims, verifier.err
}

type iamIdempotencyTransactor struct {
	active platformiam.ActiveMembership
	err    error
	calls  int
}

func (transactor *iamIdempotencyTransactor) WithinSessionTx(
	ctx context.Context,
	_ platformiam.VerifiedSession,
	fn platformiam.SessionTxFunc,
) error {
	transactor.calls++
	if transactor.err != nil {
		return transactor.err
	}
	return fn(ctx, nil, transactor.active)
}

func TestIAMIdempotencyAuthorizesLocalMembershipBeforeReplay(t *testing.T) {
	store := &iamIdempotencyStore{}
	verifier := &iamIdempotencyVerifier{claims: iamIdempotencyClaims()}
	transactor := &iamIdempotencyTransactor{
		active: platformiam.ActiveMembership{
			OrganizationID: "local-org",
			UserID:         "local-user",
		},
	}
	middleware, err := NewIAMIdempotency(store, verifier, verifier, transactor)
	if err != nil {
		t.Fatalf("NewIAMIdempotency: %v", err)
	}
	var executions atomic.Int32
	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executions.Add(1)
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Fatalf("read replayed request body: %v", readErr)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write(body)
	}))

	first := performIdempotentIAMRequest(handler, http.MethodPatch, "/api/v1/organization", "same-key", `{"name":"Norte"}`)
	second := performIdempotentIAMRequest(handler, http.MethodPatch, "/api/v1/organization", "same-key", `{"name":"Norte"}`)

	if first.Code != http.StatusAccepted || second.Code != http.StatusAccepted {
		t.Fatalf("statuses = %d, %d", first.Code, second.Code)
	}
	if first.Body.String() != second.Body.String() || executions.Load() != 1 {
		t.Fatalf("replay bodies/executions = %q, %q, %d", first.Body, second.Body, executions.Load())
	}
	if store.claim.Scope != "pymes-v2:iam:org:local-org:user:local-user" {
		t.Fatalf("trusted scope = %q", store.claim.Scope)
	}
	if verifier.calls != 2 || transactor.calls != 2 {
		t.Fatalf("authorization calls verifier=%d transactor=%d", verifier.calls, transactor.calls)
	}
}

func TestIAMIdempotencyRejectsRemovedMembershipBeforeStore(t *testing.T) {
	store := &iamIdempotencyStore{}
	verifier := &iamIdempotencyVerifier{claims: iamIdempotencyClaims()}
	transactor := &iamIdempotencyTransactor{err: platformiam.ErrActiveMembershipRequired}
	middleware, err := NewIAMIdempotency(store, verifier, verifier, transactor)
	if err != nil {
		t.Fatalf("NewIAMIdempotency: %v", err)
	}
	var executed atomic.Bool
	handler := middleware.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		executed.Store(true)
	}))

	response := performIdempotentIAMRequest(
		handler,
		http.MethodPatch,
		"/api/v1/organization",
		"blocked-key",
		`{"name":"Norte"}`,
	)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	if executed.Load() || store.claims != 0 {
		t.Fatalf("command/store executed = %v/%d", executed.Load(), store.claims)
	}
}

func TestIAMIdempotencyUsesIdentityScopeForSessionRevocation(t *testing.T) {
	store := &iamIdempotencyStore{}
	verifier := &iamIdempotencyVerifier{claims: iamIdempotencyClaims()}
	transactor := &iamIdempotencyTransactor{}
	middleware, err := NewIAMIdempotency(store, verifier, verifier, transactor)
	if err != nil {
		t.Fatalf("NewIAMIdempotency: %v", err)
	}
	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	response := performIdempotentIAMRequest(
		handler,
		http.MethodDelete,
		"/api/v1/sessions/sess_other",
		"session-key",
		"",
	)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	if store.claim.Scope != "pymes-v2:iam:user:user_verified" {
		t.Fatalf("identity scope = %q", store.claim.Scope)
	}
	if transactor.calls != 0 {
		t.Fatalf("identity command opened tenant transaction %d times", transactor.calls)
	}
}

func TestIAMIdempotencyMapsFingerprintConflict(t *testing.T) {
	store := &iamIdempotencyStore{err: platformidempotency.ErrFingerprintMismatch}
	verifier := &iamIdempotencyVerifier{claims: iamIdempotencyClaims()}
	transactor := &iamIdempotencyTransactor{
		active: platformiam.ActiveMembership{
			OrganizationID: "local-org",
			UserID:         "local-user",
		},
	}
	middleware, err := NewIAMIdempotency(store, verifier, verifier, transactor)
	if err != nil {
		t.Fatalf("NewIAMIdempotency: %v", err)
	}
	handler := middleware.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("conflicting command reached handler")
	}))

	response := performIdempotentIAMRequest(
		handler,
		http.MethodPatch,
		"/api/v1/organization",
		"conflict-key",
		`{"name":"Sur"}`,
	)

	assertIAMIdentityAPIError(
		t,
		response,
		http.StatusConflict,
		"IDEMPOTENCY_KEY_CONFLICT",
	)
}

func TestIAMIdempotencyRejectsAmbiguousOrShortKeysBeforeAuthorization(t *testing.T) {
	store := &iamIdempotencyStore{}
	verifier := &iamIdempotencyVerifier{claims: iamIdempotencyClaims()}
	transactor := &iamIdempotencyTransactor{}
	middleware, err := NewIAMIdempotency(store, verifier, verifier, transactor)
	if err != nil {
		t.Fatalf("NewIAMIdempotency: %v", err)
	}
	handler := middleware.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid key reached handler")
	}))

	for _, test := range []struct {
		name string
		keys []string
	}{
		{name: "short", keys: []string{"short"}},
		{name: "duplicate", keys: []string{"valid-key-one", "valid-key-two"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPatch,
				"/api/v1/organization",
				bytes.NewBufferString(`{"name":"Norte"}`),
			)
			request.Header.Set("Authorization", "Bearer valid-token")
			for _, key := range test.keys {
				request.Header.Add("Idempotency-Key", key)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			assertIAMIdentityAPIError(
				t,
				response,
				http.StatusBadRequest,
				"REQUEST_INVALID",
			)
		})
	}
	if verifier.calls != 0 || transactor.calls != 0 || store.claims != 0 {
		t.Fatalf(
			"invalid keys reached authorization/store: verifier=%d tx=%d claims=%d",
			verifier.calls,
			transactor.calls,
			store.claims,
		)
	}
}

func iamIdempotencyClaims() clerkadapter.SessionClaims {
	now := time.Now().UTC()
	return clerkadapter.SessionClaims{
		Subject:          "user_verified",
		SessionID:        "sess_verified",
		OrganizationID:   "org_verified",
		OrganizationRole: "org:admin",
		IssuedAt:         now.Add(-time.Minute),
		ExpiresAt:        now.Add(time.Minute),
	}
}

func performIdempotentIAMRequest(
	handler http.Handler,
	method string,
	path string,
	key string,
	body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer valid-token")
	request.Header.Set("Idempotency-Key", key)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
