package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadFromDefaults(t *testing.T) {
	cfg, err := LoadFrom(environment(map[string]string{
		"PYMES_DATABASE_URL": " postgres://example ",
	}))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.ShutdownTimeout != 10*time.Second || cfg.ReadinessTimeout != 2*time.Second {
		t.Fatalf("unexpected timeouts: %+v", cfg)
	}
}

func TestLoadFromOverrides(t *testing.T) {
	cfg, err := LoadFrom(environment(map[string]string{
		"PYMES_HTTP_ADDR":         "127.0.0.1:9000",
		"PYMES_DATABASE_URL":      "postgres://example",
		"PYMES_SHUTDOWN_TIMEOUT":  "3s",
		"PYMES_READINESS_TIMEOUT": "250ms",
	}))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9000" || cfg.ShutdownTimeout != 3*time.Second || cfg.ReadinessTimeout != 250*time.Millisecond {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadFromRejectsMissingDatabaseURL(t *testing.T) {
	_, err := LoadFrom(environment(nil))
	if err == nil || !strings.Contains(err.Error(), "PYMES_DATABASE_URL") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadFromRejectsInvalidDuration(t *testing.T) {
	_, err := LoadFrom(environment(map[string]string{
		"PYMES_DATABASE_URL":      "postgres://example",
		"PYMES_READINESS_TIMEOUT": "never",
	}))
	if err == nil || !strings.Contains(err.Error(), "PYMES_READINESS_TIMEOUT") {
		t.Fatalf("error = %v", err)
	}
}

func environment(values map[string]string) Getenv {
	return func(key string) string { return values[key] }
}
