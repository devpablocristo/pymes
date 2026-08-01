package dto

import (
	"time"

	domain "github.com/devpablocristo/pymes/v3/backend/internal/organization/usecases/domain"
)

type UpdateFeatureFlags struct {
	SchedulingEnabled     bool  `json:"scheduling_enabled"`
	WhatsAppEnabled       bool  `json:"whatsapp_enabled"`
	GoogleCalendarEnabled bool  `json:"google_calendar_enabled"`
	FiscalRealEnabled     bool  `json:"fiscal_real_enabled"`
	ExpectedVersion       int64 `json:"expected_version"`
}

type FeatureFlags struct {
	OrganizationID        string    `json:"organization_id"`
	SchedulingEnabled     bool      `json:"scheduling_enabled"`
	WhatsAppEnabled       bool      `json:"whatsapp_enabled"`
	GoogleCalendarEnabled bool      `json:"google_calendar_enabled"`
	FiscalRealEnabled     bool      `json:"fiscal_real_enabled"`
	Version               int64     `json:"version"`
	UpdatedAt             time.Time `json:"updated_at"`
	UpdatedBy             string    `json:"updated_by"`
}

func FromDomain(flags domain.FeatureFlags) FeatureFlags {
	return FeatureFlags{
		OrganizationID:        flags.OrganizationID,
		SchedulingEnabled:     flags.SchedulingEnabled,
		WhatsAppEnabled:       flags.WhatsAppEnabled,
		GoogleCalendarEnabled: flags.GoogleCalendarEnabled,
		FiscalRealEnabled:     flags.FiscalRealEnabled,
		Version:               flags.Version,
		UpdatedAt:             flags.UpdatedAt,
		UpdatedBy:             flags.UpdatedBy,
	}
}

type Error struct {
	Code string `json:"code"`
}
