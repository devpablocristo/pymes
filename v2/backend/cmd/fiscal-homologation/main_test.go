package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadOptionsFailsClosedWithoutExplicitOptIn(t *testing.T) {
	_, err := loadOptions(
		[]string{"--organization-id", "018fe915-aaba-77b0-a55c-68427afe1e77"},
		func(string) string { return "" },
	)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadOptionsRequiresExplicitTenantAndSecureRuntime(t *testing.T) {
	values := map[string]string{
		"PYMES_FISCAL_HOMOLOGATION_ENABLED": "true",
		"PYMES_DATABASE_URL":                "postgres://test",
		"PYMES_FISCAL_STORAGE_DIR":          "/tmp/pymes-homologation-test",
		"PYMES_FISCAL_MASTER_KEY": base64.StdEncoding.EncodeToString(
			make([]byte, 32),
		),
	}
	getenv := func(name string) string { return values[name] }
	if _, err := loadOptions(nil, getenv); err == nil ||
		!strings.Contains(err.Error(), "--organization-id") {
		t.Fatalf("missing tenant error = %v", err)
	}
	cfg, err := loadOptions(
		[]string{
			"--organization-id", "018fe915-aaba-77b0-a55c-68427afe1e77",
			"--actor", "test:accountant",
		},
		getenv,
	)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OrganizationID.String() != "018fe915-aaba-77b0-a55c-68427afe1e77" ||
		cfg.Actor != "test:accountant" || cfg.Storage.Backend != "local" {
		t.Fatalf("options = %+v", cfg)
	}
}

func TestLoadOptionsRequiresManagedStorageInProduction(t *testing.T) {
	values := map[string]string{
		"PYMES_FISCAL_HOMOLOGATION_ENABLED": "true",
		"PYMES_DATABASE_URL":                "postgres://test",
		"PYMES_ENVIRONMENT":                 "production",
	}
	_, err := loadOptions(
		[]string{"--organization-id", "018fe915-aaba-77b0-a55c-68427afe1e77"},
		func(name string) string { return values[name] },
	)
	if err == nil || !strings.Contains(err.Error(), "STORAGE_BACKEND=aws") {
		t.Fatalf("error = %v", err)
	}
}
