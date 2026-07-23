package main

import (
	"strings"
	"testing"
	"time"
)

func TestLoadEnvironmentFailsClosedWithoutSecrets(t *testing.T) {
	_, err := loadEnvironment(environmentFrom(map[string]string{
		"PYMES_DATABASE_URL": "postgres://example",
	}))
	if err == nil || !strings.Contains(err.Error(), "PYMES_CLERK_SECRET_KEY") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadEnvironmentUsesExplicitWorkerOverrides(t *testing.T) {
	cfg, err := loadEnvironment(environmentFrom(map[string]string{
		"PYMES_DATABASE_URL":               " postgres://example ",
		"PYMES_CLERK_SECRET_KEY":           " sk_test ",
		"PYMES_CLERK_API_URL":              "http://clerk.test/",
		"PYMES_IAM_WORKER_BATCH_SIZE":      "4",
		"PYMES_IAM_WORKER_POLL_INTERVAL":   "250ms",
		"PYMES_IAM_WORKER_LEASE_DURATION":  "45s",
		"PYMES_IAM_WORKER_PUBLISH_TIMEOUT": "30s",
	}))
	if err != nil {
		t.Fatalf("loadEnvironment() error = %v", err)
	}
	if cfg.DatabaseURL != "postgres://example" ||
		cfg.ClerkSecret != "sk_test" ||
		cfg.ClerkBaseURL != "http://clerk.test" {
		t.Fatalf("configuration = %#v", cfg)
	}
	if cfg.Worker.BatchSize != 4 ||
		cfg.Worker.PollInterval != 250*time.Millisecond ||
		cfg.Worker.LeaseDuration != 45*time.Second ||
		cfg.Worker.PublishTimeout != 30*time.Second {
		t.Fatalf("worker configuration = %#v", cfg.Worker)
	}
}

func TestLoadEnvironmentRejectsNonPositiveWorkerValues(t *testing.T) {
	_, err := loadEnvironment(environmentFrom(map[string]string{
		"PYMES_DATABASE_URL":          "postgres://example",
		"PYMES_CLERK_SECRET_KEY":      "sk_test",
		"PYMES_IAM_WORKER_BATCH_SIZE": "0",
	}))
	if err == nil || !strings.Contains(err.Error(), "BATCH_SIZE") {
		t.Fatalf("error = %v", err)
	}
}

func environmentFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
