package domain

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

type OutboxEvent struct {
	ID             string
	OrganizationID string
	Topic          string
	Payload        json.RawMessage
	Attempts       int
	LeaseToken     string
	AvailableAt    time.Time
	LeaseExpiresAt time.Time
}

type ConnectionStatus string

const (
	ConnectionPending        ConnectionStatus = "pending"
	ConnectionActive         ConnectionStatus = "active"
	ConnectionReauthRequired ConnectionStatus = "reauth_required"
	ConnectionRevoked        ConnectionStatus = "revoked"
)

type OAuthGrant struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scope        string
	ExpiresAt    time.Time
}

func (grant OAuthGrant) Valid() bool {
	return strings.TrimSpace(grant.AccessToken) != "" &&
		strings.TrimSpace(grant.RefreshToken) != "" &&
		!grant.ExpiresAt.IsZero()
}

type Connection struct {
	ID                string
	OrganizationID    string
	ActorID           string
	Provider          string
	Status            ConnectionStatus
	CalendarID        string
	TimeZone          string
	Scopes            []string
	FreeBusyEnabled   bool
	MeetEnabled       bool
	AccessTokenExpiry time.Time
	Version           int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (connection Connection) Valid() bool {
	if connection.ID == "" || connection.OrganizationID == "" ||
		connection.ActorID == "" || connection.Provider != "google" ||
		connection.TimeZone == "" || len(connection.Scopes) == 0 ||
		connection.Version < 1 {
		return false
	}
	switch connection.Status {
	case ConnectionPending, ConnectionActive, ConnectionReauthRequired, ConnectionRevoked:
		return true
	default:
		return false
	}
}

type OAuthState struct {
	Hash            string
	OrganizationID  string
	ActorID         string
	ConnectionID    string
	SessionBinding  string
	TimeZone        string
	FreeBusyEnabled bool
	MeetEnabled     bool
	ExpiresAt       time.Time
	ConsumedAt      *time.Time
	CreatedAt       time.Time
}

func (state OAuthState) Valid(now time.Time) bool {
	return len(state.Hash) == 64 &&
		state.OrganizationID != "" &&
		state.ActorID != "" &&
		state.ConnectionID != "" &&
		state.SessionBinding != "" &&
		state.TimeZone != "" &&
		state.ExpiresAt.After(now)
}

type SyncOperation string

const (
	SyncUpsert SyncOperation = "upsert"
	SyncDelete SyncOperation = "delete"
)

type CalendarSyncCommand struct {
	CommandID      string
	OrganizationID string
	BookingID      string
	Operation      SyncOperation
	SourceVersion  int
	SnapshotDigest string
	CorrelationID  string
	Summary        string
	Description    string
	Location       string
	Start          time.Time
	End            time.Time
	TimeZone       string
	AttendeeEmails []string
	MeetRequested  bool
}

func (command CalendarSyncCommand) Valid() bool {
	if command.CommandID == "" || command.OrganizationID == "" ||
		command.BookingID == "" || command.SourceVersion < 1 ||
		len(command.SnapshotDigest) != 64 || command.CorrelationID == "" {
		return false
	}
	switch command.Operation {
	case SyncDelete:
		return true
	case SyncUpsert:
		return command.Summary != "" &&
			!command.Start.IsZero() &&
			command.End.After(command.Start) &&
			command.TimeZone != ""
	default:
		return false
	}
}

type ExternalEventStatus string

const (
	ExternalEventPending   ExternalEventStatus = "pending"
	ExternalEventSynced    ExternalEventStatus = "synced"
	ExternalEventDeleting  ExternalEventStatus = "deleting"
	ExternalEventDeleted   ExternalEventStatus = "deleted"
	ExternalEventUncertain ExternalEventStatus = "uncertain"
	ExternalEventReconcile ExternalEventStatus = "reconcile"
)

type ExternalEvent struct {
	OrganizationID string
	ConnectionID   string
	BookingID      string
	GoogleEventID  string
	ETag           string
	MeetRequestID  string
	MeetStatus     string
	MeetURI        string
	SourceVersion  int
	SnapshotDigest string
	Status         ExternalEventStatus
	LastErrorCode  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (event ExternalEvent) Valid() bool {
	return event.OrganizationID != "" &&
		event.ConnectionID != "" &&
		event.BookingID != "" &&
		event.GoogleEventID != "" &&
		event.SourceVersion > 0 &&
		len(event.SnapshotDigest) == 64
}

type BusyPeriod struct {
	Start time.Time
	End   time.Time
}

type ProviderEvent struct {
	ID             string
	ETag           string
	SnapshotDigest string
	MeetStatus     string
	MeetURI        string
}

func (event ProviderEvent) MeetValid() bool {
	if event.MeetURI == "" {
		return true
	}
	parsed, err := url.Parse(event.MeetURI)
	return err == nil && parsed.Scheme == "https" &&
		parsed.Host == "meet.google.com"
}

var (
	ErrNotFound             = errors.New("CALENDAR_NOT_FOUND")
	ErrConflict             = errors.New("CALENDAR_CONFLICT")
	ErrPreconditionFailed   = errors.New("CALENDAR_PRECONDITION_FAILED")
	ErrUncertain            = errors.New("CALENDAR_UNCERTAIN")
	ErrReauthRequired       = errors.New("CALENDAR_REAUTH_REQUIRED")
	ErrProviderUnavailable  = errors.New("CALENDAR_PROVIDER_UNAVAILABLE")
	ErrOAuthStateInvalid    = errors.New("OAUTH_STATE_INVALID")
	ErrOAuthStateExpired    = errors.New("OAUTH_STATE_EXPIRED")
	ErrOAuthStateConsumed   = errors.New("OAUTH_STATE_CONSUMED")
	ErrOrganizationMismatch = errors.New("CALENDAR_ORGANIZATION_MISMATCH")
	ErrFeatureDisabled      = errors.New("FEATURE_DISABLED")
)
