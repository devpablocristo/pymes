package wire

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	kerneldomain "github.com/devpablocristo/core/saas/go/kernel/usecases/domain"
	saasmiddleware "github.com/devpablocristo/core/saas/go/middleware"
)

type crossOrgJWTVerifier struct{}

func (crossOrgJWTVerifier) Verify(_ context.Context, _ string) (kerneldomain.Principal, error) {
	return kerneldomain.Principal{
		TenantID:   "org-a",
		Actor:      "user-1",
		Role:       "admin",
		Scopes:     []string{"admin:console:write"},
		AuthMethod: "jwt",
	}, nil
}

func TestHandleListMembers_DeniesCrossOrgAccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	authMW := saasmiddleware.NewAuthMiddleware(crossOrgJWTVerifier{}, nil)
	registerProtected(mux, authMW, "GET /orgs/{org_id}/members", func(w http.ResponseWriter, r *http.Request) {
		handleListMembers(w, r, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/orgs/org-b/members", nil)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleListAPIKeys_DeniesAPIKeyCaller(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	authMW := saasmiddleware.NewAuthMiddleware(nil, sessionStubAPIKeyVerifier{})
	registerProtected(mux, authMW, "GET /orgs/{org_id}/api-keys", func(w http.ResponseWriter, r *http.Request) {
		handleListAPIKeys(w, r, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/orgs/org-uuid/api-keys", nil)
	req.Header.Set("X-API-KEY", "psk_test")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
