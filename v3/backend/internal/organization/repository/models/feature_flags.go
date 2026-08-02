package models

import "time"

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
