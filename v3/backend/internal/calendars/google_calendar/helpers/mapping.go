package helpers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	google "github.com/devpablocristo/platform/sdks/google-calendar/go"
	googlemodels "github.com/devpablocristo/pymes/v3/backend/internal/calendars/google_calendar/models"
	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
)

func OAuthGrant(token google.Token, fallbackRefreshToken string) domain.OAuthGrant {
	refreshToken := token.RefreshToken
	if refreshToken == "" {
		refreshToken = fallbackRefreshToken
	}
	return domain.OAuthGrant{
		AccessToken: token.AccessToken, RefreshToken: refreshToken,
		TokenType: token.TokenType, Scope: token.Scope,
		ExpiresAt: token.ExpiresAt(),
	}
}

func EventInput(payload googlemodels.EventPayload) google.EventInput {
	input := google.EventInput{
		EventID: payload.EventID,
		Summary: payload.Summary, Description: payload.Description,
		Location: payload.Location,
		Start: google.EventDateTime{
			DateTime: payload.Start.UTC().Format("2006-01-02T15:04:05Z07:00"),
			TimeZone: payload.TimeZone,
		},
		End: google.EventDateTime{
			DateTime: payload.End.UTC().Format("2006-01-02T15:04:05Z07:00"),
			TimeZone: payload.TimeZone,
		},
		ExtendedProperties: &google.ExtendedProperties{
			Private: map[string]string{
				"pymes_managed":         "true",
				"pymes_snapshot_digest": payload.SnapshotDigest,
			},
		},
	}
	for _, email := range payload.Attendees {
		if email = strings.TrimSpace(email); email != "" {
			input.Attendees = append(
				input.Attendees,
				google.EventAttendee{Email: email},
			)
		}
	}
	if payload.MeetRequested {
		input.ConferenceData = google.NewMeetConferenceData(
			payload.MeetRequestID,
		)
	}
	return input
}

func ProviderEvent(event google.Event, meetRequested bool) (domain.ProviderEvent, error) {
	output := domain.ProviderEvent{
		ID: event.ID, ETag: event.ETag, MeetURI: event.ConferenceData.MeetURI(),
	}
	if event.ExtendedProperties != nil {
		output.SnapshotDigest =
			event.ExtendedProperties.Private["pymes_snapshot_digest"]
	}
	if event.ConferenceData != nil && event.ConferenceData.CreateRequest != nil {
		output.MeetStatus = event.ConferenceData.CreateRequest.Status.StatusCode
	}
	if meetRequested {
		if event.ConferenceData != nil &&
			event.ConferenceData.ConferenceSolution != nil &&
			event.ConferenceData.ConferenceSolution.Key.Type != "" &&
			event.ConferenceData.ConferenceSolution.Key.Type !=
				google.ConferenceSolutionGoogleMeet {
			return domain.ProviderEvent{}, errors.New("CALENDAR_MEET_PROVIDER_INVALID")
		}
		if output.MeetStatus == google.ConferenceStatusSuccess &&
			output.MeetURI == "" {
			return domain.ProviderEvent{}, errors.New("CALENDAR_MEET_URI_MISSING")
		}
	}
	if !output.MeetValid() {
		return domain.ProviderEvent{}, errors.New("CALENDAR_MEET_URI_INVALID")
	}
	return output, nil
}

func TranslateError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case google.IsNotFound(err):
		return domain.ErrNotFound
	case google.IsConflict(err):
		return domain.ErrConflict
	case google.IsPreconditionFailed(err):
		return domain.ErrPreconditionFailed
	case google.IsTimeout(err), errors.Is(err, context.DeadlineExceeded):
		return domain.ErrUncertain
	case google.StatusCode(err) == http.StatusUnauthorized ||
		google.StatusCode(err) == http.StatusForbidden:
		return domain.ErrReauthRequired
	case google.StatusCode(err) >= 500:
		return domain.ErrProviderUnavailable
	default:
		return err
	}
}
