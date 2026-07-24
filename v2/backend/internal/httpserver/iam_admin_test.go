package httpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devpablocristo/pymes/v2/backend/internal/administration"
	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/devpablocristo/pymes/v2/backend/internal/config"
)

const (
	adminTenantID = "33333333-3333-4333-8333-333333333333"
	adminUserID   = "55555555-5555-4555-8555-555555555555"
)

type administrationStub struct {
	AdministrationService
	owner           bool
	ownerErr        error
	tenants         []administration.Tenant
	users           []administration.User
	updateUser      administration.User
	updateError     error
	tenantFilter    administration.TenantFilter
	lifecycleAction string
	lifecycleReason string
}

func (stub *administrationStub) IsOwner(context.Context, string) (bool, error) {
	return stub.owner, stub.ownerErr
}

func (stub *administrationStub) ListTenants(
	_ context.Context,
	_ string,
	filter administration.TenantFilter,
) ([]administration.Tenant, error) {
	stub.tenantFilter = filter
	return stub.tenants, nil
}

func (stub *administrationStub) ListUsers(
	context.Context,
	string,
	administration.UserFilter,
) ([]administration.User, error) {
	return stub.users, nil
}

func (stub *administrationStub) UpdateUser(
	context.Context,
	string,
	string,
	administration.UpdateUserInput,
) (administration.User, error) {
	return stub.updateUser, stub.updateError
}

func (stub *administrationStub) TrashTenant(
	_ context.Context,
	_, _, reason string,
) error {
	stub.lifecycleAction = "trash"
	stub.lifecycleReason = reason
	return nil
}

func TestAdminTenantListRequiresGlobalOwner(t *testing.T) {
	handler := newAdministrationTestHandler(&administrationStub{owner: false})
	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		newIAMIdentityRequest(http.MethodGet, "/api/v1/admin/tenants"),
	)

	assertIAMIdentityAPIError(t, response, http.StatusForbidden, "IAM_FORBIDDEN")
}

func TestGlobalOwnerFiltersAndTrashesTenantWithPlatformLifecycleContract(t *testing.T) {
	stub := &administrationStub{owner: true}
	handler := newAdministrationTestHandler(stub)

	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		listResponse,
		newIAMIdentityRequest(
			http.MethodGet,
			"/api/v1/admin/tenants?lifecycle_state=archived",
		),
	)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body)
	}
	if stub.tenantFilter.LifecycleState != "archived" {
		t.Fatalf("lifecycle filter = %q", stub.tenantFilter.LifecycleState)
	}

	request := newIAMIdentityRequest(
		http.MethodPost,
		"/api/v1/admin/tenants/"+adminTenantID+"/trash",
	)
	request.Body = ioNopCloser(`{"reason":"tenant duplicado"}`)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "admin-tenant-trash")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("trash status = %d, body = %s", response.Code, response.Body)
	}
	if stub.lifecycleAction != "trash" || stub.lifecycleReason != "tenant duplicado" {
		t.Fatalf("lifecycle action = %q, reason = %q", stub.lifecycleAction, stub.lifecycleReason)
	}
}

func TestGlobalOwnerListsTenantsWithoutActiveClerkOrganization(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	handler := newAdministrationTestHandler(&administrationStub{
		owner: true,
		tenants: []administration.Tenant{{
			ID:             adminTenantID,
			Name:           "Pymes Base",
			Slug:           "pymes-base",
			Status:         "active",
			LifecycleState: "active",
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	})
	response := httptest.NewRecorder()

	// fixedIAMIdentityVerifier emits only sub and sid. Administration must not
	// depend on Clerk's currently selected organization.
	handler.ServeHTTP(
		response,
		newIAMIdentityRequest(http.MethodGet, "/api/v1/admin/tenants"),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body)
	}
	body := decodeIAMIdentityResponse[api.AdminTenantList](t, response)
	if len(body.Items) != 1 || body.Items[0].Name != "Pymes Base" {
		t.Fatalf("tenants = %+v", body.Items)
	}
	if body.Items[0].Id.String() != adminTenantID {
		t.Fatalf("tenant id = %s", body.Items[0].Id)
	}
}

func TestAdminUserUpdatePreservesLastGlobalOwner(t *testing.T) {
	handler := newAdministrationTestHandler(&administrationStub{
		owner:       true,
		updateError: administration.ErrLastOwner,
	})
	request := newIAMIdentityRequest(
		http.MethodPatch,
		"/api/v1/admin/users/"+adminUserID,
	)
	request.Body = ioNopCloser(`{"product_role":"user","version":7}`)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "admin-user-update-last-owner")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	assertIAMIdentityAPIError(t, response, http.StatusConflict, "IAM_LAST_OWNER")
}

func TestAdminUserCreateRejectsTenantOwnerRole(t *testing.T) {
	handler := newAdministrationTestHandler(&administrationStub{owner: true})
	request := newIAMIdentityRequest(http.MethodPost, "/api/v1/admin/users")
	request.Body = ioNopCloser(
		`{"email":"user@example.test","tenant_id":"` + adminTenantID + `","role":"owner"}`,
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "admin-user-create-owner")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	assertIAMIdentityAPIError(t, response, http.StatusBadRequest, "REQUEST_INVALID")
}

func newAdministrationTestHandler(administrationService AdministrationService) http.Handler {
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
			IdentityVerifier: fixedIAMIdentityVerifier("user_owner", "sess_current"),
			Administration:   administrationService,
		}),
	)
}

func ioNopCloser(body string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(strings.TrimSpace(body)))
}
