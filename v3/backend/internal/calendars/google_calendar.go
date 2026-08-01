// architecture:adapter external
package calendars

import (
	"context"
	"fmt"
	"net/http"
	"time"

	google "github.com/devpablocristo/platform/sdks/google-calendar/go"
	googlehelpers "github.com/devpablocristo/pymes/v3/backend/internal/calendars/google_calendar/helpers"
	googlemodels "github.com/devpablocristo/pymes/v3/backend/internal/calendars/google_calendar/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
)

type GoogleCalendar struct {
	Config googlemodels.Configuration
}

func NewGoogleCalendar(config googlemodels.Configuration) (*GoogleCalendar, error) {
	adapter := &GoogleCalendar{Config: config}
	if err := adapter.oauthConfig(nil).Validate(); err != nil {
		return nil, err
	}
	return adapter, nil
}

func (adapter *GoogleCalendar) AuthorizationURL(
	state string,
	scopes []string,
) (string, error) {
	return google.BuildAuthURL(adapter.oauthConfig(scopes), state)
}

func (adapter *GoogleCalendar) Exchange(
	ctx context.Context,
	code string,
) (domain.OAuthGrant, error) {
	token, err := google.ExchangeCode(ctx, adapter.oauthConfig(nil), code)
	if err != nil {
		return domain.OAuthGrant{}, googlehelpers.TranslateError(err)
	}
	grant := googlehelpers.OAuthGrant(token, "")
	if !grant.Valid() {
		return domain.OAuthGrant{}, domain.ErrReauthRequired
	}
	return grant, nil
}

func (adapter *GoogleCalendar) Refresh(
	ctx context.Context,
	refreshToken string,
) (domain.OAuthGrant, error) {
	token, err := google.Refresh(
		ctx, adapter.oauthConfig(nil), refreshToken,
	)
	if err != nil {
		return domain.OAuthGrant{}, googlehelpers.TranslateError(err)
	}
	grant := googlehelpers.OAuthGrant(token, refreshToken)
	if !grant.Valid() {
		return domain.OAuthGrant{}, domain.ErrReauthRequired
	}
	return grant, nil
}

func (adapter *GoogleCalendar) Revoke(
	ctx context.Context,
	token string,
) error {
	return googlehelpers.TranslateError(
		google.Revoke(ctx, adapter.oauthConfig(nil), token),
	)
}

func (adapter *GoogleCalendar) CreateCalendar(
	ctx context.Context,
	grant domain.OAuthGrant,
	summary, timeZone, marker string,
) (string, error) {
	client, err := adapter.client(grant)
	if err != nil {
		return "", err
	}
	calendar, err := client.CreateCalendar(ctx, google.CalendarInput{
		Summary: summary, TimeZone: timeZone, Description: marker,
	})
	if err != nil {
		return "", googlehelpers.TranslateError(err)
	}
	if calendar.ID == "" {
		return "", fmt.Errorf("CALENDAR_PROVIDER_RESPONSE_INVALID")
	}
	return calendar.ID, nil
}

func (adapter *GoogleCalendar) FindCalendar(
	ctx context.Context,
	grant domain.OAuthGrant,
	marker string,
) (string, error) {
	client, err := adapter.client(grant)
	if err != nil {
		return "", err
	}
	pageToken := ""
	for {
		calendars, listErr := client.ListCalendarEntries(
			ctx,
			google.ListCalendarEntriesOptions{
				MaxResults: 250, MinAccessRole: "owner",
				PageToken: pageToken,
			},
		)
		if listErr != nil {
			return "", googlehelpers.TranslateError(listErr)
		}
		for _, calendar := range calendars.Items {
			if calendar.Description == marker && !calendar.Deleted {
				return calendar.ID, nil
			}
		}
		pageToken = calendars.NextPageToken
		if pageToken == "" {
			return "", domain.ErrNotFound
		}
	}
}

func (adapter *GoogleCalendar) CreateEvent(
	ctx context.Context,
	grant domain.OAuthGrant,
	calendarID string,
	eventID, meetRequestID string,
	command domain.CalendarSyncCommand,
) (domain.ProviderEvent, error) {
	client, err := adapter.client(grant)
	if err != nil {
		return domain.ProviderEvent{}, err
	}
	payload := eventPayload(eventID, meetRequestID, command)
	event, err := client.CreateEvent(
		ctx, calendarID, googlehelpers.EventInput(payload),
		google.CreateEventOptions{SendUpdates: google.SendUpdatesAll},
	)
	if err != nil {
		return domain.ProviderEvent{}, googlehelpers.TranslateError(err)
	}
	return googlehelpers.ProviderEvent(event, payload.MeetRequested)
}

