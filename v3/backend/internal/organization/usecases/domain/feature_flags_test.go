package domain

import (
	"testing"
	"time"
)

func TestFeatureFlagsRecognizeOnlyClosedProductCapabilities(t *testing.T) {
	t.Parallel()
	flags := FeatureFlags{
		OrganizationID:        "org-a",
		SchedulingEnabled:     true,
		WhatsAppEnabled:       false,
		GoogleCalendarEnabled: true,
		FiscalRealEnabled:     false,
		Version:               1,
		UpdatedAt:             time.Now().UTC(),
		UpdatedBy:             "user-a",
	}
	if !flags.Valid() {
		t.Fatal("valid feature flags rejected")
	}
	for feature, expected := range map[Feature]bool{
		FeatureScheduling:     true,
		FeatureWhatsApp:       false,
		FeatureGoogleCalendar: true,
		FeatureFiscalReal:     false,
	} {
		parsed, err := ParseFeature(string(feature))
		if err != nil || parsed != feature || flags.Enabled(feature) != expected {
			t.Fatalf(
				"feature=%q parsed=%q enabled=%v err=%v",
				feature,
				parsed,
				flags.Enabled(feature),
				err,
			)
		}
	}
	if _, err := ParseFeature("unknown_enabled"); err != ErrFeatureUnknown {
		t.Fatalf("unknown feature err=%v", err)
	}
}
