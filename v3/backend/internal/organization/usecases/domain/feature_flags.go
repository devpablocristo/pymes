package domain

import (
	"errors"
	"strings"
	"time"
)

type Feature string

const (
	FeatureScheduling     Feature = "scheduling_enabled"
	FeatureWhatsApp       Feature = "whatsapp_enabled"
	FeatureGoogleCalendar Feature = "google_calendar_enabled"
	FeatureFiscalReal     Feature = "fiscal_real_enabled"
)

var (
	ErrFeatureVersionConflict = errors.New("FEATURE_VERSION_CONFLICT")
	ErrFeatureUnknown         = errors.New("FEATURE_UNKNOWN")
)

func ParseFeature(value string) (Feature, error) {
	feature := Feature(strings.TrimSpace(value))
	switch feature {
	case FeatureScheduling, FeatureWhatsApp, FeatureGoogleCalendar, FeatureFiscalReal:
		return feature, nil
	default:
		return "", ErrFeatureUnknown
	}
}

type FeatureFlags struct {
	OrganizationID        string
	SchedulingEnabled     bool
	WhatsAppEnabled       bool
	GoogleCalendarEnabled bool
	FiscalRealEnabled     bool
	Version               int64
	UpdatedAt             time.Time
	UpdatedBy             string
}

func (flags FeatureFlags) Valid() bool {
	return strings.TrimSpace(flags.OrganizationID) != "" &&
		flags.Version > 0 &&
		!flags.UpdatedAt.IsZero() &&
		strings.TrimSpace(flags.UpdatedBy) != ""
}

func (flags FeatureFlags) Enabled(feature Feature) bool {
	switch feature {
	case FeatureScheduling:
		return flags.SchedulingEnabled
	case FeatureWhatsApp:
		return flags.WhatsAppEnabled
	case FeatureGoogleCalendar:
		return flags.GoogleCalendarEnabled
	case FeatureFiscalReal:
		return flags.FiscalRealEnabled
	default:
		return false
	}
}

type UpdateFeatureFlags struct {
	OrganizationID        string
	SchedulingEnabled     bool
	WhatsAppEnabled       bool
	GoogleCalendarEnabled bool
	FiscalRealEnabled     bool
	ExpectedVersion       int64
	ActorID               string
}

func (command UpdateFeatureFlags) Valid() bool {
	return strings.TrimSpace(command.OrganizationID) != "" &&
		command.ExpectedVersion > 0 &&
		strings.TrimSpace(command.ActorID) != "" &&
		len(command.ActorID) <= 255
}
