package main

import (
	"strings"
	"testing"
)

func TestLoadEnvironment(t *testing.T) {
	values := map[string]string{
		"PYMES_DATABASE_URL":                 "postgres://worker",
		"PYMES_FISCAL_STORAGE_DIR":           "/private/fiscal",
		"PYMES_FISCAL_MASTER_KEY":            "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=",
		"PYMES_FISCAL_WORKER_ID":             "worker-1",
		"PYMES_FISCAL_WORKER_POLL_INTERVAL":  "250ms",
		"PYMES_FISCAL_WORKER_LEASE_DURATION": "90s",
	}
	cfg, err := loadEnvironment(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkerID != "worker-1" || cfg.PollInterval.String() != "250ms" ||
		cfg.LeaseDuration.String() != "1m30s" || cfg.Storage.Backend != "local" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadEnvironmentRejectsLocalOrMissingManagedStorageInProduction(t *testing.T) {
	values := map[string]string{
		"PYMES_DATABASE_URL": "postgres://worker",
		"PYMES_ENVIRONMENT":  "production",
	}
	_, err := loadEnvironment(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "STORAGE_BACKEND=aws") {
		t.Fatalf("error = %v", err)
	}
	values["PYMES_FISCAL_STORAGE_BACKEND"] = "local"
	_, err = loadEnvironment(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("local production error = %v", err)
	}
}
