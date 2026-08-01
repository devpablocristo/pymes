package calendars

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
	identitydomain "github.com/devpablocristo/pymes/v3/backend/internal/identity/usecases/domain"
)

type calendarHandlerAuth struct {
	principal identitydomain.Principal
	err       error
}

func (auth calendarHandlerAuth) Principal(
	*http.Request,
) (identitydomain.Principal, error) {
	return auth.principal, auth.err
}

type calendarHandlerCommands struct {
	startInput    StartOAuthInput
	completeInput CompleteOAuthInput
	connection    domain.Connection
}

func (commands *calendarHandlerCommands) StartGoogleOAuth(
	_ context.Context,
	input StartOAuthInput,
) (OAuthStart, error) {
	commands.startInput = input
	return OAuthStart{
		ConnectionID:     input.ConnectionID,
		AuthorizationURL: "https://accounts.example/authorize?state=opaque",
		ExpiresAt:        time.Date(2026, 8, 1, 0, 10, 0, 0, time.UTC),
	}, nil
}

func (commands *calendarHandlerCommands) CompleteGoogleOAuth(
	_ context.Context,
	input CompleteOAuthInput,
) (domain.Connection, error) {
	commands.completeInput = input
	return commands.connection, nil
}

func (commands *calendarHandlerCommands) ListConnections(
	context.Context,
	string,
) ([]domain.Connection, error) {
	return []domain.Connection{commands.connection}, nil
}

func (commands *calendarHandlerCommands) Disconnect(
	context.Context,
	string,
	string,
) error {
	return nil
}

func TestCalendarBFFBindsOAuthToVerifiedClerkSessionAndHidesTokens(
	t *testing.T,
) {
	t.Parallel()
	commands := &calendarHandlerCommands{
		connection: domain.Connection{
			ID: "connection", OrganizationID: "org-a", ActorID: "user-a",
			Provider: "google", Status: domain.ConnectionActive,
			CalendarID: "calendar", TimeZone: "UTC",
			Scopes: []string{scopeCalendarCreated}, Version: 2,
			AccessTokenExpiry: time.Date(2026, 8, 1, 1, 0, 0, 0, time.UTC),
		},
	}
	handler := NewCalendarHTTP(commands, calendarHandlerAuth{
		principal: identitydomain.Principal{
			OrganizationID: "org-a", ActorID: "user-a",
			SessionID: "session-a", Role: identitydomain.RoleOwner,
			OrganizationStatus: "ready", MembershipStatus: "active",
		},
	}).Handler()
	startRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/organizations/org-a/calendars/google/oauth/start",
		strings.NewReader(
			`{"time_zone":"UTC","free_busy_enabled":true,"meet_enabled":true}`,
		),
	)
	startResponse := httptest.NewRecorder()
	handler.ServeHTTP(startResponse, startRequest)
	if startResponse.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", startResponse.Code, startResponse.Body)
	}
	if startResponse.Header().Get("Cache-Control") != "no-store" ||
		startResponse.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("OAuth response headers=%v", startResponse.Header())
	}
	if commands.startInput.SessionBinding != "session-a" ||
		commands.startInput.ActorID != "user-a" ||
		commands.startInput.OrganizationID != "org-a" {
		t.Fatalf("start input = %+v", commands.startInput)
	}
	state, err := domain.RoutedOAuthState(
		"org-a", bytes.Repeat([]byte{0x42}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}
	completeRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/calendars/google/oauth/callback?state="+state+
			"&code=authorization-code",
		nil,
	)
	completeResponse := httptest.NewRecorder()
	handler.ServeHTTP(completeResponse, completeRequest)
	if completeResponse.Code != http.StatusOK {
		t.Fatalf(
			"complete status=%d body=%s",
			completeResponse.Code, completeResponse.Body,
		)
	}
	body := completeResponse.Body.String()
	for _, forbidden := range []string{
		"authorization-code", state, `"access_token":`,
		`"refresh_token":`, `"calendar_id":`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("sensitive value %q leaked in %s", forbidden, body)
		}
	}
	if commands.completeInput.SessionBinding != "session-a" {
		t.Fatalf("complete input = %+v", commands.completeInput)
	}
	if commands.completeInput.State != state {
		t.Fatalf("complete state was not forwarded")
	}
}

func TestCalendarBFFCallbackRejectsTenantHintOutsideClerkSession(t *testing.T) {
	t.Parallel()
	state, err := domain.RoutedOAuthState(
		"org-a", bytes.Repeat([]byte{0x33}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewCalendarHTTP(
		&calendarHandlerCommands{},
		calendarHandlerAuth{principal: identitydomain.Principal{
			OrganizationID: "org-b", ActorID: "user-b",
			SessionID: "session-b", Role: identitydomain.RoleOwner,
			OrganizationStatus: "ready", MembershipStatus: "active",
		}},
	).Handler()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/calendars/google/oauth/callback?state="+state+"&code=code",
		nil,
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
}

func TestCalendarBFFRejectsCrossTenantAndMemberMutation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		principal identitydomain.Principal
	}{
		{
			name: "cross tenant",
			principal: identitydomain.Principal{
				OrganizationID: "org-b", ActorID: "user",
				SessionID: "session", Role: identitydomain.RoleOwner,
				OrganizationStatus: "ready", MembershipStatus: "active",
			},
		},
		{
			name: "member mutation",
			principal: identitydomain.Principal{
				OrganizationID: "org-a", ActorID: "user",
				SessionID: "session", Role: identitydomain.RoleMember,
				OrganizationStatus: "ready", MembershipStatus: "active",
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := NewCalendarHTTP(
				&calendarHandlerCommands{},
				calendarHandlerAuth{principal: test.principal},
			).Handler()
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/organizations/org-a/calendars/google/oauth/start",
				strings.NewReader(`{"time_zone":"UTC"}`),
			)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
		})
	}
}
