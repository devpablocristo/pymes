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
	if cfg.Environment != "development" {
		t.Fatalf("Environment = %q", cfg.Environment)
	}
	if cfg.Clerk.Configured() {
		t.Fatal("Clerk must be optional outside production")
	}
	if cfg.Clerk.Audience != "pymes-v2-api" {
		t.Fatalf("Clerk.Audience = %q", cfg.Clerk.Audience)
	}
	if cfg.Fiscal.Backend != "local" ||
		cfg.Fiscal.LocalDirectory != "tmp/fiscal" ||
		cfg.Fiscal.LocalMasterKeyBase64 == "" {
		t.Fatalf("Fiscal = %+v", cfg.Fiscal)
	}
}

func TestLoadFromOverrides(t *testing.T) {
	cfg, err := LoadFrom(environment(map[string]string{
		"PYMES_HTTP_ADDR":                "127.0.0.1:9000",
		"PYMES_DATABASE_URL":             "postgres://example",
		"PYMES_ENVIRONMENT":              "test",
		"PYMES_SHUTDOWN_TIMEOUT":         "3s",
		"PYMES_READINESS_TIMEOUT":        "250ms",
		"PYMES_CLERK_PUBLISHABLE_KEY":    "pk_test_value",
		"PYMES_CLERK_SECRET_KEY":         "sk_test_value",
		"PYMES_CLERK_ISSUER":             "https://example.clerk.accounts.dev/",
		"PYMES_CLERK_AUTHORIZED_PARTIES": "http://127.0.0.1:15173, http://localhost:15173,http://127.0.0.1:15173",
		"PYMES_FISCAL_STORAGE_DIR":       "/var/lib/pymes/fiscal",
		"PYMES_FISCAL_MASTER_KEY":        "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=",
	}))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9000" || cfg.ShutdownTimeout != 3*time.Second || cfg.ReadinessTimeout != 250*time.Millisecond {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if !cfg.Clerk.Configured() {
		t.Fatal("Clerk must be configured")
	}
	if cfg.Clerk.Issuer != "https://example.clerk.accounts.dev" {
		t.Fatalf("Clerk.Issuer = %q", cfg.Clerk.Issuer)
	}
	if len(cfg.Clerk.AuthorizedParties) != 2 {
		t.Fatalf("AuthorizedParties = %#v", cfg.Clerk.AuthorizedParties)
	}
	if cfg.Fiscal.LocalDirectory != "/var/lib/pymes/fiscal" {
		t.Fatalf("Fiscal.LocalDirectory = %q", cfg.Fiscal.LocalDirectory)
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

func TestLoadFromRequiresClerkInProduction(t *testing.T) {
	_, err := LoadFrom(environment(map[string]string{
		"PYMES_DATABASE_URL": "postgres://example",
		"PYMES_ENVIRONMENT":  "production",
	}))
	if err == nil || !strings.Contains(err.Error(), "Clerk") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadFromRequiresManagedFiscalStorageInProduction(t *testing.T) {
	_, err := LoadFrom(environment(map[string]string{
		"PYMES_DATABASE_URL":             "postgres://example",
		"PYMES_ENVIRONMENT":              "production",
		"PYMES_CLERK_PUBLISHABLE_KEY":    "pk_live_value",
		"PYMES_CLERK_SECRET_KEY":         "sk_live_value",
		"PYMES_CLERK_ISSUER":             "https://example.clerk.accounts.dev",
		"PYMES_CLERK_AUTHORIZED_PARTIES": "https://pymes.example",
	}))
	if err == nil || !strings.Contains(err.Error(), "PYMES_FISCAL_STORAGE_BACKEND") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadFromAcceptsCompleteManagedFiscalStorageInProduction(t *testing.T) {
	cfg, err := LoadFrom(environment(map[string]string{
		"PYMES_DATABASE_URL":               "postgres://example",
		"PYMES_ENVIRONMENT":                "production",
		"PYMES_CLERK_PUBLISHABLE_KEY":      "pk_live_value",
		"PYMES_CLERK_SECRET_KEY":           "sk_live_value",
		"PYMES_CLERK_ISSUER":               "https://example.clerk.accounts.dev",
		"PYMES_CLERK_AUTHORIZED_PARTIES":   "https://pymes.example",
		"PYMES_FISCAL_STORAGE_BACKEND":     "aws",
		"PYMES_FISCAL_AWS_REGION":          "us-east-1",
		"PYMES_FISCAL_KMS_KEY_ID":          "alias/pymes-fiscal",
		"PYMES_FISCAL_S3_BUCKET":           "pymes-fiscal-private",
		"PYMES_FISCAL_S3_PREFIX":           "production",
		"PYMES_FISCAL_S3_FORCE_PATH_STYLE": "false",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Fiscal.Backend != "aws" ||
		cfg.Fiscal.KMSKeyID != "alias/pymes-fiscal" ||
		cfg.Fiscal.S3Bucket != "pymes-fiscal-private" {
		t.Fatalf("Fiscal = %+v", cfg.Fiscal)
	}
}

func TestLoadFromRejectsMalformedFiscalMasterKey(t *testing.T) {
	_, err := LoadFrom(environment(map[string]string{
		"PYMES_DATABASE_URL":      "postgres://example",
		"PYMES_FISCAL_MASTER_KEY": "too-short",
	}))
	if err == nil || !strings.Contains(err.Error(), "PYMES_FISCAL_MASTER_KEY") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadFromRequiresAuthorizedPartiesWhenClerkConfigured(t *testing.T) {
	_, err := LoadFrom(environment(map[string]string{
		"PYMES_DATABASE_URL":          "postgres://example",
		"PYMES_CLERK_PUBLISHABLE_KEY": "pk_test_value",
		"PYMES_CLERK_SECRET_KEY":      "sk_test_value",
		"PYMES_CLERK_ISSUER":          "https://example.clerk.accounts.dev",
	}))
	if err == nil || !strings.Contains(err.Error(), "AUTHORIZED_PARTIES") {
		t.Fatalf("error = %v", err)
	}
}

func environment(values map[string]string) Getenv {
	return func(key string) string { return values[key] }
}
