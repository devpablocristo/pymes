package wire

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	kerneldomain "github.com/devpablocristo/core/saas/go/kernel/usecases/domain"
	saasmiddleware "github.com/devpablocristo/core/saas/go/middleware"
)

type sessionStubJWTVerifier struct{}

func (sessionStubJWTVerifier) Verify(ctx context.Context, token string) (kerneldomain.Principal, error) {
	_ = token
	return kerneldomain.Principal{
		TenantID:   "org-uuid",
		Actor:      "user_xxx",
		Role:       "viewer",
		Scopes:     []string{},
		AuthMethod: "jwt",
	}, nil
}

type sessionStubAPIKeyVerifier struct{}

func (sessionStubAPIKeyVerifier) Verify(ctx context.Context, token string) (kerneldomain.Principal, error) {
	_ = token
	return kerneldomain.Principal{
		TenantID:   "org-uuid",
		Actor:      "api_key:key-1",
		Role:       "service",
		Scopes:     []string{},
		AuthMethod: "api_key",
	}, nil
}

func TestHandleSessionEnriched_ProductRoleUser(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	authMW := saasmiddleware.NewAuthMiddleware(sessionStubJWTVerifier{}, nil)
	registerProtected(mux, authMW, "GET /session", func(w http.ResponseWriter, r *http.Request) {
		handleSessionEnriched(w, r, nil)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer t")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	auth, ok := body["auth"].(map[string]any)
	if !ok {
		t.Fatalf("auth: %v", body["auth"])
	}
	if auth["product_role"] != "user" {
		t.Fatalf("product_role=%v", auth["product_role"])
	}
	if auth["org_id"] != "org-uuid" {
		t.Fatalf("org_id=%v", auth["org_id"])
	}
	if auth["tenant_id"] != "org-uuid" {
		t.Fatalf("tenant_id=%v", auth["tenant_id"])
	}
}

func TestHandleSessionEnriched_ServiceWithoutConsoleScopesIsUser(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	authMW := saasmiddleware.NewAuthMiddleware(nil, sessionStubAPIKeyVerifier{})
	registerProtected(mux, authMW, "GET /session", func(w http.ResponseWriter, r *http.Request) {
		handleSessionEnriched(w, r, nil)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-API-KEY", "psk_test")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	auth, ok := body["auth"].(map[string]any)
	if !ok {
		t.Fatalf("auth: %v", body["auth"])
	}
	if auth["product_role"] != "user" {
		t.Fatalf("product_role=%v", auth["product_role"])
	}
	if auth["auth_method"] != "api_key" {
		t.Fatalf("auth_method=%v", auth["auth_method"])
	}
}
