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

func TestLoadFromEnvPanicsWithoutInternalTokenOutsideLocal(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing production internal token")
		}
	}()

	_ = LoadFromEnv()
}

func TestLoadFromEnvPanicsWithDefaultInternalTokenOutsideLocal(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("INTERNAL_SERVICE_TOKEN", localInternalServiceToken)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for default production internal token")
		}
	}()

	_ = LoadFromEnv()
}
