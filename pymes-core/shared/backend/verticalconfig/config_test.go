package verticalconfig

import (
	"os"
	"os/exec"
	"testing"
)

func TestLoadAllowsLocalDefaultInternalToken(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "")

	cfg := Load(Options{DefaultPort: "8081"})

	if cfg.InternalServiceToken != localInternalServiceToken {
		t.Fatalf("expected local default token, got %q", cfg.InternalServiceToken)
	}
}

func TestLoadExitsWithoutInternalTokenOutsideLocal(t *testing.T) {
	assertLoadExits(t, "production", "")
}

func TestLoadExitsWithDefaultInternalTokenOutsideLocal(t *testing.T) {
	assertLoadExits(t, "production", localInternalServiceToken)
}

func TestLoadFatalExit(t *testing.T) {
	if os.Getenv("VERTICALCONFIG_FATAL_CHILD") != "1" {
		return
	}

	t.Setenv("ENVIRONMENT", os.Getenv("VERTICALCONFIG_FATAL_ENVIRONMENT"))
	t.Setenv("INTERNAL_SERVICE_TOKEN", os.Getenv("VERTICALCONFIG_FATAL_INTERNAL_SERVICE_TOKEN"))
	_ = Load(Options{DefaultPort: "8081"})
}

func assertLoadExits(t *testing.T, environment, token string) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestLoadFatalExit")
	cmd.Env = append(os.Environ(),
		"VERTICALCONFIG_FATAL_CHILD=1",
		"VERTICALCONFIG_FATAL_ENVIRONMENT="+environment,
		"VERTICALCONFIG_FATAL_INTERNAL_SERVICE_TOKEN="+token,
	)

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected Load to exit")
	}
	if _, ok := err.(*exec.ExitError); !ok {
		t.Fatalf("expected exit error, got %T: %v", err, err)
	}
}
