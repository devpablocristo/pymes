package helpers

import "testing"

func TestSettingsFromEnvRejectsInvalidRatio(t *testing.T) {
	_, err := SettingsFromEnv("service", "test", func(key string) string {
		if key == "PYMES_TRACE_SAMPLE_RATIO" {
			return "2"
		}
		return ""
	})
	if err == nil {
		t.Fatal("expected invalid ratio")
	}
}
