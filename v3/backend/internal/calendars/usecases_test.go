package calendars

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
)

type calendarUsecaseStore struct {
	connection domain.Connection
	grant      domain.OAuthGrant
	state      domain.OAuthState
	consumed   bool
}

func (store *calendarUsecaseStore) BeginOAuth(
	_ context.Context,
	connection domain.Connection,
	state domain.OAuthState,
) error {
	store.connection, store.state = connection, state
	return nil
}

func (store *calendarUsecaseStore) ConsumeOAuthState(
	_ context.Context,
	organizationID, actorID, sessionBinding, hash string,
	now time.Time,
) (domain.OAuthState, error) {
	if store.state.OrganizationID != organizationID ||
		store.state.ActorID != actorID ||
		store.state.SessionBinding != sessionBinding ||
		store.state.Hash != hash {
		return domain.OAuthState{}, domain.ErrOAuthStateInvalid
	}
	if store.consumed {
		return domain.OAuthState{}, domain.ErrOAuthStateConsumed
	}
	if !store.state.ExpiresAt.After(now) {
		return domain.OAuthState{}, domain.ErrOAuthStateExpired
	}
	store.consumed = true
	return store.state, nil
}

func (store *calendarUsecaseStore) SaveConnectionGrant(
	_ context.Context,
	connection domain.Connection,
	grant domain.OAuthGrant,
) error {
	store.connection, store.grant = connection, grant
	return nil
}

func (store *calendarUsecaseStore) GetConnection(
	_ context.Context,
	organizationID, connectionID string,
) (domain.Connection, domain.OAuthGrant, error) {
	if store.connection.OrganizationID != organizationID ||
		store.connection.ID != connectionID {
		return domain.Connection{}, domain.OAuthGrant{}, domain.ErrNotFound
	}
	return store.connection, store.grant, nil
}

func (store *calendarUsecaseStore) ListConnections(
	_ context.Context,
	organizationID string,
) ([]domain.Connection, error) {
	if store.connection.OrganizationID != organizationID {
		return nil, nil
	}
	return []domain.Connection{store.connection}, nil
}

func (store *calendarUsecaseStore) RevokeConnection(
	_ context.Context,
	organizationID, connectionID string,
	now time.Time,
) error {
	if store.connection.OrganizationID != organizationID ||
		store.connection.ID != connectionID {
		return domain.ErrNotFound
	}
	store.connection.Status = domain.ConnectionRevoked
	store.connection.UpdatedAt = now
	store.grant = domain.OAuthGrant{}
	return nil
}

type calendarUsecaseGoogle struct {
	state               string
	scopes              []string
	createErr           error
	findID              string
	revokeToken         string
	createCalendarCalls int
}

func (google *calendarUsecaseGoogle) AuthorizationURL(
	state string,
	scopes []string,
) (string, error) {
	google.state = state
	google.scopes = append([]string(nil), scopes...)
	return "https://accounts.example/authorize?state=" +
		url.QueryEscape(state), nil
}

func (google *calendarUsecaseGoogle) Exchange(
	context.Context,
	string,
) (domain.OAuthGrant, error) {
	return testOAuthGrant(), nil
}

func (google *calendarUsecaseGoogle) Refresh(
	context.Context,
	string,
) (domain.OAuthGrant, error) {
	return testOAuthGrant(), nil
}

func (google *calendarUsecaseGoogle) Revoke(
	_ context.Context,
	token string,
) error {
	google.revokeToken = token
	return nil
}

func (google *calendarUsecaseGoogle) CreateCalendar(
	context.Context,
	domain.OAuthGrant,
	string,
	string,
	string,
) (string, error) {
	google.createCalendarCalls++
	if google.createErr != nil {
		return "", google.createErr
	}
	return "calendar-id", nil
}

func (google *calendarUsecaseGoogle) FindCalendar(
	context.Context,
	domain.OAuthGrant,
	string,
) (string, error) {
	if google.findID == "" {
		return "", domain.ErrNotFound
	}
	return google.findID, nil
}

