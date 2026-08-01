package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadCalendarsIsDisabledWithoutSecrets(t *testing.T) {
	t.Parallel()
	cfg, err := loadCalendars(func(string) string { return "" }, "development")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled || cfg.ClientSecret != "" || len(cfg.LocalKey) != 0 {
		t.Fatalf("calendar config=%+v", cfg)
	}
}

func TestLoadCalendarsAcceptsLocalEnvelopeKeyOnlyOutsideProduction(t *testing.T) {
	t.Parallel()
	values := validCalendarValues()
	values["PYMES_CALENDAR_LOCAL_KEY"] = base64.StdEncoding.EncodeToString(
		[]byte("01234567890123456789012345678901"),
	)
	cfg, err := loadCalendars(
		func(key string) string { return values[key] },
		"test",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || len(cfg.LocalKey) != 32 || cfg.KMSKeyName != "" {
		t.Fatalf("calendar config=%+v", cfg)
	}
}

func TestLoadCalendarsRequiresKMSAndHTTPSInProduction(t *testing.T) {
	t.Parallel()
	values := validCalendarValues()
	values["PYMES_CALENDAR_KMS_KEY"] =
		"projects/shared/locations/us-central1/keyRings/pymes-prd/cryptoKeys/calendars"
	cfg, err := loadCalendars(
		func(key string) string { return values[key] },
		"production",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.KMSKeyName == "" || len(cfg.LocalKey) != 0 {
		t.Fatalf("calendar config=%+v", cfg)
	}

	values["PYMES_CALENDAR_LOCAL_KEY"] = base64.StdEncoding.EncodeToString(
		make([]byte, 32),
	)
	if _, err := loadCalendars(
		func(key string) string { return values[key] },
		"production",
	); err == nil || !strings.Contains(err.Error(), "forbids") {
		t.Fatalf("expected production local key rejection, got %v", err)
	}

	delete(values, "PYMES_CALENDAR_LOCAL_KEY")
	values["PYMES_GOOGLE_REDIRECT_URL"] =
		"http://pymes.test/api/v1/calendars/google/oauth/callback"
	if _, err := loadCalendars(
		func(key string) string { return values[key] },
		"production",
	); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS rejection, got %v", err)
	}
}

func TestLoadCalendarsFailsClosedOnPartialOrAmbiguousConfiguration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(map[string]string)
	}{
		{
			name: "secrets while disabled",
			mutate: func(values map[string]string) {
				delete(values, "PYMES_GOOGLE_CALENDAR_ENABLED")
			},
		},
		{
			name:   "no envelope key",
			mutate: func(map[string]string) {},
		},
		{
			name: "two envelope keys",
			mutate: func(values map[string]string) {
				values["PYMES_CALENDAR_KMS_KEY"] =
					"projects/shared/locations/us-central1/keyRings/pymes/cryptoKeys/calendars"
				values["PYMES_CALENDAR_LOCAL_KEY"] =
					base64.StdEncoding.EncodeToString(make([]byte, 32))
			},
		},
		{
			name: "partial fake endpoints",
			mutate: func(values map[string]string) {
				values["PYMES_CALENDAR_LOCAL_KEY"] =
					base64.StdEncoding.EncodeToString(make([]byte, 32))
				values["PYMES_GOOGLE_TOKEN_URL"] = "https://fake.test/token"
			},
		},
		{
			name: "bad key length",
			mutate: func(values map[string]string) {
				values["PYMES_CALENDAR_LOCAL_KEY"] =
					base64.StdEncoding.EncodeToString(make([]byte, 31))
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			values := validCalendarValues()
			test.mutate(values)
			if _, err := loadCalendars(
				func(key string) string { return values[key] },
				"development",
			); err == nil {
				t.Fatal("expected invalid calendar configuration")
			}
		})
	}
}

func TestWorkerCalendarConfigurationUsesStableErrorCode(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"PYMES_DATABASE_URL":                  "postgres://db",
		"FISCAL_ADAPTER_URL":                  "http://fiscal",
		"ACCOUNTING_URL":                      "http://accounting",
		"PYMES_GOOGLE_CALENDAR_ENABLED":       "true",
		"PYMES_GOOGLE_CLIENT_ID":              "client",
		"PYMES_GOOGLE_CLIENT_SECRET":          "secret",
		"PYMES_GOOGLE_REDIRECT_URL":           "https://app.test/api/v1/calendars/google/oauth/callback",
		"PYMES_ALLOW_INSECURE_LOCAL_SERVICES": "true",
	}
	_, err := LoadWorkerFrom(func(key string) string { return values[key] })
	if err == nil || WorkerErrorCode(err) != "CALENDAR_CONFIG_INVALID" {
		t.Fatalf("err=%v code=%s", err, WorkerErrorCode(err))
	}
}

func validCalendarValues() map[string]string {
	return map[string]string{
		"PYMES_GOOGLE_CALENDAR_ENABLED": "true",
		"PYMES_GOOGLE_CLIENT_ID":        "client-id",
		"PYMES_GOOGLE_CLIENT_SECRET":    "client-secret",
		"PYMES_GOOGLE_REDIRECT_URL":     "https://app.test/api/v1/calendars/google/oauth/callback",
	}
}

func TestLoadCalendarsRejectsNonBFFCallbackPath(t *testing.T) {
	t.Parallel()
	values := validCalendarValues()
	values["PYMES_CALENDAR_LOCAL_KEY"] = base64.StdEncoding.EncodeToString(
		make([]byte, 32),
	)
	values["PYMES_GOOGLE_REDIRECT_URL"] =
		"https://app.test/organizations/org-a/calendars/google/oauth/complete"
	_, err := loadCalendars(
		func(key string) string { return values[key] },
		"development",
	)
	if err == nil || !strings.Contains(err.Error(), "global BFF") {
		t.Fatalf("expected callback path rejection, got %v", err)
	}
}
