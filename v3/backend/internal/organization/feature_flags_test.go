package organization

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
)

type featureStoreFake struct {
	flags   domain.FeatureFlags
	command domain.UpdateFeatureFlags
	err     error
}

func (store *featureStoreFake) GetFeatureFlags(
	context.Context,
	string,
) (domain.FeatureFlags, error) {
	return store.flags, store.err
}

func (store *featureStoreFake) UpdateFeatureFlags(
	_ context.Context,
	command domain.UpdateFeatureFlags,
) (domain.FeatureFlags, error) {
	store.command = command
	return store.flags, store.err
}

type featureAuthFake struct {
	principal identitydomain.Principal
	err       error
}

func (auth featureAuthFake) Principal(
	*http.Request,
) (identitydomain.Principal, error) {
	return auth.principal, auth.err
}

func TestFeaturesFailClosedAndValidateUnknownCapabilities(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	store := &featureStoreFake{flags: domain.FeatureFlags{
		OrganizationID: "org-a", Version: 1,
		UpdatedAt: now, UpdatedBy: "system",
	}}
	features := Features{Store: store}
	enabled, err := features.Enabled(
		context.Background(),
		"org-a",
		string(domain.FeatureScheduling),
	)
	if err != nil || enabled {
		t.Fatalf("enabled=%v err=%v", enabled, err)
	}
	if _, err = features.Enabled(
		context.Background(),
		"org-a",
		"unknown_enabled",
	); !errors.Is(err, domain.ErrFeatureUnknown) {
		t.Fatalf("unknown feature err=%v", err)
	}
	if _, err = (Features{}).Enabled(
		context.Background(),
		"org-a",
		string(domain.FeatureScheduling),
	); err == nil {
		t.Fatal("missing feature store must fail closed")
	}
}

func TestFeatureHTTPRequiresTenantAdminAndOptimisticVersion(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	store := &featureStoreFake{flags: domain.FeatureFlags{
		OrganizationID: "org-a", SchedulingEnabled: true,
		Version: 2, UpdatedAt: now, UpdatedBy: "owner-a",
	}}
	handler := NewFeatureHTTP(
		Features{Store: store},
		featureAuthFake{principal: identitydomain.Principal{
			OrganizationID: "org-a", ActorID: "owner-a",
			Role: identitydomain.RoleOwner, MembershipStatus: "active",
			OrganizationStatus: "ready",
		}},
	).Handler()
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/organizations/org-a/features",
		bytes.NewBufferString(`{
		  "scheduling_enabled":true,
		  "whatsapp_enabled":false,
		  "google_calendar_enabled":false,
		  "fiscal_real_enabled":false,
		  "expected_version":1
		}`),
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
	if store.command.OrganizationID != "org-a" ||
		store.command.ActorID != "owner-a" ||
		store.command.ExpectedVersion != 1 ||
		!store.command.SchedulingEnabled {
		t.Fatalf("command=%+v", store.command)
	}

	memberHandler := NewFeatureHTTP(
		Features{Store: store},
		featureAuthFake{principal: identitydomain.Principal{
			OrganizationID: "org-a", ActorID: "member-a",
			Role: identitydomain.RoleMember, MembershipStatus: "active",
			OrganizationStatus: "ready",
		}},
	).Handler()
	response = httptest.NewRecorder()
	memberHandler.ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodPut,
			"/api/v1/organizations/org-a/features",
			bytes.NewBufferString(`{
			  "scheduling_enabled":true,
			  "whatsapp_enabled":true,
			  "google_calendar_enabled":true,
			  "fiscal_real_enabled":true,
			  "expected_version":2
			}`),
		),
	)
	if response.Code != http.StatusForbidden {
		t.Fatalf("member status=%d body=%s", response.Code, response.Body)
	}
}
