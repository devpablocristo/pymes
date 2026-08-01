package config

import "testing"

func TestLoadFromRequiresCompleteClerkConfiguration(t *testing.T) {
	values := map[string]string{"PYMES_DATABASE_URL": "postgres://db", "PYMES_CLERK_ISSUER": "https://issuer", "PYMES_CLERK_AUTHORIZED_PARTIES": "https://app", "PYMES_CLERK_JWT_KEY": "pem", "PYMES_CLERK_WEBHOOK_SECRET": "whsec_test", "PYMES_SCHEDULING_ACTION_TOKEN_SECRET": "01234567890123456789012345678901"}
	if _, err := LoadFrom(func(key string) string { return values[key] }); err != nil {
		t.Fatal(err)
	}
	delete(values, "PYMES_CLERK_WEBHOOK_SECRET")
	if _, err := LoadFrom(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected missing webhook secret to fail")
	}
}

func TestLoadFromRequiresSignedPerGoWebhookConfigurationWhenEnabled(t *testing.T) {
	values := map[string]string{
		"PYMES_DATABASE_URL":                   "postgres://db",
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
