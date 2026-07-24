package main

import (
	"strings"
	"testing"
	"time"
)

func TestLoadEnvironmentUsesSecureDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := loadEnvironment(func(key string) string {
		if key == "PYMES_DATABASE_URL" {
			return "postgres://pymes_backend:secret@postgres/pymes_v2"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL == "" ||
		cfg.WorkerConfig.WorkerID == "" ||
		cfg.WorkerConfig.ActorID != "system:fiscal-accounting" ||
		cfg.WorkerConfig.PollInterval != time.Second ||
		cfg.WorkerConfig.RetryDelay != 30*time.Second ||
		cfg.WorkerConfig.MaxAttempts != 10 ||
		cfg.OrganizationBatch != defaultOrganizationBatchSize {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadEnvironmentUsesExplicitWorkerOverrides(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"PYMES_DATABASE_URL":                     "postgres://backend",
		"PYMES_FISCAL_ACCOUNTING_WORKER_ID":      "posting-1",
		"PYMES_FISCAL_ACCOUNTING_ACTOR":          "system:posting-1",
		"PYMES_FISCAL_ACCOUNTING_POLL_INTERVAL":  "250ms",
		"PYMES_FISCAL_ACCOUNTING_RETRY_DELAY":    "45s",
		"PYMES_FISCAL_ACCOUNTING_MAX_ATTEMPTS":   "20",
		"PYMES_FISCAL_ACCOUNTING_ORG_BATCH_SIZE": "40",
	}
	cfg, err := loadEnvironment(func(key string) string {
		return values[key]
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkerConfig.WorkerID != "posting-1" ||
		cfg.WorkerConfig.ActorID != "system:posting-1" ||
		cfg.WorkerConfig.PollInterval != 250*time.Millisecond ||
		cfg.WorkerConfig.RetryDelay != 45*time.Second ||
		cfg.WorkerConfig.MaxAttempts != 20 ||
		cfg.OrganizationBatch != 40 {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadEnvironmentFailsClosedWithoutDatabaseURL(t *testing.T) {
	t.Parallel()

	_, err := loadEnvironment(func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "PYMES_DATABASE_URL") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadEnvironmentRejectsInvalidWorkerValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "poll interval",
			key:   "PYMES_FISCAL_ACCOUNTING_POLL_INTERVAL",
			value: "0s",
		},
		{
			name:  "retry delay",
			key:   "PYMES_FISCAL_ACCOUNTING_RETRY_DELAY",
			value: "-1s",
		},
		{
			name:  "attempts",
			key:   "PYMES_FISCAL_ACCOUNTING_MAX_ATTEMPTS",
			value: "0",
		},
		{
			name:  "organization batch",
			key:   "PYMES_FISCAL_ACCOUNTING_ORG_BATCH_SIZE",
			value: "1001",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadEnvironment(func(key string) string {
				if key == "PYMES_DATABASE_URL" {
					return "postgres://backend"
				}
				if key == test.key {
					return test.value
				}
				return ""
			})
			if err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
