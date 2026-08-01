package scheduling

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/scheduling/usecases/domain"
	"github.com/google/uuid"
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
	created       bool
	bookingInput  CreateBookingInput
	publicCatalog PublicCatalog
}

func (f *handlerUsecasesFake) CreateBranch(
	_ context.Context,
	value domain.Branch,
) (domain.Branch, error) {
	f.created = true
	return value, nil
}

func (f *handlerUsecasesFake) ResolvePublicOrganization(
	context.Context,
	string,
) (string, error) {
	return "secret-org-id", nil
}

func (f *handlerUsecasesFake) PublicCatalog(
	context.Context,
	string,
) (PublicCatalog, error) {
	return f.publicCatalog, nil
}

func (f *handlerUsecasesFake) CreateBooking(
	_ context.Context,
	_ domain.CommandMetadata,
	input CreateBookingInput,
) ([]domain.Booking, error) {
	f.created = true
	f.bookingInput = input
	return []domain.Booking{{
		OrganizationID:  input.OrganizationID,
		ID:              uuid.New(),
		BranchID:        input.BranchID,
		ServiceID:       input.ServiceID,
		Status:          domain.BookingPendingConfirmation,
		Participants:    input.Participants,
		StartAt:         input.StartAt,
		EndAt:           input.StartAt.Add(time.Hour),
		Version:         1,
		ServiceName:     "Consulta",
		Price:           "100",
		Currency:        "ARS",
		DurationMinutes: 60,
		Timezone:        "UTC",
	}}, nil
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

func TestPublicBookingCannotInjectInternalStatus(t *testing.T) {
	usecases := &handlerUsecasesFake{}
	handler := NewHTTPHandler(usecases, nil).Handler()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/public/scheduling/acme/bookings",
		strings.NewReader(`{
			"branch_id":"11111111-1111-1111-1111-111111111111",
			"service_id":"22222222-2222-2222-2222-222222222222",
			"customer":{"name":"Ada"},
			"start_at":"2026-08-03T12:00:00Z",
			"participants":1,
			"status":"confirmed"
		}`),
	)
	request.Header.Set("Idempotency-Key", "public-status-injection")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if usecases.created {
		t.Fatal("public status injection reached the booking use case")
	}
}

func TestPublicCatalogRequiresNoSessionAndDoesNotExposeTenantMetadata(t *testing.T) {
	usecases := &handlerUsecasesFake{publicCatalog: PublicCatalog{
		Branches: []domain.Branch{{
			OrganizationID: "secret-org-id",
			ID:             uuid.New(),
			Code:           "internal-code",
			Slug:           "centro",
			Name:           "Centro",
			Timezone:       "America/Argentina/Buenos_Aires",
			Address:        "Calle 1",
			Active:         true,
		}},
		Services: []domain.Service{{
			OrganizationID:  "secret-org-id",
			ID:              uuid.New(),
			Code:            "consulta",
			Name:            "Consulta",
			DurationMinutes: 30,
			SlotMinutes:     15,
			Price:           "100",
			Currency:        "ARS",
			Mode:            domain.FulfillmentInPerson,
			MaxParticipants: 1,
			Active:          true,
		}},
		Resources: []domain.Resource{{
			OrganizationID: "secret-org-id",
			ID:             uuid.New(),
			BranchID:       uuid.New(),
			Code:           "internal-professional-code",
			Name:           "Profesional",
			Kind:           domain.ResourceProfessional,
			Capacity:       1,
			Timezone:       "America/Argentina/Buenos_Aires",
			Active:         true,
		}},
	}}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/public/scheduling/acme/catalog",
		nil,
	)
	NewHTTPHandler(usecases, nil).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	body := response.Body.String()
	if strings.Contains(body, "secret-org-id") ||
		strings.Contains(body, "internal-professional-code") ||
		strings.Contains(body, "internal-code") {
		t.Fatalf("public catalog leaked internal tenant metadata: %s", body)
	}
}
