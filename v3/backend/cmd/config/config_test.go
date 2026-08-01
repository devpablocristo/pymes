package config

import "testing"

func TestLoadFromRequiresCompleteClerkConfiguration(t *testing.T) {
	values := map[string]string{"PYMES_DATABASE_URL": "postgres://db", "FISCAL_ADAPTER_URL": "https://fiscal.internal", "PYMES_CLERK_ISSUER": "https://issuer", "PYMES_CLERK_AUTHORIZED_PARTIES": "https://app", "PYMES_CLERK_JWT_KEY": "pem", "PYMES_CLERK_WEBHOOK_SECRET": "whsec_test", "PYMES_SCHEDULING_ACTION_TOKEN_SECRET": "01234567890123456789012345678901"}
	if _, err := LoadFrom(func(key string) string { return values[key] }); err != nil {
		t.Fatal(err)
	}
	delete(values, "PYMES_CLERK_WEBHOOK_SECRET")
	if _, err := LoadFrom(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected missing webhook secret to fail")
	}
}

func TestLoadFromFailsClosedOnFiscalURLAndSchedulingActionSecret(t *testing.T) {
	t.Parallel()
	base := map[string]string{
		"PYMES_DATABASE_URL":                   "postgres://db",
		"FISCAL_ADAPTER_URL":                   "https://fiscal.internal",
		"PYMES_CLERK_ISSUER":                   "https://issuer",
		"PYMES_CLERK_AUTHORIZED_PARTIES":       "https://app",
		"PYMES_CLERK_JWT_KEY":                  "pem",
		"PYMES_CLERK_WEBHOOK_SECRET":           "whsec_test",
		"PYMES_SCHEDULING_ACTION_TOKEN_SECRET": "01234567890123456789012345678901",
	}
	for _, test := range []struct {
		name    string
		missing string
	}{
		{name: "fiscal adapter URL", missing: "FISCAL_ADAPTER_URL"},
		{
			name:    "scheduling action token secret",
			missing: "PYMES_SCHEDULING_ACTION_TOKEN_SECRET",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			values := make(map[string]string, len(base))
			for key, value := range base {
				values[key] = value
			}
			delete(values, test.missing)
			if _, err := LoadFrom(func(key string) string {
				return values[key]
			}); err == nil {
				t.Fatalf("expected missing %s to fail closed", test.missing)
			}
		})
	}
}

func TestLoadFromRequiresSignedPerGoWebhookConfigurationWhenEnabled(t *testing.T) {
	values := map[string]string{
		"PYMES_DATABASE_URL":                   "postgres://db",
		"FISCAL_ADAPTER_URL":                   "https://fiscal.internal",
		"PYMES_CLERK_ISSUER":                   "https://issuer",
		"PYMES_CLERK_AUTHORIZED_PARTIES":       "https://app",
		"PYMES_CLERK_JWT_KEY":                  "pem",
		"PYMES_CLERK_WEBHOOK_SECRET":           "whsec_test",
		"PYMES_SCHEDULING_ACTION_TOKEN_SECRET": "01234567890123456789012345678901",
		"PYMES_PERGO_ENABLED":                  "true",
		"PERGO_WORKSPACE_ID":                   "workspace-1",
		"PERGO_WEBHOOK_SECRETS":                "0123456789abcdef0123456789abcdef",
	}
	cfg, err := LoadFrom(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.PerGo.Enabled || len(cfg.PerGo.WebhookSecrets) != 1 {
		t.Fatalf("PerGo config = %+v", cfg.PerGo)
	}
	delete(values, "PERGO_WEBHOOK_SECRETS")
	if _, err := LoadFrom(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected missing webhook secret to fail closed")
	}
	values["PERGO_WEBHOOK_SECRETS"] = "too-short"
	if _, err := LoadFrom(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected weak webhook secret to fail closed")
	}
}

func TestLoadFromRejectsProductionIdentityBypass(t *testing.T) {
	values := map[string]string{
		"PYMES_ENVIRONMENT":                   "production",
		"PYMES_DATABASE_URL":                  "postgres://db",
		"FISCAL_ADAPTER_URL":                  "https://fiscal.internal",
		"PYMES_ALLOW_INSECURE_LOCAL_SERVICES": "true",
	}
	if _, err := LoadFrom(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected production identity bypass to fail")
	}
}

func TestLoadFromRequiresStrongProductionPreflightGate(t *testing.T) {
	base := map[string]string{
		"PYMES_ENVIRONMENT":                    "production",
		"PYMES_DATABASE_URL":                   "postgres://db",
		"FISCAL_ADAPTER_URL":                   "https://fiscal.internal",
		"PYMES_CLERK_ISSUER":                   "https://issuer",
		"PYMES_CLERK_AUTHORIZED_PARTIES":       "https://app",
		"PYMES_CLERK_JWT_KEY":                  "pem",
		"PYMES_CLERK_WEBHOOK_SECRET":           "whsec_test",
		"PYMES_SCHEDULING_ACTION_TOKEN_SECRET": "01234567890123456789012345678901",
		"PYMES_PREFLIGHT_TAG":                  "candidate-1111111111111111111111111111111111111111",
		"PYMES_PREFLIGHT_TOKEN":                "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	if _, err := LoadFrom(func(key string) string { return base[key] }); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"PYMES_PREFLIGHT_TAG", "PYMES_PREFLIGHT_TOKEN"} {
		values := make(map[string]string, len(base))
		for name, value := range base {
			values[name] = value
		}
		delete(values, key)
		if _, err := LoadFrom(func(name string) string {
			return values[name]
		}); err == nil {
			t.Fatalf("expected missing %s to fail closed", key)
		}
	}
}
