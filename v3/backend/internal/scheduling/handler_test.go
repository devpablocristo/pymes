package scheduling

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
)

type authenticatorFake struct {
	principal Principal
	err       error
}

func (f authenticatorFake) Principal(*http.Request) (Principal, error) {
	return f.principal, f.err
}

type handlerUsecasesFake struct {
	SchedulingUsecases
	created bool
}

func (f *handlerUsecasesFake) CreateBranch(
	_ context.Context,
	value domain.Branch,
) (domain.Branch, error) {
	f.created = true
	return value, nil
}

func TestSchedulingHandlerEnforcesTenantAndExplicitPermission(t *testing.T) {
	tests := []struct {
		name       string
		principal  Principal
		wantStatus int
		wantCreate bool
	}{
		{
			name: "cross tenant owner denied",
			principal: Principal{
				OrganizationID: "org_other", ActorID: "owner", Role: "owner",
				OrganizationStatus: "ready", MembershipStatus: "active",
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "member without manage denied",
			principal: Principal{
				OrganizationID: "org_a", ActorID: "member", Role: "member",
				Permissions:        []string{domain.PermissionRead},
				OrganizationStatus: "ready", MembershipStatus: "active",
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "explicit manage allowed",
			principal: Principal{
				OrganizationID: "org_a", ActorID: "member", Role: "member",
				Permissions:        []string{domain.PermissionManage},
				OrganizationStatus: "ready", MembershipStatus: "active",
			},
			wantStatus: http.StatusCreated,
			wantCreate: true,
		},
	}
	body := `{"code":"main","slug":"main","name":"Main","timezone":"UTC","address":""}`
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usecases := &handlerUsecasesFake{}
			handler := NewHTTPHandler(usecases, authenticatorFake{principal: test.principal}).Handler()
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/organizations/org_a/scheduling/branches",
				strings.NewReader(body),
			)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus || usecases.created != test.wantCreate {
				t.Fatalf("status=%d created=%v body=%s", response.Code, usecases.created, response.Body.String())
			}
		})
	}
}

func TestSchedulingHasNoPublicPhoneLookupAuthenticationRoute(t *testing.T) {
	handler := NewHTTPHandler(&handlerUsecasesFake{}, nil).Handler()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/public/scheduling/acme/bookings?phone=541155555555",
		nil,
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed && response.Code != http.StatusNotFound {
		t.Fatalf("phone lookup unexpectedly exposed: status=%d body=%s", response.Code, response.Body.String())
	}
}
