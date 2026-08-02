package helpers

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/calendars/usecases/domain"
	workermodels "github.com/devpablocristo/pymes/v3/backend/internal/calendars/worker/models"
)

func DecodeSyncRequested(
	organizationID string,
	payload json.RawMessage,
) (domain.CalendarSyncCommand, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var input workermodels.CalendarSyncRequested
	if err := decoder.Decode(&input); err != nil {
		return domain.CalendarSyncCommand{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.CalendarSyncCommand{}, fmt.Errorf("multiple JSON values are forbidden")
	}
	command := domain.CalendarSyncCommand{
		CommandID: input.CommandID, OrganizationID: organizationID,
		BookingID:      input.BookingID,
		Operation:      domain.SyncOperation(input.Operation),
		SourceVersion:  input.SourceVersion,
		SnapshotDigest: input.SnapshotDigest,
		CorrelationID:  input.CorrelationID,
		Summary:        input.Summary, Description: input.Description,
		Location: input.Location, TimeZone: input.TimeZone,
		AttendeeEmails: append([]string(nil), input.AttendeeEmails...),
		MeetRequested:  input.MeetRequested,
	}
	var err error
	if input.Start != "" {
		command.Start, err = time.Parse(time.RFC3339, input.Start)
		if err != nil {
			return domain.CalendarSyncCommand{}, fmt.Errorf("invalid start")
		}
	}
	if input.End != "" {
		command.End, err = time.Parse(time.RFC3339, input.End)
		if err != nil {
			return domain.CalendarSyncCommand{}, fmt.Errorf("invalid end")
		}
	}
	if !command.Valid() {
		return domain.CalendarSyncCommand{}, fmt.Errorf("invalid calendar sync command")
	}
	return command, nil
}

func EventID(organizationID, connectionID, bookingID string) string {
	return deterministicID(
		"event", organizationID, connectionID, bookingID,
	)
}

func MeetRequestID(
	organizationID, connectionID, bookingID string,
) string {
	return deterministicID(
		"meet", organizationID, connectionID, bookingID,
	)
}

func deterministicID(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(part))
	}
	return strings.ToLower(
		base32.HexEncoding.WithPadding(base32.NoPadding).
			EncodeToString(hash.Sum(nil)),
	)
}

func StableErrorCode(err error) string {
	switch err {
	case domain.ErrNotFound:
		return "CALENDAR_NOT_FOUND"
	case domain.ErrConflict:
		return "CALENDAR_CONFLICT"
	case domain.ErrPreconditionFailed:
		return "CALENDAR_PRECONDITION_FAILED"
	case domain.ErrUncertain:
		return "CALENDAR_UNCERTAIN"
	case domain.ErrReauthRequired:
		return "CALENDAR_REAUTH_REQUIRED"
	case domain.ErrProviderUnavailable:
		return "CALENDAR_PROVIDER_UNAVAILABLE"
	default:
		return "CALENDAR_SYNC_FAILED"
	}
}
