package models

import (
	"net/http"
	"time"
)

type Configuration struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	RevokeURL    string
	CalendarURL  string
	HTTPClient   *http.Client
}

type EventPayload struct {
	EventID        string
	MeetRequestID  string
	SnapshotDigest string
	Summary        string
	Description    string
	Location       string
	Start          time.Time
	End            time.Time
	TimeZone       string
	Attendees      []string
	MeetRequested  bool
}
