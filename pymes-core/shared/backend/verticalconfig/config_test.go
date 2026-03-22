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

func TestLoadPanicsWithoutInternalTokenOutsideLocal(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing production internal token")
		}
	}()

	_ = Load(Options{DefaultPort: "8081"})
}

func TestLoadPanicsWithDefaultInternalTokenOutsideLocal(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("INTERNAL_SERVICE_TOKEN", localInternalServiceToken)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for default production internal token")
		}
	}()

	_ = Load(Options{DefaultPort: "8081"})
}
