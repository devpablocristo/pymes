package config

import "testing"

func TestLoadFromEnvAllowsLocalDefaultInternalToken(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")

	cfg := LoadFromEnv()

	if cfg.InternalServiceToken != localInternalServiceToken {
		t.Fatalf("expected local default token, got %q", cfg.InternalServiceToken)
	}
}

func TestValidateInternalServiceTokenRejectsEmptyInProduction(t *testing.T) {
	t.Parallel()
	if err := validateInternalServiceToken("production", ""); err == nil {
		t.Fatal("expected error for missing production internal token")
	}
}

func TestValidateInternalServiceTokenRejectsDefaultInProduction(t *testing.T) {
	t.Parallel()
	if err := validateInternalServiceToken("production", localInternalServiceToken); err == nil {
		t.Fatal("expected error for default production internal token")
	}
}

func TestValidateInternalServiceTokenAllowsLocalDefault(t *testing.T) {
	t.Parallel()
	if err := validateInternalServiceToken("development", ""); err != nil {
		t.Fatalf("unexpected error for local token: %v", err)
	}
}