func (adapter *GoogleCalendar) GetEvent(
	ctx context.Context,
	grant domain.OAuthGrant,
	calendarID, eventID string,
	meetRequested bool,
) (domain.ProviderEvent, error) {
	client, err := adapter.client(grant)
	if err != nil {
		return domain.ProviderEvent{}, err
	}
	event, err := client.GetEvent(
		ctx, calendarID, eventID, google.GetEventOptions{},
	)
	if err != nil {
		return domain.ProviderEvent{}, googlehelpers.TranslateError(err)
	}
	return googlehelpers.ProviderEvent(event, meetRequested)
}

func (adapter *GoogleCalendar) UpdateEvent(
	ctx context.Context,
	grant domain.OAuthGrant,
	calendarID, eventID, etag string,
	meetRequestID string,
	command domain.CalendarSyncCommand,
) (domain.ProviderEvent, error) {
	client, err := adapter.client(grant)
	if err != nil {
		return domain.ProviderEvent{}, err
	}
	payload := eventPayload(eventID, meetRequestID, command)
	input := googlehelpers.EventInput(payload)
	input.EventID = ""
	event, err := client.UpdateEvent(
		ctx, calendarID, eventID, input,
		google.UpdateEventOptions{
			ETag: etag, SendUpdates: google.SendUpdatesAll,
		},
	)
	if err != nil {
		return domain.ProviderEvent{}, googlehelpers.TranslateError(err)
	}
	return googlehelpers.ProviderEvent(event, payload.MeetRequested)
}

func eventPayload(
	eventID, meetRequestID string,
	command domain.CalendarSyncCommand,
) googlemodels.EventPayload {
	return googlemodels.EventPayload{
		EventID: eventID, MeetRequestID: meetRequestID,
		SnapshotDigest: command.SnapshotDigest,
		Summary:        command.Summary, Description: command.Description,
		Location: command.Location, Start: command.Start, End: command.End,
		TimeZone:      command.TimeZone,
		Attendees:     append([]string(nil), command.AttendeeEmails...),
		MeetRequested: command.MeetRequested && meetRequestID != "",
	}
}

func (adapter *GoogleCalendar) DeleteEvent(
	ctx context.Context,
	grant domain.OAuthGrant,
	calendarID, eventID, etag string,
) error {
	client, err := adapter.client(grant)
	if err != nil {
		return err
	}
	return googlehelpers.TranslateError(client.DeleteEvent(
		ctx, calendarID, eventID,
		google.DeleteEventOptions{
			ETag: etag, SendUpdates: google.SendUpdatesAll,
		},
	))
}

func (adapter *GoogleCalendar) QueryFreeBusy(
	ctx context.Context,
	grant domain.OAuthGrant,
	calendarID string,
	start, end string,
	timeZone string,
) ([]domain.BusyPeriod, error) {
	client, err := adapter.client(grant)
	if err != nil {
		return nil, err
	}
	response, err := client.QueryFreeBusy(ctx, google.FreeBusyRequest{
		TimeMin: start, TimeMax: end, TimeZone: timeZone,
		Items: []google.FreeBusyItem{{ID: calendarID}},
	})
	if err != nil {
		return nil, googlehelpers.TranslateError(err)
	}
	var periods []domain.BusyPeriod
	for _, period := range response.Calendars[calendarID].Busy {
		startAt, startErr := time.Parse(time.RFC3339, period.Start)
		endAt, endErr := time.Parse(time.RFC3339, period.End)
		if startErr != nil || endErr != nil || !endAt.After(startAt) {
			return nil, fmt.Errorf("CALENDAR_PROVIDER_RESPONSE_INVALID")
		}
		periods = append(periods, domain.BusyPeriod{
			Start: startAt, End: endAt,
		})
	}
	return periods, nil
}

func (adapter *GoogleCalendar) oauthConfig(scopes []string) google.Config {
	return google.Config{
		ClientID:     adapter.Config.ClientID,
		ClientSecret: adapter.Config.ClientSecret,
		RedirectURL:  adapter.Config.RedirectURL,
		Scopes:       scopes, AuthURL: adapter.Config.AuthURL,
		TokenURL: adapter.Config.TokenURL, RevokeURL: adapter.Config.RevokeURL,
		HTTPClient: adapter.httpClient(),
	}
}

func (adapter *GoogleCalendar) client(
	grant domain.OAuthGrant,
) (*google.Client, error) {
	if !grant.Valid() {
		return nil, domain.ErrReauthRequired
	}
	client, err := google.NewClient(google.ClientConfig{
		AccessToken: grant.AccessToken,
		BaseURL:     adapter.Config.CalendarURL,
		HTTPClient:  adapter.httpClient(),
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (adapter *GoogleCalendar) httpClient() *http.Client {
	if adapter.Config.HTTPClient != nil {
		return adapter.Config.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}
