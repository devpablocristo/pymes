package storage

import (
	"strings"
	"testing"
)

func TestLoadConfigDefaultsToLocalOnlyOutsideProduction(t *testing.T) {
	cfg, err := LoadConfig(func(string) string { return "" }, "development")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backend != BackendLocal || cfg.LocalDirectory != "tmp/fiscal" ||
		cfg.LocalMasterKeyBase64 == "" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadConfigRequiresManagedStorageInProduction(t *testing.T) {
	_, err := LoadConfig(func(string) string { return "" }, "production")
	if err == nil || !strings.Contains(err.Error(), "STORAGE_BACKEND=aws") {
		t.Fatalf("missing production backend error = %v", err)
	}
	values := map[string]string{
		"PYMES_FISCAL_STORAGE_BACKEND": "local",
	}
	_, err = LoadConfig(func(name string) string { return values[name] }, "production")
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("local production backend error = %v", err)
	}
}

func TestLoadConfigRequiresKMSAndS3AndRejectsInsecureProductionEndpoints(t *testing.T) {
	values := map[string]string{
		"PYMES_FISCAL_STORAGE_BACKEND": "aws",
		"PYMES_FISCAL_AWS_REGION":      "us-east-1",
		"PYMES_FISCAL_KMS_KEY_ID":      "alias/pymes-fiscal",
		"PYMES_FISCAL_S3_BUCKET":       "pymes-fiscal-private",
		"PYMES_FISCAL_S3_ENDPOINT":     "http://s3.internal",
	}
	getenv := func(name string) string { return values[name] }
	if _, err := LoadConfig(getenv, "production"); err == nil ||
		!strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("insecure endpoint error = %v", err)
	}
	values["PYMES_FISCAL_S3_ENDPOINT"] = "https://s3.internal"
	values["PYMES_FISCAL_S3_FORCE_PATH_STYLE"] = "true"
	cfg, err := LoadConfig(getenv, "production")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backend != BackendAWS || !cfg.S3ForcePathStyle ||
		cfg.S3Prefix != "pymes-v2" {
		t.Fatalf("config = %+v", cfg)
	}
	delete(values, "PYMES_FISCAL_KMS_KEY_ID")
	if _, err := LoadConfig(getenv, "production"); err == nil ||
		!strings.Contains(err.Error(), "KMS_KEY_ID") {
		t.Fatalf("missing KMS error = %v", err)
	}
}
