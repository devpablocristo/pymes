package verticalconfig

import "testing"

func TestLoadAllowsLocalDefaultInternalToken(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")

	cfg := Load(Options{DefaultPort: "8081"})

	if cfg.InternalServiceToken != localInternalServiceToken {
		t.Fatalf("expected local default token, got %q", cfg.InternalServiceToken)
	}
}

func TestValidateInternalServiceTokenRejectsMissingTokenOutsideLocal(t *testing.T) {
	if err := validateInternalServiceToken("production", ""); err == nil {
		t.Fatal("expected error for missing production internal token")
	}
}

func TestValidateInternalServiceTokenRejectsDefaultTokenOutsideLocal(t *testing.T) {
	if err := validateInternalServiceToken("production", localInternalServiceToken); err == nil {
		t.Fatal("expected error for default production internal token")
	}
}