func testOAuthGrant() domain.OAuthGrant {
	return domain.OAuthGrant{
		AccessToken: "access", RefreshToken: "refresh",
		TokenType: "Bearer", Scope: scopeCalendarCreated,
		ExpiresAt: time.Date(2026, 8, 1, 2, 0, 0, 0, time.UTC),
	}
}

func TestOAuthFlowBindsStateToSessionAndReconcilesLostCalendarResponse(
	t *testing.T,
) {
	t.Parallel()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	store := &calendarUsecaseStore{}
	google := &calendarUsecaseGoogle{
		createErr: domain.ErrUncertain, findID: "calendar-after-timeout",
	}
	commands := Commands{
		Repository: store, Google: google,
		Random: bytes.NewReader(bytes.Repeat([]byte{0x42}, 32)),
		Now:    func() time.Time { return now },
	}
	start, err := commands.StartGoogleOAuth(
		context.Background(),
		StartOAuthInput{
			OrganizationID: "org-a", ActorID: "user-a",
			SessionBinding: "session-a", ConnectionID: "connection-a",
			TimeZone:        "America/Argentina/Buenos_Aires",
			FreeBusyEnabled: true, MeetEnabled: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if start.AuthorizationURL == "" ||
		strings.Contains(store.state.Hash, google.state) ||
		len(store.state.Hash) != 64 {
		t.Fatalf("unsafe OAuth state persistence: start=%+v state=%+v", start, store.state)
	}
	if !contains(google.scopes, scopeCalendarCreated) ||
		!contains(google.scopes, scopeCalendarListRead) ||
		!contains(google.scopes, scopeCalendarFreeBusy) {
		t.Fatalf("scopes = %v", google.scopes)
	}
	if _, err := commands.CompleteGoogleOAuth(
		context.Background(),
		CompleteOAuthInput{
			ActorID: "user-a", SessionBinding: "other-session",
			State: google.state, Code: "code",
		},
	); !errors.Is(err, domain.ErrOAuthStateInvalid) {
		t.Fatalf("cross-session callback = %v", err)
	}
	connection, err := commands.CompleteGoogleOAuth(
		context.Background(),
		CompleteOAuthInput{
			ActorID: "user-a", SessionBinding: "session-a",
			State: google.state, Code: "code",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if connection.Status != domain.ConnectionActive ||
		connection.CalendarID != "calendar-after-timeout" ||
		google.createCalendarCalls != 1 {
		t.Fatalf("connection = %+v calls=%d", connection, google.createCalendarCalls)
	}
	if _, err := commands.CompleteGoogleOAuth(
		context.Background(),
		CompleteOAuthInput{
			ActorID: "user-a", SessionBinding: "session-a",
			State: google.state, Code: "code",
		},
	); !errors.Is(err, domain.ErrOAuthStateConsumed) {
		t.Fatalf("replayed callback = %v", err)
	}
}

func TestDisconnectRevokesProviderTokenAndLocalGrant(t *testing.T) {
	t.Parallel()
	store := &calendarUsecaseStore{
		connection: domain.Connection{
			ID: "connection", OrganizationID: "org", ActorID: "user",
			Provider: "google", Status: domain.ConnectionActive,
			CalendarID: "calendar", TimeZone: "UTC",
			Scopes: []string{scopeCalendarCreated}, Version: 1,
		},
		grant: testOAuthGrant(),
	}
	google := &calendarUsecaseGoogle{}
	commands := Commands{
		Repository: store, Google: google,
		Now: func() time.Time {
			return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
		},
	}
	if err := commands.Disconnect(
		context.Background(), "org", "connection",
	); err != nil {
		t.Fatal(err)
	}
	if google.revokeToken != "refresh" ||
		store.connection.Status != domain.ConnectionRevoked ||
		store.grant.AccessToken != "" {
		t.Fatalf(
			"revoke token=%q connection=%+v grant=%+v",
			google.revokeToken, store.connection, store.grant,
		)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
