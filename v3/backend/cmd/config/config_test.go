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
