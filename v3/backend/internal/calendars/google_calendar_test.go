package calendars

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	googlemodels "github.com/devpablocristo/pymes/v3/backend/internal/calendars/google_calendar/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
)

func TestGoogleCalendarAdapterUsesPublishedSDKContract(t *testing.T) {
	t.Parallel()
	var mutex sync.Mutex
	var receivedETag string
	var createdEventID string
	server := httptest.NewServer(http.HandlerFunc(
		func(response http.ResponseWriter, request *http.Request) {
			response.Header().Set("Content-Type", "application/json")
			switch {
			case request.URL.Path == "/token":
				if err := request.ParseForm(); err != nil {
					t.Errorf("parse token form: %v", err)
				}
				if request.Form.Get("grant_type") == "authorization_code" {
					_, _ = io.WriteString(response,
						`{"access_token":"access","refresh_token":"refresh",`+
							`"token_type":"Bearer","expires_in":3600,`+
							`"scope":"https://www.googleapis.com/auth/calendar.app.created"}`,
					)
					return
				}
				_, _ = io.WriteString(response,
					`{"access_token":"refreshed","token_type":"Bearer",`+
						`"expires_in":3600}`,
				)
			case request.URL.Path == "/revoke":
				response.WriteHeader(http.StatusOK)
			case request.URL.Path == "/calendars" &&
				request.Method == http.MethodPost:
				_, _ = io.WriteString(
					response,
					`{"id":"calendar-secondary","summary":"Pymes"}`,
				)
			case request.URL.Path == "/users/me/calendarList":
				_, _ = io.WriteString(
					response,
					`{"items":[{"id":"calendar-secondary",`+
						`"description":"pymes-connection:connection"}]}`,
				)
			case request.URL.Path == "/calendars/calendar-secondary/events" &&
				request.Method == http.MethodPost:
				var input map[string]any
				if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
					t.Errorf("decode create: %v", err)
					return
				}
				createdEventID, _ = input["id"].(string)
				writeProviderEvent(
					response, createdEventID, `"v1"`,
					strings.Repeat("a", 64), "pending", "",
				)
			case strings.HasPrefix(
				request.URL.Path,
				"/calendars/calendar-secondary/events/",
			) && request.Method == http.MethodGet:
				writeProviderEvent(
					response, createdEventID, `"v1"`,
					strings.Repeat("a", 64), "pending", "",
				)
			case strings.HasPrefix(
				request.URL.Path,
				"/calendars/calendar-secondary/events/",
			) && request.Method == http.MethodPut:
				mutex.Lock()
				receivedETag = request.Header.Get("If-Match")
				mutex.Unlock()
				writeProviderEvent(
					response, createdEventID, `"v2"`,
					strings.Repeat("b", 64), "success",
					"https://meet.google.com/abc-defg-hij",
				)
			case strings.HasPrefix(
				request.URL.Path,
				"/calendars/calendar-secondary/events/",
			) && request.Method == http.MethodDelete:
				mutex.Lock()
				receivedETag = request.Header.Get("If-Match")
				mutex.Unlock()
				response.WriteHeader(http.StatusNoContent)
			case request.URL.Path == "/freeBusy":
				_, _ = io.WriteString(
					response,
					`{"calendars":{"calendar-secondary":{"busy":[`+
						`{"start":"2026-08-01T10:00:00Z",`+
						`"end":"2026-08-01T11:00:00Z"}]}}}`,
				)
			default:
				http.Error(response, `{"error":{"code":404}}`, http.StatusNotFound)
			}
		},
	))
	defer server.Close()

	adapter, err := NewGoogleCalendar(googlemodels.Configuration{
		ClientID: "client", ClientSecret: "secret",
		RedirectURL: server.URL + "/callback",
		AuthURL:     server.URL + "/authorize", TokenURL: server.URL + "/token",
		RevokeURL: server.URL + "/revoke", CalendarURL: server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := adapter.AuthorizationURL(
		"state-value",
		[]string{scopeCalendarCreated, scopeCalendarListRead},
	)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("state") != "state-value" ||
		parsed.Query().Get("access_type") != "offline" {
		t.Fatalf("authorization URL=%s", authorizationURL)
	}

	grant, err := adapter.Exchange(context.Background(), "code")
	if err != nil || grant.AccessToken != "access" ||
		grant.RefreshToken != "refresh" {
		t.Fatalf("exchange grant=%+v err=%v", grant, err)
	}
	refreshed, err := adapter.Refresh(context.Background(), grant.RefreshToken)
	if err != nil || refreshed.AccessToken != "refreshed" ||
		refreshed.RefreshToken != grant.RefreshToken {
		t.Fatalf("refresh grant=%+v err=%v", refreshed, err)
	}
	if err := adapter.Revoke(context.Background(), grant.RefreshToken); err != nil {
		t.Fatal(err)
	}
	calendarID, err := adapter.CreateCalendar(
		context.Background(), grant, "Pymes", "UTC",
		"pymes-connection:connection",
	)
	if err != nil || calendarID != "calendar-secondary" {
		t.Fatalf("calendar id=%q err=%v", calendarID, err)
	}
	foundID, err := adapter.FindCalendar(
		context.Background(), grant, "pymes-connection:connection",
	)
	if err != nil || foundID != calendarID {
		t.Fatalf("found id=%q err=%v", foundID, err)
	}

	start := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	command := domain.CalendarSyncCommand{
		CommandID: "command", OrganizationID: "organization",
		BookingID: "booking", Operation: domain.SyncUpsert,
		SourceVersion: 1, SnapshotDigest: strings.Repeat("a", 64),
		CorrelationID: "correlation", Summary: "Consulta",
		Start: start, End: start.Add(time.Hour), TimeZone: "UTC",
		MeetRequested: true,
	}
	eventID := strings.Repeat("a", 52)
	event, err := adapter.CreateEvent(
		context.Background(), grant, calendarID, eventID,
		strings.Repeat("b", 52), command,
	)
	if err != nil || event.ID != eventID ||
		event.MeetStatus != "pending" {
		t.Fatalf("created event=%+v err=%v", event, err)
	}
	if _, err := adapter.GetEvent(
		context.Background(), grant, calendarID, eventID, true,
	); err != nil {
		t.Fatal(err)
	}
	command.SourceVersion = 2
	command.SnapshotDigest = strings.Repeat("b", 64)
	updated, err := adapter.UpdateEvent(
		context.Background(), grant, calendarID, eventID, `"v1"`,
		strings.Repeat("b", 52), command,
	)
	if err != nil || updated.MeetStatus != "success" ||
		updated.MeetURI != "https://meet.google.com/abc-defg-hij" {
		t.Fatalf("updated event=%+v err=%v", updated, err)
	}
	mutex.Lock()
	if receivedETag != `"v1"` {
		t.Fatalf("update If-Match=%q", receivedETag)
	}
	mutex.Unlock()
	periods, err := adapter.QueryFreeBusy(
		context.Background(), grant, calendarID,
		start.Format(time.RFC3339), start.Add(2*time.Hour).Format(time.RFC3339),
		"UTC",
	)
	if err != nil || len(periods) != 1 ||
		!periods[0].Start.Equal(start) {
		t.Fatalf("busy=%+v err=%v", periods, err)
	}
	if err := adapter.DeleteEvent(
		context.Background(), grant, calendarID, eventID, `"v2"`,
	); err != nil {
		t.Fatal(err)
	}
	mutex.Lock()
	if receivedETag != `"v2"` {
		t.Fatalf("delete If-Match=%q", receivedETag)
	}
	mutex.Unlock()
}

func TestGoogleCalendarAdapterTranslatesRecoveryErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status int
		want   error
	}{
		{status: http.StatusUnauthorized, want: domain.ErrReauthRequired},
		{status: http.StatusConflict, want: domain.ErrConflict},
		{status: http.StatusPreconditionFailed, want: domain.ErrPreconditionFailed},
		{status: http.StatusNotFound, want: domain.ErrNotFound},
		{status: http.StatusServiceUnavailable, want: domain.ErrProviderUnavailable},
	}
	for _, test := range tests {
		test := test
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(
				func(response http.ResponseWriter, _ *http.Request) {
					response.Header().Set("Content-Type", "application/json")
					response.WriteHeader(test.status)
					_, _ = io.WriteString(
						response,
						`{"error":{"code":`+
							strconv.Itoa(test.status)+
							`,"message":"provider failure"}}`,
					)
				},
			))
			defer server.Close()
			adapter, err := NewGoogleCalendar(googlemodels.Configuration{
				ClientID: "client", ClientSecret: "secret",
				RedirectURL: "https://app.test/callback",
				CalendarURL: server.URL, HTTPClient: server.Client(),
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = adapter.GetEvent(
				context.Background(), workerGrant(), "calendar", "event", false,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("status=%d err=%v want=%v", test.status, err, test.want)
			}
		})
	}
	adapter, err := NewGoogleCalendar(googlemodels.Configuration{
		ClientID: "client", ClientSecret: "secret",
		RedirectURL: "https://app.test/callback",
		CalendarURL: "https://calendar.test",
		HTTPClient: &http.Client{Transport: roundTripFunc(
			func(*http.Request) (*http.Response, error) {
				return nil, context.DeadlineExceeded
			},
		)},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.GetEvent(
		context.Background(), workerGrant(), "calendar", "event", false,
	)
	if !errors.Is(err, domain.ErrUncertain) {
		t.Fatalf("timeout err=%v", err)
	}
}

func writeProviderEvent(
	response http.ResponseWriter,
	eventID, etag, digest, meetStatus, meetURI string,
) {
	entryPoints := []map[string]string(nil)
	if meetURI != "" {
		entryPoints = append(entryPoints, map[string]string{
			"entryPointType": "video", "uri": meetURI,
		})
	}
	_ = json.NewEncoder(response).Encode(map[string]any{
		"id": eventID, "etag": etag,
		"extendedProperties": map[string]any{
			"private": map[string]string{
				"pymes_snapshot_digest": digest,
			},
		},
		"conferenceData": map[string]any{
			"createRequest": map[string]any{
				"requestId": "meet",
				"conferenceSolutionKey": map[string]string{
					"type": "hangoutsMeet",
				},
				"status": map[string]string{"statusCode": meetStatus},
			},
			"conferenceSolution": map[string]any{
				"key": map[string]string{"type": "hangoutsMeet"},
			},
			"entryPoints": entryPoints,
		},
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	return function(request)
}
