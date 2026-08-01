package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var kmsCryptoKeyPattern = regexp.MustCompile(
	`^projects/[^/]+/locations/[^/]+/keyRings/[^/]+/cryptoKeys/[^/]+$`,
)

// Calendars contains the workload configuration for the tenant-owned Google
// Calendar projection. Secrets are injected at runtime; they are never
// persisted by cmd/config or exposed by an HTTP response.
type Calendars struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	RedirectURL  string
	KMSKeyName   string
	LocalKey     []byte
	AuthURL      string
	TokenURL     string
	RevokeURL    string
	CalendarURL  string
}

func loadCalendars(
	getenv func(string) string,
	environment string,
) (Calendars, error) {
	enabled, err := parseConfigBoolean(
		"PYMES_GOOGLE_CALENDAR_ENABLED",
		getenv("PYMES_GOOGLE_CALENDAR_ENABLED"),
	)
	if err != nil {
		return Calendars{}, err
	}
	cfg := Calendars{
		Enabled:      enabled,
		ClientID:     strings.TrimSpace(getenv("PYMES_GOOGLE_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(getenv("PYMES_GOOGLE_CLIENT_SECRET")),
		RedirectURL:  strings.TrimSpace(getenv("PYMES_GOOGLE_REDIRECT_URL")),
		KMSKeyName:   strings.TrimSpace(getenv("PYMES_CALENDAR_KMS_KEY")),
		AuthURL:      strings.TrimSpace(getenv("PYMES_GOOGLE_AUTH_URL")),
		TokenURL:     strings.TrimSpace(getenv("PYMES_GOOGLE_TOKEN_URL")),
		RevokeURL:    strings.TrimSpace(getenv("PYMES_GOOGLE_REVOKE_URL")),
		CalendarURL:  strings.TrimSpace(getenv("PYMES_GOOGLE_CALENDAR_URL")),
	}
	localKey := strings.TrimSpace(getenv("PYMES_CALENDAR_LOCAL_KEY"))
	if !enabled {
		if cfg.hasConfiguration() || localKey != "" {
			return Calendars{}, fmt.Errorf(
				"calendar configuration requires PYMES_GOOGLE_CALENDAR_ENABLED=true",
			)
		}
		return cfg, nil
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
		return Calendars{}, fmt.Errorf(
			"enabled Google Calendar requires client ID, client secret and redirect URL",
		)
	}
	redirect, err := url.Parse(cfg.RedirectURL)
	if err != nil || redirect.Scheme == "" || redirect.Host == "" {
		return Calendars{}, fmt.Errorf(
			"PYMES_GOOGLE_REDIRECT_URL must be an absolute URL",
		)
	}
	if environment == "production" && redirect.Scheme != "https" {
		return Calendars{}, fmt.Errorf(
			"production Google Calendar redirect URL must use HTTPS",
		)
	}
	if cfg.KMSKeyName != "" && !kmsCryptoKeyPattern.MatchString(cfg.KMSKeyName) {
		return Calendars{}, fmt.Errorf(
			"PYMES_CALENDAR_KMS_KEY must identify a Google Cloud KMS CryptoKey",
		)
	}
	if environment == "production" {
		if cfg.KMSKeyName == "" {
			return Calendars{}, fmt.Errorf(
				"production Google Calendar requires PYMES_CALENDAR_KMS_KEY",
			)
		}
		if localKey != "" || cfg.hasEndpointOverrides() {
			return Calendars{}, fmt.Errorf(
				"production Google Calendar forbids local keys and endpoint overrides",
			)
		}
		return cfg, nil
	}
	if (cfg.KMSKeyName == "") == (localKey == "") {
		return Calendars{}, fmt.Errorf(
			"Google Calendar requires exactly one of KMS or a local envelope key",
		)
	}
	if localKey != "" {
		decoded, decodeErr := base64.StdEncoding.DecodeString(localKey)
		if decodeErr != nil || len(decoded) != 32 {
			return Calendars{}, fmt.Errorf(
				"PYMES_CALENDAR_LOCAL_KEY must be standard base64 for exactly 32 bytes",
			)
		}
		cfg.LocalKey = decoded
	}
	if cfg.hasEndpointOverrides() &&
		(cfg.AuthURL == "" || cfg.TokenURL == "" ||
			cfg.RevokeURL == "" || cfg.CalendarURL == "") {
		return Calendars{}, fmt.Errorf(
			"Google endpoint overrides must configure auth, token, revoke and calendar URLs together",
		)
	}
	return cfg, nil
}

func (cfg Calendars) hasConfiguration() bool {
	return cfg.ClientID != "" || cfg.ClientSecret != "" ||
		cfg.RedirectURL != "" || cfg.KMSKeyName != "" ||
		cfg.hasEndpointOverrides()
}

func (cfg Calendars) hasEndpointOverrides() bool {
	return cfg.AuthURL != "" || cfg.TokenURL != "" ||
		cfg.RevokeURL != "" || cfg.CalendarURL != ""
}

func parseConfigBoolean(name, value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return parsed, nil
}
