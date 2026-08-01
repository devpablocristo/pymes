package config

import "testing"

func TestLoadFromRequiresCompleteClerkConfiguration(t *testing.T) {
	values := map[string]string{"PYMES_DATABASE_URL": "postgres://db", "FISCAL_ADAPTER_URL": "https://fiscal.internal", "PYMES_CLERK_ISSUER": "https://issuer", "PYMES_CLERK_AUTHORIZED_PARTIES": "https://app", "PYMES_CLERK_JWT_KEY": "pem", "PYMES_CLERK_WEBHOOK_SECRET": "whsec_test"}
	if _, err := LoadFrom(func(key string) string { return values[key] }); err != nil {
		t.Fatal(err)
	}
	delete(values, "PYMES_CLERK_WEBHOOK_SECRET")
	if _, err := LoadFrom(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected missing webhook secret to fail")
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
